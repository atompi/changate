package handler

import (
	"context"
	"testing"
	"time"

	"github.com/atompi/changate/internal/agent"
	"github.com/atompi/changate/internal/config"
	"github.com/atompi/changate/internal/model"
)

type mockAgentClient struct{}

func (m *mockAgentClient) ChatCompletionsWithContent(ctx context.Context, contentParts []model.ChatCompletionsContentPart) (*model.ChatCompletionsResponse, error) {
	return nil, nil
}

func (m *mockAgentClient) OpenResponsesWithContent(ctx context.Context, contentParts []model.OpenResponsesContentPart) (*model.OpenResponsesResponse, error) {
	return nil, nil
}

func TestNewAgentCache(t *testing.T) {
	cache := NewAgentCache(100, 30*time.Second)
	if cache == nil {
		t.Fatal("NewAgentCache returned nil")
	}
	if cache.maxSize != 100 {
		t.Errorf("maxSize = %d, want 100", cache.maxSize)
	}
	if cache.ttl != 30*time.Second {
		t.Errorf("ttl = %v, want 30s", cache.ttl)
	}
}

func TestAgentCache_Get_NotFound(t *testing.T) {
	cache := NewAgentCache(100, 30*time.Second)
	key := cacheKey{appName: "app1", userID: "user1"}

	client := cache.Get(key)
	if client != nil {
		t.Errorf("Get() returned %v, want nil", client)
	}
}

func TestAgentCache_SetAndGet(t *testing.T) {
	cache := NewAgentCache(100, 30*time.Second)
	key := cacheKey{appName: "app1", userID: "user1"}

	cache.Set(key, &mockAgentClient{})

	client := cache.Get(key)
	if client == nil {
		t.Error("Get() returned nil for cached entry")
	}
}

func TestAgentCache_Get_Expired(t *testing.T) {
	cache := NewAgentCache(100, 10*time.Millisecond)
	key := cacheKey{appName: "app1", userID: "user1"}

	cache.Set(key, &mockAgentClient{})
	time.Sleep(20 * time.Millisecond)

	client := cache.Get(key)
	if client != nil {
		t.Error("Get() returned non-nil for expired entry")
	}
}

func TestAgentCache_LRU_Eviction(t *testing.T) {
	cache := NewAgentCache(2, 30*time.Second)

	key1 := cacheKey{appName: "app1", userID: "user1"}
	key2 := cacheKey{appName: "app2", userID: "user2"}
	key3 := cacheKey{appName: "app3", userID: "user3"}

	cache.Set(key1, &mockAgentClient{})
	cache.Set(key2, &mockAgentClient{})
	cache.Set(key3, &mockAgentClient{})

	if cache.Get(key1) != nil {
		t.Error("key1 should have been evicted")
	}
	if cache.Get(key2) == nil {
		t.Error("key2 should still be in cache")
	}
}

func TestAgentCache_UpdateExisting(t *testing.T) {
	cache := NewAgentCache(100, 30*time.Second)
	key := cacheKey{appName: "app1", userID: "user1"}

	client1 := &mockAgentClient{}
	client2 := &mockAgentClient{}

	cache.Set(key, client1)
	cache.Set(key, client2)

	client := cache.Get(key)
	if client != client2 {
		t.Errorf("Get() did not return updated client")
	}
}

func TestAgentCache_GetOrCreate_CreatesClient(t *testing.T) {
	cache := NewAgentCache(100, 30*time.Second)
	key := cacheKey{appName: "app1", userID: "user1"}

	factoryCalls := 0
	factory := func(*config.AppConfig) agent.Client {
		factoryCalls++
		return &mockAgentClient{}
	}

	client := cache.GetOrCreate(context.Background(), key, &config.AppConfig{}, factory)
	if client == nil {
		t.Error("GetOrCreate() returned nil")
	}
	if factoryCalls != 1 {
		t.Errorf("factory called %d times, want 1", factoryCalls)
	}
}

func TestAgentCache_GetOrCreate_ReusesExisting(t *testing.T) {
	cache := NewAgentCache(100, 30*time.Second)
	key := cacheKey{appName: "app1", userID: "user1"}

	factoryCalls := 0
	factory := func(*config.AppConfig) agent.Client {
		factoryCalls++
		return &mockAgentClient{}
	}

	client1 := cache.GetOrCreate(context.Background(), key, &config.AppConfig{}, factory)
	client2 := cache.GetOrCreate(context.Background(), key, &config.AppConfig{}, factory)

	if client1 != client2 {
		t.Error("GetOrCreate() should return same client on second call")
	}
	if factoryCalls != 1 {
		t.Errorf("factory called %d times, want 1 (cache miss should be 1 only)", factoryCalls)
	}
}

func TestAgentCache_ContainsKey(t *testing.T) {
	cache := NewAgentCache(2, 30*time.Second)
	key1 := cacheKey{appName: "app1", userID: "user1"}
	key2 := cacheKey{appName: "app2", userID: "user2"}
	key3 := cacheKey{appName: "app3", userID: "user3"}

	cache.Set(key1, &mockAgentClient{})
	cache.Set(key2, &mockAgentClient{})
	cache.Set(key3, &mockAgentClient{})

	if cache.containsKey(key1) {
		t.Error("key1 should have been evicted from map when LRU evicted it")
	}
	if !cache.containsKey(key2) {
		t.Error("key2 should be in map")
	}
}

func (c *AgentCache) containsKey(key cacheKey) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.cache[key.String()]
	return ok
}
