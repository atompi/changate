# PROJECT KNOWLEDGE BASE

**Generated:** 2026-05-15T10:52:05Z
**Commit:** e6bccaa
**Branch:** main

## OVERVIEW
飞书(Feishu)与AI Agent服务的通道网关，接收飞书消息回调并转发给后端Agent(Hermes/OpenClaw)。

## STRUCTURE
```
changate/
├── cmd/server/        # 程序入口
├── internal/          # 核心业务 (agent, config, handler, model, router, feishu, etcd)
├── pkg/               # 工具库 (crypto, logger, retry)
├── config/            # 配置文件
├── docker/            # Docker编排
└── dist/              # 构建产物
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| 回调处理 | `internal/handler/callback.go` | 消息解密/验证/分发 |
| Agent调用 | `internal/agent/` | OpenResponses/ChatCompletions客户端 |
| 配置加载 | `internal/config/` | Viper + ETCD配置管理 |
| 飞书API | `internal/feishu/` | 消息发送/资源下载 |
| 路由入口 | `internal/router/router.go` | Gin路由设置 |
| 模型定义 | `internal/model/` | Event/Agent响应结构 |

## CONVENTIONS (THIS PROJECT)
- 使用 `mapstructure:` 标签而非 `json:` (Viper配置绑定)
- 配置默认值在 `validateConfig()` 函数中设置，非struct tag
- 使用自定义 `pkg/logger` 而非标准 `log/slog`
- 多Agent类型支持: `OpenResponses` (默认) / `ChatCompletions`
- 异步消息处理: 回调立即返回，goroutine处理Agent调用

## ANTI-PATTERNS (THIS PROJECT)
- **不要**在 `internal/` 外创建业务逻辑包
- **不要**使用 `interface{}`，用 `any` 替代
- **不要**提交未格式化的代码 (`gofmt` 检查失败)

## UNIQUE STYLES
- ETCD配置路径: `/changate/<app_name>` + `/changate/<app_name>/<user_id>`
- 消息内容解析: `model.ParseMessageContent()` 统一处理文本/图片
- Agent响应: 支持 `MEDIA:/path/to/file` 格式触发文件上传
- 并发控制: per-app semaphore 限制 MaxConcurrent

## COMMANDS
```bash
go build -o changate ./cmd/server    # 构建
go test ./...                        # 测试
gofmt -w .                           # 格式化
docker build -t atompi/changate .    # Docker构建
```

## NOTES
- `dist/` 是构建产物目录，应在 .gitignore 中
- `config/` 使用单数形式(非标准 `configs/`)
- 5个文件需要gofmt格式化: `internal/agent/chatcompletions.go`, `client_test.go`, `config_test.go`, `handler/agent_cache_test.go`, `pkg/retry/retry_test.go`
- 无GitHub Actions / Makefile / golangci.toml