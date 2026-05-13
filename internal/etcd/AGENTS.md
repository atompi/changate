# internal/etcd

## OVERVIEW
ETCD客户端封装。

## STRUCTURE
```
etcd/
├── client.go        # ETCD客户端
└── client_test.go
```

## KEY SYMBOLS

| Symbol | Type | Role |
|--------|------|------|
| `Client` | struct | ETCD客户端 |
| `NewClient` | func | 构造函数 |

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| ETCD连接 | `client.go` | NewClient函数 |

## NOTES
- 用于从ETCD读取app/user配置
- 由config.EtcdConfigLoader使用