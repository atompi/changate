# internal/agent

## OVERVIEW
OpenResponses/ChatCompletions 客户端封装，支持两种Agent类型。

## STRUCTURE
```
agent/
├── client.go          # Client接口 + NewClient工厂
├── responses.go       # OpenResponses客户端
├── chatcompletions.go # ChatCompletions客户端
└── client_test.go     # 测试
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `Client` | interface | Agent客户端接口 |
| `NewClient` | func | 工厂函数，根据agentType创建对应客户端 |
| `OpenResponsesClient` | struct | OpenResponses实现 |
| `ChatCompletionsClient` | struct | ChatCompletions实现 |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| 添加新Agent类型 | `client.go` | 修改NewClient工厂 |
| 修改OpenResponses | `responses.go` | 需同步修改processMessageContentPartsToOpenResponsesContentParts |
| 修改ChatCompletions | `chatcompletions.go` | 需同步修改processMessageContentPartsToChatCompletionsContentParts |
| Agent响应解析 | `model.OpenResponsesResponse` / `model.ChatCompletionsResponse` | 在 internal/model/agent.go |

## CONVENTIONS (THIS PACKAGE)
- AgentType为"ChatCompletions"时使用ChatCompletions客户端，否则默认OpenResponses
- timeout默认3600s，maxRetries默认3，retryBaseDelay默认100ms
- 响应格式: `GetContent()` 获取文本, `GetFilePath()` 获取文件路径(如果有MEDIA:前缀)