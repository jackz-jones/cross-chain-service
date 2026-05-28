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

	// GenericConf 通用框架配置（可选，启用后使用插件化事件处理器和三级路由）
	GenericConf GenericConf `json:",optional"` //nolint:staticcheck
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

// RouteRule 路由规则（链级别，兼容旧配置）
type RouteRule struct {
	// SourceChain 源链名称
	SourceChain string

	// TargetChains 目标链名称列表
	TargetChains []string
}

// EventPluginConfig 事件插件配置
// 用于声明式注册事件处理器，无需修改核心代码
type EventPluginConfig struct {
	// EventName 事件名称（用于路由匹配，对应合约事件名）
	EventName string

	// HandlerType 处理器类型名称（对应内置处理器注册名）
	// 例如："cross_chain_mint"、"enterprise_notified"
	HandlerType string

	// SourceContractType 源合约类型（用于事件过滤）
	SourceContractType string `json:",optional"` //nolint:staticcheck

	// SourceEventName 源事件名称（仅事件级别需要）
	SourceEventName string `json:",optional"` //nolint:staticcheck

	// TargetContractType 目标合约类型（用于查找目标合约名称）
	TargetContractType string `json:",optional"` //nolint:staticcheck

	// TargetMethod 目标方法名（仅事件级别需要）
	TargetMethod string `json:",optional"` //nolint:staticcheck

	// Enabled 是否启用
	Enabled bool `json:",default=true"` //nolint:staticcheck
}

// RouteLevelType 路由级别类型
type RouteLevelType string

const (
	// RouteLevelChain 链级别路由
	RouteLevelChain RouteLevelType = "chain"
	// RouteLevelContract 合约级别路由
	RouteLevelContract RouteLevelType = "contract"
	// RouteLevelEvent 事件级别路由
	RouteLevelEvent RouteLevelType = "event"
)

// DetailedRouteRule 详细路由规则
// 支持链级别、合约级别、事件级别三级路由
type DetailedRouteRule struct {
	// Level 路由级别：chain / contract / event
	Level RouteLevelType `json:",default=chain"` //nolint:staticcheck

	// SourceChain 源链名称（所有级别都需要）
	SourceChain string

	// SourceContractType 源合约类型（合约级别和事件级别需要）
	SourceContractType string `json:",optional"` //nolint:staticcheck

	// SourceEventName 源事件名称（仅事件级别需要）
	SourceEventName string `json:",optional"` //nolint:staticcheck

	// TargetChain 目标链名称
	TargetChain string

	// TargetContractType 目标合约类型（合约级别和事件级别需要）
	TargetContractType string `json:",optional"` //nolint:staticcheck

	// TargetMethod 目标方法名（仅事件级别需要）
	TargetMethod string `json:",optional"` //nolint:staticcheck
}

// GenericConf 通用框架配置
type GenericConf struct {
	// EnableGenericMode 是否启用通用模式（false 则使用旧的硬编码处理器）
	EnableGenericMode bool `json:",default=false"` //nolint:staticcheck

	// EventPlugins 事件插件配置列表
	EventPlugins []EventPluginConfig `json:",optional"` //nolint:staticcheck

	// DetailedRoutes 详细路由规则（支持三级路由，优先级高于旧的 RouteConf）
	DetailedRoutes []DetailedRouteRule `json:",optional"` //nolint:staticcheck

	// TimeoutCheckInterval 超时检查间隔（秒），默认 30
	TimeoutCheckInterval int64 `json:",default=30"` //nolint:staticcheck

	// UnroutedMessagePolicy 路由未找到时的策略：discard（丢弃）/retry（重试）
	UnroutedMessagePolicy string `json:",default=discard"` //nolint:staticcheck
}
