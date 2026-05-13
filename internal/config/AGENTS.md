# internal/config

## OVERVIEW
Viper配置加载 + ETCD配置管理。

## STRUCTURE
```
config/
├── config.go          # Config结构体 + Load函数 + validateConfig
├── etcd_loader.go     # EtcdConfigLoader从ETCD读取app/user配置
├── config_test.go
└── etcd_loader_test.go
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `Config` | struct | 根配置，包含Server/Etcd/LogLevel |
| `AppConfig` | struct | /changate/<app_name> 存储 |
| `UserConfig` | struct | /changate/<app_name>/<user_id> 存储 |
| `EtcdConfigLoader` | struct | 从ETCD加载配置 |
| `Load` | func | 从文件加载配置 |
| `NewEtcdConfigLoader` | func | 创建ETCD加载器 |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| 配置结构定义 | `config.go:11-64` | Config/AppConfig/UserConfig |
| 配置加载 | `config.go:66-89` | Load函数 |
| ETCD路径结构 | `etcd_loader.go` | 理解配置层级 |
| 配置验证 | `config.go:91-121` | validateConfig设置默认值 |

## CONVENTIONS (THIS PACKAGE)
- 使用 `mapstructure:` 标签 (非json:)
- 默认值在 `validateConfig()` 中设置，不在struct tag
- ETCD路径: `/changate/<app_name>` 和 `/changate/<app_name>/<user_id>`
- Viper配置绑定使用 `SetEnvKeyReplacer(strings.NewReplacer(".", "_"))`