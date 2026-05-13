# internal/feishu

## OVERVIEW
飞书API客户端：获取tenant token、发送消息、下载资源。

## STRUCTURE
```
feishu/
├── client.go        # Feishu API客户端
└── client_test.go
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `Client` | struct | 飞书API客户端 |
| `NewClient` | func | 构造函数 |
| `GetTenantAccessToken` | method | 获取tenant access token |
| `GetAppAccessToken` | method | 获取app access token |
| `SendTextMessage` | method | 发送文本消息 |
| `SendFileMessage` | method | 发送文件/图片消息 |
| `DownloadMessageResource` | method | 下载消息中的图片 |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| 飞书客户端 | `client.go` | 所有API方法 |
| token获取 | `client.go` | GetTenantAccessToken/GetAppAccessToken |
| 消息发送 | `client.go` | SendTextMessage/SendFileMessage |
| 图片下载 | `client.go` | DownloadMessageResource |

## CONVENTIONS (THIS PACKAGE)
- 使用tenant_access_token发送消息
- 使用app_access_token上传文件
- 图片以base64 data URL格式发送给Agent
- 文件消息使用multipart/form-data上传