# internal/handler

## OVERVIEW
飞书回调处理核心逻辑：解密、签名验证、消息分发、异步Agent调用。

## STRUCTURE
```
handler/
├── callback.go        # CallbackHandler主逻辑
├── agent_cache.go     # LRU+TTL Agent客户端缓存
└── agent_cache_test.go
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `CallbackHandler` | struct | 核心处理器，管理semaphore和agentCache |
| `NewCallbackHandler` | func | 构造函数 |
| `HandleCallback` | method | HTTP入口处理飞书回调 |
| `processMessageAsync` | method | 异步处理消息，调用Agent |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| 回调处理入口 | `callback.go:63` | HandleCallback函数 |
| 签名验证逻辑 | `callback.go:187` | verifySignature方法 |
| 消息解密 | `callback.go:167` | decryptBody方法 |
| 异步处理 | `callback.go:239` | processMessageAsync，gofunc处理 |
| Agent缓存 | `agent_cache.go` | LRU TTL缓存 |

## CONVENTIONS (THIS PACKAGE)
- 回调立即返回200，避免飞书超时
- 异步处理使用per-app semaphore限制并发
- AgentClient通过缓存获取，key={appName, userID}
- 图片消息需要先下载再处理