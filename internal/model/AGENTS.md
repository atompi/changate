# internal/model

## OVERVIEW
事件结构体和Agent响应模型。

## STRUCTURE
```
model/
├── event.go           # 飞书事件结构体
├── agent.go           # Agent响应结构体
└── model_test.go
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `MessageEvent` | struct | 飞书消息事件 |
| `EventCallbackRequest` | struct | 回调请求体 |
| `URLVerificationRequest` | struct | URL验证请求 |
| `OpenResponsesResponse` | struct | OpenResponses响应 |
| `ChatCompletionsResponse` | struct | ChatCompletions响应 |
| `MessageContentPart` | struct | 消息内容部分(文本/图片) |
| `ParseMessageContent` | func | 解析消息内容 |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| 飞书事件结构 | `event.go` | MessageEvent/Header/Sender/Message |
| Agent响应 | `agent.go` | OpenResponsesResponse/ChatCompletionsResponse |
| 消息解析 | `model.go` | ParseMessageContent函数 |
| ContentPart格式 | `agent.go` | OpenResponses/ChatCompletions内容部分格式 |

## CONVENTIONS (THIS PACKAGE)
- 图片消息: type="input_image", key存储image_key
- 文本消息: type="input_text", text存储内容
- Agent响应包含MEDIA:/path/to/file时触发文件上传
- OpenResponses使用content parts格式，ChatCompletions使用image_url格式