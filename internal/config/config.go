package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	GrpcConf      GrpcConf
	SubscribeConf SubscribeConf

	// ExternalGrpcConfs 要访问的外部gRPC配置
	ExternalGrpcConfs map[string]*ExternalGrpcConf

	// SendTxConf 交易发送配置
	SendTxConf SendTxConf

	// ReliabilityConf 可靠性配置（可选）
	ReliabilityConf ReliabilityConf `json:",optional"` //nolint:staticcheck

	// RouteConf 路由配置（可选，不配置则使用默认的全互联路由）
	RouteConf []RouteRule `json:",optional"` //nolint:staticcheck
}

// GrpcConf contain all config items for grpc server initiation
type GrpcConf struct {
	// CaCertFile 是 CA 根证书文件的路径
	CaCertFile string

	// ServerCertFile 是服务端证书文件的路径
	ServerCertFile string

	// ServerKeyFile 是服务端私钥文件的路径
	ServerKeyFile string

	// MaxRecvMsgSize 是最大接收消息大小
	MaxRecvMsgSize int

	// MaxSendMsgSize 是最大发送消息大小
	MaxSendMsgSize int
}

// SubscribeConf contain all config items for subscribing chain event
type SubscribeConf struct {

	// confType 配置类型（cluster或者node）
	ConfType string

	// RedisAddr 是 Redis 服务器地址
	RedisAddr string

	// RedisUserName 是 Redis 用户名
	RedisUserName string

	// RedisPassword 是 Redis 密码
	RedisPassword string

	// nolint:staticcheck
	// 哨兵模式的MasterName，其他模式可忽略
	MasterName string `json:",optional"`

	// GroupName 是 redis 分组名
	GroupName string
}

// ExternalGrpcConf defines the configuration for the external gRPC
type ExternalGrpcConf struct {
	ClientCertFile string
	ClientKeyFile  string
	CaCertFile     string
	DNS            string
	Endpoint       string
}

// SendTxConf 交易发送配置
type SendTxConf struct {

	// WithSyncResult 是否同步返回交易结果
	WithSyncResult bool

	// TxTimeout 发送交易超时时间
	TxTimeout int64
}

// ReliabilityConf 可靠性配置
type ReliabilityConf struct {
	// EnableIdempotency 是否启用幂等性检查
	EnableIdempotency bool `json:",optional"` //nolint:staticcheck

	// IdempotencyTTL 幂等性记录过期时间（秒），默认 86400（24小时）
	IdempotencyTTL int64 `json:",optional"` //nolint:staticcheck

	// EnableRetry 是否启用重试
	EnableRetry bool `json:",optional"` //nolint:staticcheck

	// MaxRetries 最大重试次数，默认 3
	MaxRetries int `json:",optional"` //nolint:staticcheck

	// RetryBaseDelay 重试基础延迟（毫秒），默认 1000
	RetryBaseDelay int64 `json:",optional"` //nolint:staticcheck

	// RetryMaxDelay 重试最大延迟（毫秒），默认 30000
	RetryMaxDelay int64 `json:",optional"` //nolint:staticcheck

	// RetryMultiplier 退避乘数，默认 2.0
	RetryMultiplier float64 `json:",optional"` //nolint:staticcheck
}

// RouteRule 路由规则
type RouteRule struct {
	// SourceChain 源链名称
	SourceChain string

	// TargetChains 目标链名称列表
	TargetChains []string
}
