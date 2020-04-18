package knockrd

import (
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

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
	Expires   int64
}

func NewDynamoDBBackend(conf *Config) (Backend, error) {
	awsCfg := &aws.Config{
		Region: aws.String(conf.AWS.Region),
	}
	if conf.AWS.Endpoint != "" {
		awsCfg.Endpoint = aws.String(conf.AWS.Endpoint)
	}
	db := dynamo.New(session.New(), awsCfg)
	name := conf.TableName
	if _, err := db.Table(conf.TableName).Describe().Run(); err != nil {
		log.Printf("describe table %s failed, creating", name)
		// table not exists
		if err := db.CreateTable(name, Item{}).OnDemand(true).Stream(dynamo.KeysOnlyView).Run(); err != nil {
			return nil, err
		}
		if err := db.Table(name).UpdateTTL("Expires", true).Run(); err != nil {
			return nil, err
		}
	}
	return &DynamoDBBackend{
		db:        db,
		TableName: name,
		Expires:   conf.Expires,
	}, nil
}

func (d *DynamoDBBackend) Get(key string) (bool, error) {
	table := d.db.Table(d.TableName)
	var item Item
	if err := table.Get("Key", key).One(&item); err != nil {
		if strings.Contains(err.Error(), "no item found") {
			// expired or not found
			err = nil
		}
		return false, err
	}
	ts := time.Now().Unix()
	log.Printf("[debug] key:%s expires:%d remain:%d sec", key, item.Expires, item.Expires-ts)
	return ts <= item.Expires, nil
}

func (d *DynamoDBBackend) Set(key string) error {
	expires := time.Now().Unix() + d.Expires
	table := d.db.Table(d.TableName)
	item := Item{
		Key:     key,
		Expires: expires,
	}
	return table.Put(item).Run()
}

func (d *DynamoDBBackend) Delete(key string) error {
	table := d.db.Table(d.TableName)
	log.Printf("[debug] deleting %s", key)
	return table.Delete("Key", key).Run()
}
