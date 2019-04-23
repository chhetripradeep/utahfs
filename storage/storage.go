// Package storage implements handlers for compatible object-storage backends.
package storage

import (
	"errors"

	"github.com/Bren2010/utahfs"

	"github.com/hashicorp/golang-lru"
)

func dup(in []byte) []byte {
	if in == nil {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

type memory map[string][]byte

// NewMemory returns an object storage backend that simply stores data
// in-memory.
func NewMemory() utahfs.ObjectStorage {
	return make(memory)
}

func (m memory) Get(key string) ([]byte, error) {
	data, ok := m[key]
	if !ok {
		return nil, utahfs.ErrObjectNotFound
	}
	return dup(data), nil
}

func (m memory) Set(key string, data []byte) error {
	m[key] = dup(data)
	return nil
}

func (m memory) Delete(key string) error {
	delete(m, key)
	return nil
}

type retry struct {
	base     utahfs.ObjectStorage
	attempts int
}

// NewRetry wraps a base object storage backend, and will retry if requests
// fail.
func NewRetry(base utahfs.ObjectStorage, attempts int) (utahfs.ObjectStorage, error) {
	if attempts <= 0 {
		return nil, errors.New("storage: attempts must be greater than zero")
	}
	return &retry{base, attempts}, nil
}

func (r *retry) Get(key string) (data []byte, err error) {
	for i := 0; i < r.attempts; i++ {
		data, err = r.base.Get(key)
		if err == nil || err == utahfs.ErrObjectNotFound {
			return
		}
	}

	return
}

func (r *retry) Set(key string, data []byte) (err error) {
	for i := 0; i < r.attempts; i++ {
		err = r.base.Set(key, data)
		if err == nil {
			return
		}
	}

	return
}

func (r *retry) Delete(key string) (err error) {
	for i := 0; i < r.attempts; i++ {
		err = r.base.Delete(key)
		if err == nil {
			return
		}
	}

	return
}

type cache struct {
	base  utahfs.ObjectStorage
	cache *lru.Cache
}

// NewCache wraps a base object storage backend with an LRU cache of the
// requested size.
func NewCache(base utahfs.ObjectStorage, size int) (utahfs.ObjectStorage, error) {
	c, err := lru.New(size)
	if err != nil {
		return nil, err
	}
	return &cache{base, c}, nil
}

func (c *cache) Get(key string) ([]byte, error) {
	val, ok := c.cache.Get(key)
	if ok {
		return dup(val.([]byte)), nil
	}
	data, err := c.base.Get(key)
	if err != nil {
		return nil, err
	}
	c.cache.Add(key, dup(data))
	return data, nil
}

func (c *cache) Set(key string, data []byte) error {
	c.cache.Remove(key)
	if err := c.base.Set(key, data); err != nil {
		return err
	}
	c.cache.Add(key, dup(data))
	return nil
}

func (c *cache) Delete(key string) error {
	c.cache.Remove(key)
	return c.base.Delete(key)
}