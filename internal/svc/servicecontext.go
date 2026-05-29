package svc

import (
	"context"
	"time"

	"github.com/jackz-jones/cross-chain-service/internal/config"

	"github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	"github.com/jackz-jones/common/grpc"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config config.Config

	// ChainInteractiveServiceClient 链交互服务客户端
	ChainInteractiveServiceClient chaininteractive.ChainInteractive

	// GenericRouter 通用消息路由引擎（通用框架模式启用时使用）
	// 注意：为避免循环导入，这里使用 interface{} 类型，实际类型为 *message.MessageRouter
	GenericRouter interface{}
}

func NewServiceContext(ctx context.Context, c config.Config) *ServiceContext {
	svc := &ServiceContext{
		Config: c,
	}

	// 初始化 chain-interactive-service 客户端
	svc.newChainClient(ctx, c.ExternalGrpcConfs["chain-interactive-service"])

	return svc
}

// newChainClient 初始化 chain-interactive-service 客户端
// ctx 用于在服务退出时中断重试循环
func (svc *ServiceContext) newChainClient(ctx context.Context, c *config.ExternalGrpcConf) {
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
				time.Sleep(time.Second * 3)
				continue
			}

			// 创建 chain-interactive-service 客户端
			svc.ChainInteractiveServiceClient = chaininteractive.NewChainInteractive(*grpcClient)
			logx.Info("success to create chain-interactive-service client")
			return
		}
	}()
}
