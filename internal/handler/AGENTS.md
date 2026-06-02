# internal/handler

## OVERVIEW
飞书回调处理核心逻辑：解密、签名验证、消息分发、异步 Agent 调用、Agent 客户端 LRU+TTL 缓存。

## STRUCTURE
```
handler/
├── callback.go             # CallbackHandler 主逻辑
├── agent_cache.go          # LRU+TTL Agent 客户端缓存（O(1) 驱逐）
└── agent_cache_test.go     # 缓存单元测试
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `CallbackHandler` | struct | 核心处理器，管理 semaphores 和 agentCache |
| `NewCallbackHandler` | func | 构造函数 |
| `HandleCallback` | method | HTTP 入口处理飞书回调 |
| `processMessageAsync` | method | 异步处理消息，调用 Agent |
| `WaitForCompletion` | method | 等待所有活跃 goroutine 结束（用于 graceful shutdown） |
| `cacheKey` | struct | `{appName, userID}` 复合 key |
| `cachedClient` | struct | 缓存条目：client + expiresAt + key（驱逐用） |
| `AgentCache` | struct | LRU+TTL 缓存 |
| `NewAgentCache` | func | 构造（maxSize, ttl） |
| `GetOrCreate` | method | 取缓存；miss 时调 factory 并存 |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| 回调处理入口 | `callback.go` `HandleCallback` | POST /feishu/:appName |
| 签名验证 | `callback.go` `verifySignature` | HMAC-SHA256 |
| 消息解密 | `callback.go` `decryptBody` | AES-256-CBC (Feishu encrypt) |
| 异步消息处理 | `callback.go` `processMessageAsync` | goroutine + per-app semaphore |
| 内容转换 | `callback.go` `processMessageContentPartsTo*` | text/image 转换为 Agent API 格式 |
| Agent 缓存 | `agent_cache.go` | LRU 驱逐 O(1) 通过 cachedClient.key 反查 |

## CONVENTIONS (THIS PACKAGE)
- 回调立即返回 200 OK，避免飞书超时；实际处理在 goroutine 中
- 异步处理使用 per-app semaphore 限制并发（来自 `app.MaxConcurrent`）
- AgentClient 通过缓存获取，key = `{appName, userID}`
- 图片消息需先从 Feishu 下载再 base64 编码后发送到 Agent
- `processMessageContentPartsTo*ContentParts` 函数：text 直接映射，image 需调用 `feishuClient.DownloadMessageResource` + base64
- LRU 缓存：每条目记录 `key` 字段，驱逐时 O(1) 从 map 删除（避免遍历全表）
- `WaitForCompletion` 轮询 `active` 原子计数器（10ms 间隔），用于 graceful shutdown
- `handleMessageEvent` 立即 `c.JSON(200, {code:0})` 然后 async 处理；Feishu 重试由 Agent 端保证
