package knockrd

import (
	"context"
	"log"
	"net"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/wafv2"
	mapset "github.com/deckarep/golang-set"
)

type streamer struct {
	conf *Config
	svc  *wafv2.WAFV2
}

func newStreamHandler(conf *Config) func(context.Context, events.DynamoDBEvent) error {
	awsCfg := &aws.Config{
		Region: aws.String(conf.AWS.Region),
	}
	if conf.AWS.Endpoint != "" {
		awsCfg.Endpoint = aws.String(conf.AWS.Endpoint)
	}
	svc := wafv2.New(session.New(), awsCfg)

	s := &streamer{
		conf: conf,
		svc:  svc,
	}

	return s.Handler
}

type ipSetEvent struct {
	address string
	add     bool
}

func (s *streamer) Handler(ctx context.Context, event events.DynamoDBEvent) error {
	var v4, v6 []ipSetEvent
	for _, r := range event.Records {
		key, ok := r.Change.Keys["Key"]
		if !ok {
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
			v4 = append(v4, ipSetEvent{ipv4.String() + "/32", add})
		} else {
			switch r.EventName {
			case "INSERT", "MODIFY":
				add = true
			case "REMOVE":
				v6 = append(v6, ipSetEvent{ip.String() + "/128", false})
			default:
				log.Printf("[warn] unknown event %s", r.EventName)
				continue
			}
			log.Printf("[debug] IPV6 %s add %t", ip.String(), add)
			v6 = append(v6, ipSetEvent{ip.String() + "/128", add})
		}
	}
	if err := s.updateIPSet(s.conf.IPSet.V4, v4); err != nil {
		return err
	}
	if err := s.updateIPSet(s.conf.IPSet.V6, v6); err != nil {
		return err
	}
	return nil
}

func (s *streamer) updateIPSet(c IPSetConfig, events []ipSetEvent) error {
	if c.ID == "" || len(events) == 0 {
		return nil
	}
	res, err := s.svc.GetIPSet(&wafv2.GetIPSetInput{
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
		ad := ad
		addrs.Add(*ad)
	}
	log.Printf("[debug] current addresses %s", addrs.String())
	for _, e := range events {
		if e.add {
			addrs.Add(e.address)
		} else {
			addrs.Remove(e.address)
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
	_, err = s.svc.UpdateIPSet(&wafv2.UpdateIPSetInput{
		Name:      &c.Name,
		Id:        &c.ID,
		Scope:     &c.Scope,
		Addresses: updates,
		LockToken: lockToken,
	})
	return err
}
