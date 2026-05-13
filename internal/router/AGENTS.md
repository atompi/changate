# internal/router

## OVERVIEW
Gin路由设置。

## STRUCTURE
```
router/
└── router.go
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `Setup` | func | 设置路由，返回RouterResult |
| `RouterResult` | struct | 包含Engine和Handler |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| 路由配置 | `router.go` | Setup函数 |
| 路由注册 | `router.go:28-30` | /health 和 /feishu/:appName |

## CONVENTIONS (THIS PACKAGE)
- /health: 健康检查
- /feishu/:appName: 飞书回调入口
- 路由设置在router.Setup()中完成