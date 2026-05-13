# pkg

## OVERVIEW
工具库包，提供加解密、日志、重试功能。

## STRUCTURE
```
pkg/
├── crypto/     # AES加解密
├── logger/     # 结构化日志
└── retry/      # 重试逻辑
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| AES加解密 | `pkg/crypto/aes.go` | DecryptAES256CBC函数 |
| 日志 | `pkg/logger/logger.go` | Info/Debug/Error/Warn |
| 重试 | `pkg/retry/retry.go` | DoWithRetry函数 |

## CONVENTIONS (THIS PROJECT)
- 使用自定义logger而非标准log/slog
- crypto包处理飞书回调加密/解密
- retry包提供指数退避重试