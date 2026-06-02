# internal/model

## OVERVIEW
飞书事件结构体和 Agent 响应模型。`MessageContentPart.ImageData` 字段名澄清：实际承载 data URL（`data:image/...;base64,...`），而非普通 URL。

## STRUCTURE
```
model/
├── event.go           # 飞书事件结构体 + ParseMessageContent
├── agent.go           # Agent 请求/响应结构体 + GetContent/GetFilePath helpers
└── model_test.go      # ParseMessageContent/GetContent/GetFilePath 单元测试
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `MessageEvent` | struct | 飞书消息事件 |
| `EventCallbackRequest` | struct | 回调请求体 |
| `URLVerificationRequest` | struct | URL 验证请求 |
| `OpenResponsesResponse` | struct | OpenResponses 响应 (ID/Status/CreatedAt/Model/Output/Usage) |
| `ChatCompletionsResponse` | struct | ChatCompletions 响应 (ID/Object/Created/Model/Choices/Usage) |
| `MessageContentPart` | struct | 网关内部消息部分 (Type/Text/Key/ImageData) |
| `OpenResponsesContentPart` | struct | OpenResponses API 内容部分 (Type/Text/ImageData) |
| `ChatCompletionsContentPart` | struct | ChatCompletions API 内容部分 (Type/Text/ImageURL) |
| `ParseMessageContent` | func | 解析飞书消息内容为 `[]MessageContentPart` |
| `extractMediaPath` | func | 从响应文本中提取 `MEDIA:/path` 文件路径 |
| `GetContent` / `GetFilePath` | method | 统一从 OpenResponses / ChatCompletions 响应提取文本/文件路径 |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| 飞书事件结构 | `event.go` | MessageEvent/Header/Sender/MessageInfo |
| 消息内容解析 | `event.go` | `ParseMessageContent` (text/image/post) |
| Agent 请求/响应模型 | `agent.go` | OpenResponses/ChatCompletions ContentPart + Response |
| MEDIA: 路径提取 | `agent.go` | `extractMediaPath` (共享 helper) |

## CONVENTIONS (THIS PACKAGE)
- `MessageContentPart.ImageData` 已重命名（原 `ImageURL`），强调承载的是 data URL 而非 HTTP URL；图片消息时 `Key` 存 Feishu image_key，`ImageData` 由 callback 在下载后填入
- 图片消息: Type="input_image", Key 存 image_key，下载后填入 data URL
- 文本消息: Type="input_text", Text 存内容
- Agent 响应包含 `MEDIA:/path/to/file` 时触发文件上传
- OpenResponses 使用 `image_url` 字段直接放 data URL（string 形式），ChatCompletions 使用嵌套 `image_url: { url: "..." }` 结构
- OpenResponses 响应解析: 只保留 type=message, role=assistant 的 content；function_call/tool_use 跳过
- `GetFilePath` 跨 OpenResponses/ChatCompletions 共用 `extractMediaPath(replyText, separators)` 逻辑，OpenResponses 用 `" \t\n"` 分隔符，ChatCompletions 仅用 `"\n"`
- `Usage` 结构体：InputTokens/OutputTokens/TotalTokens
- `MessageEvent.Sender.SenderID` 包含 UnionID/UserID/OpenID（仅 UserID 当前使用）
- `URLVerificationRequest.Type == "url_verification"` 用于网关 URL 验证
