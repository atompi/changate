# internal/agent

## OVERVIEW
OpenResponses/ChatCompletions HTTP客户端封装，支持两种Agent类型。使用Builder模式统一处理请求构建和响应解析。

## STRUCTURE
```
agent/
├── client.go       # Client接口 + NewClient工厂
├── agent_http.go   # 统一HTTP客户端 + requestBuilder实现
└── client_test.go  # 测试
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `Client` | interface | Agent客户端接口 |
| `NewClient` | func | 工厂函数，根据agentType创建对应客户端 |
| `agentHTTPClient` | struct | 统一HTTP客户端实现 |
| `requestBuilder` | interface | 请求构建器接口(responsesBuilder/chatCompletionsBuilder) |
| `responsesBuilder` | struct | OpenResponses API请求构建器 |
| `chatCompletionsBuilder` | struct | ChatCompletions API请求构建器 |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| 添加新Agent类型 | `client.go` | 修改NewClient工厂，添加新的builder |
| 修改OpenResponses | `agent_http.go` | responsesBuilder.buildRequest/parseResponse |
| 修改ChatCompletions | `agent_http.go` | chatCompletionsBuilder.buildRequest/parseResponse |
| 修改HTTP客户端逻辑 | `agent_http.go` | agentHTTPClient.doRequest/doRequestWithRetry |
| Agent响应解析 | `model.OpenResponsesResponse` / `model.ChatCompletionsResponse` | 在 internal/model/agent.go |

## CONVENTIONS (THIS PACKAGE)
- AgentType为"ChatCompletions"时使用chatCompletionsBuilder，否则默认responsesBuilder
- timeout默认3600s，maxRetries默认3，retryBaseDelay默认100ms
- 响应格式: `GetContent()` 获取文本, `GetFilePath()` 获取文件路径(如果有MEDIA:前缀)
- 使用原始HTTP请求，debug级别打印请求/响应体
- MCP tools通过AgentConfig.Tools配置，token从AgentConfig.Token获取
- 请求头添加"Bearer "前缀
- 重试逻辑: 网络错误和5xx响应会指数退避重试

## BUILDER PATTERN

统一客户端通过requestBuilder接口支持不同API格式：

```
agentHTTPClient
├── OpenResponses() → responsesBuilder.buildRequest() → JSON Input格式
├── ChatCompletions() → chatCompletionsBuilder.buildRequest() → JSON Messages格式
└── doRequestWithRetry() → pkg/retry.Do() with exponential backoff
```