package knockrd

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"path"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/wafv2"
	mapset "github.com/deckarep/golang-set"
	consul "github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
)

var DefaultConsulKVPath = "knockrd/allowed"

type streamer struct {
	conf        *Config
	svcRegional *wafv2.WAFV2
	svcCF       *wafv2.WAFV2
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
		conf:        conf,
		svcRegional: wafv2.New(session.New(), awsCfgRegional),
		svcCF:       wafv2.New(session.New(), awsCfgCF),
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

func (s *streamer) Handler(ctx context.Context, event events.DynamoDBEvent) error {
	var v4, v6 []ipSetEvent
	for _, r := range event.Records {
		key, ok := r.Change.Keys["Key"]
		if !ok {
			log.Printf("[warn] unkown key %v", r.Change.Keys)
			continue
		}
		ip := net.ParseIP(key.String())
		if ip == nil {
			log.Printf("[debug] ignore Key:%s", key.String())
			continue
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
				continue
			}
			log.Printf("[debug] IPV4 %s add %t", ip.String(), add)
			v4 = append(v4, ipSetEvent{ipv4.String(), add, true})
		} else {
			switch r.EventName {
			case "INSERT", "MODIFY":
				add = true
			case "REMOVE":
			default:
				log.Printf("[warn] unknown event %s", r.EventName)
				continue
			}
			log.Printf("[debug] IPV6 %s add %t", ip.String(), add)
			v6 = append(v6, ipSetEvent{ip.String(), add, false})
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
	return nil
}

func (s *streamer) updateIPSet(c *IPSetConfig, events []ipSetEvent) error {
	if c == nil || c.ID == "" || len(events) == 0 {
		return nil
	}
	var svc *wafv2.WAFV2
	switch c.Scope {
	case "REGIONAL":
		svc = s.svcRegional
	case "CLOUDFRONT":
		svc = s.svcCF
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
