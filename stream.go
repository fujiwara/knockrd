package knockrd

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/wafv2"
	mapset "github.com/deckarep/golang-set"
	consul "github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
)

var DefaultConsulKVPath = "knockrd/allowed"

type streamer struct {
	conf          *Config
	wafv2Regional *wafv2.WAFV2
	wafv2CF       *wafv2.WAFV2
	ec2           *ec2.EC2
}

// NewStreamHandler creates a DynamoDB Stream handler function
func NewStreamHandler(conf *Config) func(context.Context, events.DynamoDBEvent) error {
	awsCfgRegional := &aws.Config{
		Region: aws.String(conf.AWS.Region),
	}
	awsCfgCF := &aws.Config{
		Region: aws.String("us-east-1"), // for CloudFront
	}
	if conf.AWS.Endpoint != "" {
		awsCfgRegional.Endpoint = aws.String(conf.AWS.Endpoint)
		awsCfgCF.Endpoint = aws.String(conf.AWS.Endpoint)
	}

	s := &streamer{
		conf:          conf,
		wafv2Regional: wafv2.New(session.New(), awsCfgRegional),
		wafv2CF:       wafv2.New(session.New(), awsCfgCF),
		ec2:           ec2.New(session.New(), awsCfgRegional),
	}

	return s.Handler
}

type ipSetEvent struct {
	address string
	add     bool
	v4      bool
}

func (e ipSetEvent) CIDR() string {
	if e.v4 {
		return e.address + "/32"
	}
	return e.address + "/128"
}

func parseEventRecord(r events.DynamoDBEventRecord) *ipSetEvent {
	key, ok := r.Change.Keys["Key"]
	if !ok {
		log.Printf("[warn] unkown key %v", r.Change.Keys)
		return nil
	}
	ip := net.ParseIP(key.String())
	if ip == nil {
		log.Printf("[debug] ignore Key:%s", key.String())
		return nil
	}
	log.Printf("[info] processing IP:%s Event:%s", ip.String(), r.EventName)
	var add bool
	if ipv4 := ip.To4(); ipv4 != nil {
		switch r.EventName {
		case "INSERT", "MODIFY":
			add = true
		case "REMOVE":
		default:
			log.Printf("[warn] unknown event %s", r.EventName)
			return nil
		}
		log.Printf("[debug] IPV4 %s add %t", ip.String(), add)
		return &ipSetEvent{ipv4.String(), add, true}
	} else {
		switch r.EventName {
		case "INSERT", "MODIFY":
			add = true
		case "REMOVE":
		default:
			log.Printf("[warn] unknown event %s", r.EventName)
			return nil
		}
		log.Printf("[debug] IPV6 %s add %t", ip.String(), add)
		return &ipSetEvent{ip.String(), add, false}
	}
}

func (s *streamer) Handler(ctx context.Context, event events.DynamoDBEvent) error {
	var v4, v6 []ipSetEvent
	for _, r := range event.Records {
		ipsev := parseEventRecord(r)
		if ipsev == nil {
			continue
		}
		if ipsev.v4 {
			v4 = append(v4, *ipsev)
		} else {
			v6 = append(v6, *ipsev)
		}
	}
	if s.conf.IPSet != nil {
		if err := s.updateIPSet(s.conf.IPSet.V4, v4); err != nil {
			return err
		}
		if err := s.updateIPSet(s.conf.IPSet.V6, v6); err != nil {
			return err
		}
	}
	if s.conf.Consul != nil {
		if err := s.updateConsulKV(s.conf.Consul, v4, v6); err != nil {
			return err
		}
	}
	if len(s.conf.SecurityGroups) > 0 {
		if err := s.updateSecurityGroup(s.conf.SecurityGroups, v4, v6); err != nil {
			return err
		}
	}
	return nil
}

func (s *streamer) updateIPSet(c *IPSetConfig, events []ipSetEvent) error {
	if c == nil || c.ID == "" || len(events) == 0 {
		return nil
	}
	var svc *wafv2.WAFV2
	switch c.Scope {
	case "REGIONAL":
		svc = s.wafv2Regional
	case "CLOUDFRONT":
		svc = s.wafv2CF
	default:
		return fmt.Errorf("invalid scope %s: Set REGIONAL or CLOUDFRONT", c.Scope)
	}

	res, err := svc.GetIPSet(&wafv2.GetIPSetInput{
		Name:  &c.Name,
		Id:    &c.ID,
		Scope: &c.Scope,
	})
	if err != nil {
		return err
	}
	lockToken := res.LockToken
	addrs := mapset.NewSet()
	for _, ad := range res.IPSet.Addresses {
		_, ipnet, _ := net.ParseCIDR(*ad)
		addrs.Add(ipnet.String())
	}
	log.Printf("[debug] current addresses %s", addrs.String())
	for _, e := range events {
		if e.add {
			log.Printf("[debug] add address %s", e.CIDR())
			addrs.Add(e.CIDR())
		} else {
			log.Printf("[debug] remove address %s", e.CIDR())
			addrs.Remove(e.CIDR())
		}
	}
	log.Printf(
		"[info] update ip-set id:%s name:%s scope:%s addresses:%s",
		c.ID, c.Name, c.Scope, addrs.String(),
	)
	updates := make([]*string, 0, addrs.Cardinality())
	for _, ad := range addrs.ToSlice() {
		updates = append(updates, aws.String(ad.(string)))
	}
	_, err = svc.UpdateIPSet(&wafv2.UpdateIPSetInput{
		Name:      &c.Name,
		Id:        &c.ID,
		Scope:     &c.Scope,
		Addresses: updates,
		LockToken: lockToken,
	})
	return err
}

