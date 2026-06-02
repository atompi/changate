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

func (k cacheKey) String() string {
	return k.appName + "|" + k.userID
}

type cachedClient struct {
	client    agent.Client
	expiresAt time.Time
	key       cacheKey
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

	elem, ok := c.cache[key.String()]
	if !ok {
		return nil
	}

	cached := elem.Value.(*cachedClient)
	if time.Now().After(cached.expiresAt) {
		c.removeElement(elem)
		return nil
	}

	c.lru.MoveToFront(elem)
	return cached.client
}

func (c *AgentCache) Set(key cacheKey, client agent.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key.String()]; ok {
		cached := elem.Value.(*cachedClient)
		cached.client = client
		cached.expiresAt = time.Now().Add(c.ttl)
		c.lru.MoveToFront(elem)
		return
	}

	if c.lru.Len() >= c.maxSize {
		if oldest := c.lru.Back(); oldest != nil {
			c.removeElement(oldest)
		}
	}

	elem := c.lru.PushFront(&cachedClient{
		client:    client,
		expiresAt: time.Now().Add(c.ttl),
		key:       key,
	})
	c.cache[key.String()] = elem
}

func (c *AgentCache) GetOrCreate(ctx context.Context, key cacheKey, cfg *config.AppConfig, factory func(*config.AppConfig) agent.Client) agent.Client {
	if client := c.Get(key); client != nil {
		return client
	}

	client := factory(cfg)
	c.Set(key, client)
	return client
}

func (c *AgentCache) removeElement(elem *list.Element) {
	cached := elem.Value.(*cachedClient)
	delete(c.cache, cached.key.String())
	c.lru.Remove(elem)
}
