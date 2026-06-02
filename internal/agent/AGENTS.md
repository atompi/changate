# internal/agent

## OVERVIEW
OpenResponses/ChatCompletions HTTP 客户端封装，支持两种 Agent 类型。客户端在构造时绑定具体 builder 指针，无 `any`/`interface{}` 类型断言；共享调用管线通过顶级 `doCall[T any]` 泛型函数实现。

## STRUCTURE
```
agent/
├── client.go        # Client interface + Config struct + NewClient factory
├── agent_http.go    # agentHTTPClient + 2 builders + doCall generic
└── agent_test.go    # httptest + table-driven unit tests
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `Client` | interface | Agent 客户端接口（仅暴露 `*WithContent` 变体） |
| `Config` | struct | NewClient 工厂参数 |
| `NewClient` | func | 工厂函数，根据 `AgentType` 选择 builder；返回 error |
| `TypeOpenResponses` / `TypeChatCompletions` | const | AgentType 取值常量 |
| `agentHTTPClient` | struct | 统一 HTTP 客户端实现 |
| `responsesBuilder` | struct | OpenResponses API builder |
| `chatCompletionsBuilder` | struct | ChatCompletions API builder |
| `doCall` | func | 顶级泛型函数 build→retry→read→parse 共享管线 |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| 添加新 Agent 类型 | `client.go` | 加常量 + 在 `newAgentHTTPClient` switch 中注册 builder |
| 修改 OpenResponses 请求/响应 | `agent_http.go` | `responsesBuilder.buildRequest/parseResponse` |
| 修改 ChatCompletions 请求/响应 | `agent_http.go` | `chatCompletionsBuilder.buildRequest/parseResponse` |
| 修改 HTTP/重试逻辑 | `agent_http.go` | `agentHTTPClient.executeWithRetry/doHTTP` |
| 共享调用管线 | `agent_http.go` | 顶级 `doCall[T any]` 函数 |
| Agent 响应模型 | `internal/model/agent.go` | `OpenResponsesResponse` / `ChatCompletionsResponse` |

## CONVENTIONS (THIS PACKAGE)
- `AgentType` 为 `TypeChatCompletions` 时使用 chatCompletionsBuilder；`TypeOpenResponses` 或空字符串使用 responsesBuilder
- 未知 `AgentType` → `NewClient` 返回 error
- `BaseURL` 为空 → `NewClient` 返回 error（绝不静默回落）
- 字段：baseURL, apiPath, maxRetries, retryBaseDelay（camelCase，匹配 config 风格）
- HTTP 超时默认 120s（`defaultHTTPTimeout`），可通过 Config.Timeout 覆盖
- 重试条件：网络错误 + 5xx + 429；4xx 不重试；指数退避
- 响应体读取上限 `maxResponseBodyBytes = 10 MiB`（`readBoundedResponse`）
- 日志中请求/响应体经 `truncateForLog` 截断到 2 KiB，避免 PII/大体积 base64
- 错误消息只含 status code，不含完整响应体
- MCP tools 通过 `Config.Tools`；`tool_choice` 通过 `Config.ToolChoice`
- 共享 `requestBase` struct 消除 `responsesRequest` / `chatCompletionsRequest` 字段重复
- 未知 content type：convertModelMessageToInput 返回 error（不再静默 `%v` 格式化）
- 未知 content part type：slog.Warn 跳过（不静默丢弃）
- function_call / tool_use 输出：slog.Info 标记（暂未传播到 Feishu）
- Token usage 不一致：slog.Warn 标记
- body 所有权：doHTTP 失败时关闭 body；executeWithRetry 在 retry 循环内显式关闭每次响应；成功响应传给调用者（caller 用 defer 关闭）

## BUILDER PATTERN

```
agentHTTPClient
├── apiName:    "responses" | "chatcompletions"   (log label)
├── apiPath:    /v1/responses | /v1/chat/completions  (default)
├── responsesBld: *responsesBuilder    (set iff kind=Responses)
├── chatBld:      *chatCompletionsBuilder (set iff kind=ChatCompletions)
└── doCall[T any](c, ctx, msgs, parse) (shared build→retry→parse pipeline)
    ├── responsesBuilder.parseResponse   (*OpenResponsesResponse)
    └── chatCompletionsBuilder.parseResponse (*ChatCompletionsResponse)
```

无类型断言：builder 指针在 ctor 阶段绑定，调用方直接传 `c.responsesBld.parseResponse` 或 `c.chatBld.parseResponse` 给泛型 `doCall`。
