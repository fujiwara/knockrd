package knockrd

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

var Timeout = 30 * time.Second

type Backend interface {
	Set(string) error
	Get(string) (bool, error)
	Delete(string) error
}

type Item struct {
	Key     string `dynamo:"Key,hash"`
	Expires int64  `dynamo:"Expires"`
}

type DynamoDBBackend struct {
	db        *dynamo.DB
	TableName string
	TTL       time.Duration
}

func NewDynamoDBBackend(conf *Config) (Backend, error) {
	log.Println("[debug] initialize dynamodb backend")
	awsCfg := &aws.Config{
		Region: aws.String(conf.AWS.Region),
	}
	if conf.AWS.Endpoint != "" {
		awsCfg.Endpoint = aws.String(conf.AWS.Endpoint)
	}
	db := dynamo.New(session.New(), awsCfg)
	name := conf.TableName
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	if _, err := db.Table(conf.TableName).Describe().RunWithContext(ctx); err != nil {
		log.Printf("[info] describe table %s failed, creating", name)
		// table not exists
		if err := db.CreateTable(name, Item{}).OnDemand(true).Stream(dynamo.KeysOnlyView).RunWithContext(ctx); err != nil {
			return nil, err
		}
		if err := db.Table(name).UpdateTTL("Expires", true).RunWithContext(ctx); err != nil {
			return nil, err
		}
	}
	return &DynamoDBBackend{
		db:        db,
		TableName: name,
		TTL:       conf.TTL,
	}, nil
}

func (d *DynamoDBBackend) Get(key string) (bool, error) {
	table := d.db.Table(d.TableName)
	var item Item
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	log.Printf("[debug] get %s from dynamodb", key)
	if err := table.Get("Key", key).OneWithContext(ctx, &item); err != nil {
		if strings.Contains(err.Error(), "no item found") {
			// expired or not found
			err = nil
		}
		return false, err
	}
	ts := time.Now().Unix()
	log.Printf("[debug] got %s from dynamodb expires:%d remain:%d sec", key, item.Expires, item.Expires-ts)
	return ts <= item.Expires, nil
}

func (d *DynamoDBBackend) Set(key string) error {
	expireAt := time.Now().Add(d.TTL)
	table := d.db.Table(d.TableName)
	item := Item{
		Key:     key,
		Expires: expireAt.Unix(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	log.Printf("[debug] set %s to dynamodb", key)
	return table.Put(item).RunWithContext(ctx)
}

func (d *DynamoDBBackend) Delete(key string) error {
	table := d.db.Table(d.TableName)
	log.Printf("[debug] delete %s from dynamodb", key)
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	return table.Delete("Key", key).RunWithContext(ctx)
}

type CachedBackend struct {
	backend     Backend
	cache       *ttlcache.Cache
	negativeTTL time.Duration
}

func NewCachedBackend(backend Backend, defaultTTL, negativeTTL time.Duration) (Backend, error) {
	c := ttlcache.NewCache()
	c.SetTTL(defaultTTL)
	log.Printf("[debug] new cached backend defaultTTL:%s negativeTTL:%s", defaultTTL, negativeTTL)
	return &CachedBackend{
		backend:     backend,
		cache:       c,
		negativeTTL: negativeTTL,
	}, nil
}

func (b *CachedBackend) Set(key string) error {
	log.Printf("[debug] set %s to backend", key)
	if err := b.backend.Set(key); err != nil {
		b.cache.Remove(key)
		return err
	}
	log.Printf("[debug] set %s to cache", key)
	b.cache.Set(key, struct{}{})
	return nil
}

func (b *CachedBackend) Get(key string) (bool, error) {
	log.Printf("[debug] get %s from cache", key)
	if v, ok := b.cache.Get(key); ok {
		log.Printf("[debug] hit %s in cache (negative=%t)", key, v == nil)
		return v != nil, nil
	}
	log.Printf("[debug] miss %s in cache", key)
	if ok, err := b.backend.Get(key); err != nil {
		return false, err
	} else if ok {
		log.Printf("[debug] set %s to cache", key)
		b.cache.Set(key, struct{}{})
		return true, nil
	}

	log.Printf("[debug] set %s to negative cache", key)
	b.cache.SetWithTTL(key, nil, b.negativeTTL)
	return false, nil
}

func (b *CachedBackend) Delete(key string) error {
	log.Printf("[debug] delete %s from cache", key)
	b.cache.Remove(key)
	return b.backend.Delete(key)
}
