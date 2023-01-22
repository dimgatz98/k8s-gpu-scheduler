package client

import (
	"context"
	"fmt"

	redis "github.com/go-redis/redis/v8"
)

var ctx = context.Background()

type (
	Client interface {
		Set(key string, value string) (err error)
		Get(key string) (val string, err error)
		GetRange(key string, start int64, end int64) (val string, err error)
		GetKeys() (interface{}, error)
	}
	Descriptor struct {
		Addr     string
		Password string
		DB       int
		Rdb      *redis.Client
	}
)

func (d *Descriptor) Set(key string, value string) (err error) {
	err = d.Rdb.Set(ctx, key, value, 0).Err()
	return err
}

func (d *Descriptor) Get(key string) (val string, err error) {
	val, err = d.Rdb.Get(ctx, key).Result()
	return val, err
}

func (d *Descriptor) GetRange(key string, start int64, end int64) (val string, err error) {
	val, err = d.Rdb.GetRange(ctx, key, start, end).Result()
	return val, err
}

func (d *Descriptor) GetKeys() (interface{}, error) {
	ret, err := d.Rdb.Do(ctx, "KEYS", "*").Result()
	if err != nil {
		return nil, err
	}
	fmt.Println(ret)
	return ret, nil
}

func New(addr string, password string, db int) *Descriptor {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	return &Descriptor{
		Addr:     addr,
		Password: password,
		DB:       db,
		Rdb:      rdb,
	}
}
