package svc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackz-jones/cross-chain-service/internal/config"

	"github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/common/grpc"
	"github.com/zeromicro/go-zero/core/logx"
)

// chainInteractiveKey chain-interactive-service 在 ExternalGrpcConfs 中的键名
const chainInteractiveKey = "chain-interactive-service"

type ServiceContext struct {
	Config config.Config

	// SharedRedisClient 全局共享的事件 Redis 客户端
	// 由 event.Manager / reliableExecutor / logic 层复用，避免每次消息都新建连接
	SharedRedisClient *commonEvent.RedisClient

	// chainInteractiveClient 链交互服务客户端（并发安全）
	// 由后台协程异步创建、可能在运行期重建；通过 GetChainInteractiveClient/setChainInteractiveClient 访问
	chainInteractiveMu     sync.RWMutex
	chainInteractiveClient chaininteractive.ChainInteractive

	// Router 消息路由引擎
	// 注意：为避免循环导入，这里使用 interface{} 类型，实际类型为 *message.MessageRouter
	Router interface{}
}

// NewServiceContext 构造 ServiceContext。
// - 共享 Redis 客户端初始化失败时返回 error，由调用方决定是否退出（避免 panic 使整个服务崩溃）。
// - chain-interactive-service 配置缺失时返回 error（原实现会直接 nil 解引用）。
// - chain-interactive-service 的 gRPC 客户端仍异步建立，避免阻塞启动。
func NewServiceContext(ctx context.Context, c config.Config) (*ServiceContext, error) {
	svc := &ServiceContext{
		Config: c,
	}

	// 校验 chain-interactive-service 配置
	chainConf, ok := c.ExternalGrpcConfs[chainInteractiveKey]
	if !ok || chainConf == nil {
		return nil, fmt.Errorf("missing ExternalGrpcConfs[%q] in config", chainInteractiveKey)
	}

	// 初始化共享 Redis 客户端（失败返回 error，不 panic）
	redisClient, err := commonEvent.NewRedisClient(
		c.SubscribeConf.ConfType, c.SubscribeConf.RedisAddr,
		c.SubscribeConf.RedisUserName, c.SubscribeConf.RedisPassword,
		c.SubscribeConf.MasterName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init shared redis client: %w", err)
	}
	svc.SharedRedisClient = redisClient

	// 异步初始化 chain-interactive-service 客户端
	svc.newChainClient(ctx, chainConf)

	return svc, nil
}

// GetChainInteractiveClient 并发安全地获取 chain-interactive-service 客户端
// 若客户端尚未建立则返回 nil，调用方需自行判空。
func (svc *ServiceContext) GetChainInteractiveClient() chaininteractive.ChainInteractive {
	svc.chainInteractiveMu.RLock()
	defer svc.chainInteractiveMu.RUnlock()
	return svc.chainInteractiveClient
}

// SetChainInteractiveClient 并发安全地设置 chain-interactive-service 客户端
// 主要供测试或未来的健康检查/重连逻辑使用。
func (svc *ServiceContext) SetChainInteractiveClient(client chaininteractive.ChainInteractive) {
	svc.chainInteractiveMu.Lock()
	defer svc.chainInteractiveMu.Unlock()
	svc.chainInteractiveClient = client
}

// newChainClient 初始化 chain-interactive-service 客户端
// ctx 用于在服务退出时中断重试循环
func (svc *ServiceContext) newChainClient(ctx context.Context, c *config.ExternalGrpcConf) {
	if c == nil {
		logx.Errorf("[svc] chain-interactive-service config is nil, skip client init")
		return
	}
	go func() {
		for {
			// 检查 context 是否已取消
			select {
			case <-ctx.Done():
				logx.Info("[svc] context cancelled, stopping chain client initialization")
				return
			default:
			}

			// 创建 grpc 客户端
			grpcClient, err := grpc.CreateGRPCClient(c.CaCertFile, c.ClientCertFile, c.ClientKeyFile, c.DNS, c.Endpoint)
			if err != nil {
				logx.Errorf("failed to create grpc client for chain-interactive-service,err: %v", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
				continue
			}

			// 创建 chain-interactive-service 客户端（并发安全地写入）
			svc.SetChainInteractiveClient(chaininteractive.NewChainInteractive(*grpcClient))
			logx.Info("success to create chain-interactive-service client")
			return
		}
	}()
}

// ErrChainInteractiveNotReady chain-interactive-service 客户端尚未就绪
var ErrChainInteractiveNotReady = errors.New("chain-interactive-service client is not ready")