func (s *streamer) updateConsulKV(c *ConsulConfig, events ...[]ipSetEvent) error {
	client, err := consul.NewClient(&consul.Config{
		Address:    c.Address,
		Scheme:     c.Scheme,
		Datacenter: c.Datacenter,
	})
	if err != nil {
		return errors.Wrap(err, "failed to new consul client")
	}
	kv := client.KV()
	kvPath := c.KVPath
	if kvPath == "" {
		kvPath = DefaultConsulKVPath
	}
	for _, evs := range events {
		for _, ev := range evs {
			key := path.Join(kvPath, url.PathEscape(ev.address))
			if ev.add {
				log.Printf("[info] put to consul key=%s", key)
				p := consul.KVPair{
					Key:   key,
					Value: []byte(ev.CIDR()),
				}
				if _, err := kv.Put(&p, nil); err != nil {
					return errors.Wrapf(err, "failed to put to consul key=%s", key)
				}
			} else {
				log.Printf("[info] delete from consul key=%s", key)
				if _, err = kv.Delete(key, nil); err != nil {
					return errors.Wrapf(err, "failed to delete from consul key=%s", key)
				}
			}
		}
	}
	return nil
}

func (s *streamer) updateSecurityGroup(groups []*SecurityGroupConfig, v4Events []ipSetEvent, v6Events []ipSetEvent) error {
	description := aws.String(
		fmt.Sprintf(
			"by lambda function %s at %s",
			os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
			time.Now().Format(time.RFC3339),
		),
	)
	for _, gc := range groups {
		authorize := &ec2.IpPermission{
			FromPort:   aws.Int64(gc.FromPort),
			ToPort:     aws.Int64(gc.ToPort),
			IpProtocol: aws.String(gc.Protocol),
		}
		revoke := &ec2.IpPermission{
			FromPort:   aws.Int64(gc.FromPort),
			ToPort:     aws.Int64(gc.ToPort),
			IpProtocol: aws.String(gc.Protocol),
		}
		for _, ev := range v4Events {
			if ev.add {
				authorize.IpRanges = append(authorize.IpRanges, &ec2.IpRange{
					CidrIp:      aws.String(ev.CIDR()),
					Description: description,
				})
			} else {
				revoke.IpRanges = append(revoke.IpRanges, &ec2.IpRange{
					CidrIp:      aws.String(ev.CIDR()),
					Description: description,
				})
			}
		}
		for _, ev := range v6Events {
			if ev.add {
				authorize.Ipv6Ranges = append(authorize.Ipv6Ranges, &ec2.Ipv6Range{
					CidrIpv6:    aws.String(ev.CIDR()),
					Description: description,
				})
			} else {
				revoke.Ipv6Ranges = append(revoke.Ipv6Ranges, &ec2.Ipv6Range{
					CidrIpv6:    aws.String(ev.CIDR()),
					Description: description,
				})
			}
		}
		if len(authorize.IpRanges) > 0 || len(authorize.Ipv6Ranges) > 0 {
			log.Printf("[debug] authorizing security group(%s) %s", gc.ID, JSONString(authorize))
			_, err := s.ec2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
				GroupId:       aws.String(gc.ID),
				IpPermissions: []*ec2.IpPermission{authorize},
			})
			if err != nil {
				errors.Wrapf(err, "failed to AuthorizeSecurityGroupIngress for %s", gc.ID)
			}
			log.Printf("[info] authorized security group(%s) %s", gc.ID, JSONString(authorize))
		}
		if len(revoke.IpRanges) > 0 || len(revoke.Ipv6Ranges) > 0 {
			log.Printf("[debug] revoking security group(%s) %s", gc.ID, JSONString(revoke))
			_, err := s.ec2.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
				GroupId:       aws.String(gc.ID),
				IpPermissions: []*ec2.IpPermission{revoke},
			})
			if err != nil {
				errors.Wrapf(err, "failed to RevokeSecurityGroupIngress for %s", gc.ID)
			}
			log.Printf("[info] revoked security group(%s) %s", gc.ID, JSONString(revoke))
		}
	}
	return nil
}
