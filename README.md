# WeChat SDK for OpenClaw (Go)

这是微信官方 OpenClaw 插件 (`@tencent-weixin/openclaw-weixin`) 的 Go 语言完整实现，提供与官方 npm 包完全一致的功能。

## 功能特性

- ✅ **二维码扫码登录** - 完整的 QR 码登录流程
- ✅ **消息收发** - 文本、图片、视频、文件、语音
- ✅ **长轮询监控** - 实时接收消息
- ✅ **上下文管理** - Context Token 持久化
- ✅ **CDN 加密传输** - AES-128-ECB 加密上传下载
- ✅ **账号管理** - 多账号支持
- ✅ **Pairing 授权** - AllowFrom 白名单

## 安装

```bash
go get github.com/openclaw/weixin-sdk-go
```

## 快速开始

### 1. 扫码登录

```go
package main

import (
    "context"
    "fmt"
    weixinsdk "github.com/openclaw/weixin-sdk-go"
)

func main() {
    sdk := weixinsdk.New(nil)
    ctx := context.Background()

    // 开始登录
    result, start, err := sdk.Login(ctx, "", false)
    if err != nil {
        panic(err)
    }

    // 打印二维码链接
    if start != nil && start.QRCodeURL != "" {
        fmt.Println("请扫描二维码:", start.QRCodeURL)
    }

    if result.Connected {
        fmt.Printf("登录成功！账号ID: %s\n", result.AccountID)
    }
}
```

### 2. 接收消息

```go
package main

import (
    "context"
    "fmt"
    weixinsdk "github.com/openclaw/weixin-sdk-go"
    "github.com/openclaw/weixin-sdk-go/pkg/api"
)

func main() {
    sdk := weixinsdk.New(nil)
    accountID := "your-account-id"

    // 创建消息处理器
    handler := func(ctx context.Context, msg *api.WeixinMessage, accID string) error {
        fmt.Printf("收到消息: from=%s\n", msg.FromUserID)
        
        // 提取文本
        for _, item := range msg.ItemList {
            if item.Type == int(api.MessageItemTypeText) {
                fmt.Printf("内容: %s\n", item.TextItem.Text)
            }
        }
        
        // 保存 context token
        if msg.ContextToken != "" {
            sdk.SetContextToken(accID, msg.FromUserID, msg.ContextToken)
        }
        
        return nil
    }

    // 启动监控
    monitor, _ := sdk.NewMonitor(accountID, weixinsdk.WithHandler(handler))
    monitor.Start(context.Background())
}
```

### 3. 发送消息

```go
package main

import (
    "context"
    "fmt"
    weixinsdk "github.com/openclaw/weixin-sdk-go"
)

func main() {
    sdk := weixinsdk.New(nil)
    accountID := "your-account-id"
    toUser := "target-user-id"
    contextToken := sdk.GetContextToken(accountID, toUser)

    ctx := context.Background()

    // 发送文本
    msgID, err := sdk.SendText(ctx, accountID, toUser, "你好！", contextToken)
    if err != nil {
        panic(err)
    }
    fmt.Printf("消息已发送: %s\n", msgID)

    // 发送图片
    imageData := []byte{/* 图片二进制数据 */}
    msgID, err = sdk.SendImage(ctx, accountID, toUser, "", imageData, contextToken)
    if err != nil {
        panic(err)
    }
}
```

## CLI 工具

```bash
# 构建
go build -o weixin-sdk ./cmd/weixin-cli

# 扫码登录
./weixin-sdk login

# 查看账号列表
./weixin-sdk list

# 发送消息
./weixin-sdk send --account <id> --to <user-id> --text "Hello"

# 监控消息
./weixin-sdk monitor --account <id>

# 删除账号
./weixin-sdk delete --account <id>
```

## API 文档

### SDK

```go
// 创建 SDK 实例
sdk := weixinsdk.New(config *Config)

// 登录
result, start, err := sdk.Login(ctx, accountID string, force bool)

// 消息监控
monitor, err := sdk.NewMonitor(accountID string, opts ...MonitorOption)

// 发送消息
msgID, err := sdk.SendText(ctx, accountID, to, text, contextToken string)
msgID, err := sdk.SendImage(ctx, accountID, to, caption string, imageData []byte, contextToken string)
msgID, err := sdk.SendFile(ctx, accountID, to, caption, fileName string, fileData []byte, contextToken string)

// 上下文管理
token := sdk.GetContextToken(accountID, userID string)
sdk.SetContextToken(accountID, userID, token string)

// 账号管理
accounts := sdk.ListAccounts()
account, err := sdk.GetAccount(accountID string)
err := sdk.DeleteAccount(accountID string)

// 媒体下载上传
data, err := sdk.DownloadMedia(ctx, encryptedQueryParam, aesKeyBase64 string)
info, err := sdk.UploadMedia(ctx, accountID, toUserID string, data []byte, mediaType api.UploadMediaType)
```

### Monitor

```go
// 创建监控器
monitor, err := sdk.NewMonitor(accountID,
    weixinsdk.WithHandler(handler),
    weixinsdk.WithStatusCallback(statusCb),
    weixinsdk.WithCDNBaseURL(cdnURL),
)

// 启动监控
err := monitor.Start(ctx)

// 获取状态
status := monitor.GetStatus()

// 停止监控
monitor.Stop()
```

### Sender

```go
sender, err := sdk.NewSender(accountID)

// 发送各种类型消息
msgID, err := sender.SendText(ctx, to, text, contextToken)
msgID, err := sender.SendImage(ctx, to, caption string, imageData []byte, contextToken)
msgID, err := sender.SendVideo(ctx, to, caption string, videoData []byte, contextToken)
msgID, err := sender.SendFile(ctx, to, caption, fileName string, fileData []byte, contextToken)
```

## 与官方 npm 包对应关系

| npm 模块 | Go 包 |
|---------|------|
| `src/api/api.ts` | `pkg/api/api.go` |
| `src/api/types.ts` | `pkg/api/types.go` |
| `src/auth/login-qr.ts` | `pkg/auth/login.go` |
| `src/auth/accounts.ts` | `pkg/auth/accounts.go` |
| `src/cdn/aes-ecb.ts` | `pkg/cdn/aes.go` |
| `src/cdn/upload.ts` | `pkg/cdn/upload.go` |
| `src/cdn/pic-decrypt.ts` | `pkg/cdn/download.go` |
| `src/messaging/send.ts` | `pkg/messaging/send.go` |
| `src/messaging/inbound.ts` | `pkg/messaging/inbound.go` |
| `src/monitor/monitor.ts` | `pkg/monitor/monitor.go` |
| `src/storage/*.ts` | `pkg/storage/state.go` |

## 协议说明

详见官方 README 文档，所有 API 接口与官方 npm 包完全一致：

- `getUpdates` - 长轮询获取新消息
- `sendMessage` - 发送消息
- `getUploadUrl` - 获取 CDN 上传 URL
- `getConfig` - 获取账号配置
- `sendTyping` - 发送输入状态

## License

MIT
