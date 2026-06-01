# 跨链消息处理示例

本目录包含基于通用跨链消息框架实现具体业务的示例代码，展示如何通过实现 `GenericEventHandler` 接口来接入跨链服务。

## 目录结构

```
examples/
├── README.md                  # 本文件
├── nft/
│   └── main.go               # NFT 跨链铸造/转移示例
└── notification/
    └── main.go               # 通知类跨链消息示例
```

## 核心概念

### GenericEventHandler 接口

每个业务插件需要实现 `message.GenericEventHandler` 接口：

```go
type GenericEventHandler interface {
    // Name 返回处理器名称（用于注册和路由匹配，通常为事件名称）
    Name() string

    // RequiredEventDataLength 返回 Chainmaker 事件数据所需的最小长度
    RequiredEventDataLength() int

    // BuildMessage 从解析后的事件构建跨链消息
    BuildMessage(parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent) (*CrossChainMessage, error)
}
```

### 注册处理器

在应用启动时，将实现好的处理器注册到全局注册表：

```go
registry := message.GlobalHandlerRegistry()
registry.Register(&MyEventHandler{})
```

### 配置跨链服务

确保 `crosschain.yaml` 中配置了 `GenericConf` 相关选项（事件插件、路由规则等）。

## 运行示例

示例代码仅作为参考，不参与主项目编译。如需运行，请确保已安装相关业务合约依赖包。

```bash
# NFT 示例（需要 nft-contract-go 依赖）
cd examples/nft && go run main.go

# Notification 示例（需要 notification-contract-go 依赖）
cd examples/notification && go run main.go
```
