package cache

import (
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

var g = &singleflight.Group{}

type Cache struct {
	capcity int64
	size    atomic.Int64
	data    *sync.Map
}

type item struct {
	expire int64
	value  interface{}
}

func NewCache(capcity int64) *Cache {
	capcity = max(capcity, 1)
	return &Cache{
		capcity: capcity,
		data:    &sync.Map{},
		size:    atomic.Int64{},
	}
}

type Option func(v *item)

func (c *Cache) Put(key, value interface{}, opts ...Option) {
	val := &item{
		value: value,
	}
	for _, opt := range opts {
		opt(val)
	}

	if !(c.size.Load() < c.capcity) {
		g.Do("clean", func() (interface{}, error) {
			if c.size.Load() < c.capcity {
				return nil, nil
			}
			plan := max(c.capcity>>2, 1)
			done := int64(0)
			c.data.Range(func(key, value interface{}) bool {
				c.data.Delete(key)
				c.size.Add(-1)
				done++
				return done < plan
			})
			return nil, nil
		})
	}

	if _, ok := c.data.LoadOrStore(key, val); !ok {
		c.size.Add(1)
		return
	}
	c.data.Store(key, val)
}

func (c *Cache) Get(key interface{}) (interface{}, bool) {
	vali, ok := c.data.Load(key)
	if !ok {
		return nil, false
	}
	val, _ := vali.(*item)
	if !(time.Now().Compare(time.Unix(val.expire, 0)) > 0) {
		return val.value, true
	}
	c.data.Delete(key)
	return nil, false
}

func WithExpire(d time.Duration) Option {
	return func(val *item) {
		val.expire = time.Now().Add(d).Unix()
	}
}
