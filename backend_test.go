package knockrd_test

import (
	"crypto/md5"
	"fmt"
	"testing"
	"time"

	"github.com/fujiwara/knockrd"
)

var conf *knockrd.Config

func init() {
	testing.Init()
	var err error
	conf, err = knockrd.LoadConfig("test/config.yaml")
	if err != nil {
		panic(err)
	}
	_, _, err = conf.Setup()
	if err != nil {
		panic(err)
	}
}
func TestDynamoDBBackend(t *testing.T) {
	dynamo, err := knockrd.NewDynamoDBBackend(conf)
	if err != nil {
		t.Error(err)
	}
	testBackend(t, dynamo, "")
}

func TestDynamoDBBackendNoCache(t *testing.T) {
	dynamo, err := knockrd.NewDynamoDBBackend(conf)
	if err != nil {
		t.Error(err)
	}
	testBackend(t, dynamo, knockrd.NoCachePrefix)
}

func TestCachedBackend(t *testing.T) {
	dynamo, err := knockrd.NewDynamoDBBackend(conf)
	if err != nil {
		t.Error(err)
	}
	cached, err := knockrd.NewCachedBackend(dynamo, conf.CacheTTL)
	if err != nil {
		t.Error(err)
	}
	testBackend(t, cached, "")
}

func TestCachedBackendNoCache(t *testing.T) {
	dynamo, err := knockrd.NewDynamoDBBackend(conf)
	if err != nil {
		t.Error(err)
	}
	cached, err := knockrd.NewCachedBackend(dynamo, conf.CacheTTL)
	if err != nil {
		t.Error(err)
	}
	testBackend(t, cached, knockrd.NoCachePrefix)
}

func testBackend(t *testing.T, b knockrd.Backend, prefix string) {
	key := prefix + fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%v%v", t, b))))
	for _ = range []int{0, 1} {
		if ok, err := b.Get(key); err != nil {
			t.Error(err)
		} else if ok {
			t.Errorf("unexpected %s found", key)
		}
	}

	if err := b.Set(key); err != nil {
		t.Error(err)
	}
	defer func(key string) {
		if err := b.Delete(key); err != nil {
			t.Error(err)
		}
	}(key)

	if ok, err := b.Get(key); err != nil {
		t.Error(err)
	} else if !ok {
		t.Errorf("unexpected %s not found", key)
	}

	time.Sleep(conf.TTL / 2)
	if ok, err := b.Get(key); err != nil {
		t.Error(err)
	} else if !ok {
		t.Errorf("unexpected %s not found", key)
	}

	time.Sleep(conf.TTL)
	if ok, err := b.Get(key); err != nil {
		t.Error(err)
	} else if ok {
		t.Errorf("unexpected %s found", key)
	}
}
