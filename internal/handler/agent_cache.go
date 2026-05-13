package handler

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/atompi/changate/internal/agent"
	"github.com/atompi/changate/internal/config"
)

type cacheKey struct {
	appName string
	userID  string
}

type cachedClient struct {
	client    agent.Client
	expiresAt time.Time
}

type AgentCache struct {
	mu      sync.Mutex
	cache   map[string]*list.Element
	lru     *list.List
	maxSize int
	ttl     time.Duration
}

func NewAgentCache(maxSize int, ttl time.Duration) *AgentCache {
	return &AgentCache{
		cache:   make(map[string]*list.Element),
		lru:     list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func (c *AgentCache) Get(key cacheKey) agent.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	fullKey := key.appName + "|" + key.userID
	elem, ok := c.cache[fullKey]
	if !ok {
		return nil
	}

	cached := elem.Value.(*cachedClient)
	if time.Now().After(cached.expiresAt) {
		c.lru.Remove(elem)
		delete(c.cache, fullKey)
		return nil
	}

	c.lru.MoveToFront(elem)
	return cached.client
}

func (c *AgentCache) Set(key cacheKey, client agent.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fullKey := key.appName + "|" + key.userID

	if elem, ok := c.cache[fullKey]; ok {
		c.lru.MoveToFront(elem)
		elem.Value.(*cachedClient).client = client
		elem.Value.(*cachedClient).expiresAt = time.Now().Add(c.ttl)
		return
	}

	if c.lru.Len() >= c.maxSize {
		oldest := c.lru.Back()
		if oldest != nil {
			c.lru.Remove(oldest)
			// Note: Simple deletion - in production you'd track the fullKey
			for k := range c.cache {
				if c.cache[k] == oldest {
					delete(c.cache, k)
					break
				}
			}
		}
	}

	elem := c.lru.PushFront(&cachedClient{
		client:    client,
		expiresAt: time.Now().Add(c.ttl),
	})
	c.cache[fullKey] = elem
}

func (c *AgentCache) GetOrCreate(ctx context.Context, key cacheKey, cfg *config.AppConfig, factory func(*config.AppConfig) agent.Client) agent.Client {
	if client := c.Get(key); client != nil {
		return client
	}

	client := factory(cfg)
	c.Set(key, client)
	return client
}
