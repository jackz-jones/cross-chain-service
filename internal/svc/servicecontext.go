package svc

import (
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
}

func NewServiceContext(c config.Config) *ServiceContext {
	svc := &ServiceContext{
		Config: c,
	}

	// 初始化 chain-interactive-service 客户端
	svc.newChainClient(c.ExternalGrpcConfs["chain-interactive-service"])

	return svc
}

// newChainClient 初始化 chain-interactive-service 客户端
func (svc *ServiceContext) newChainClient(c *config.ExternalGrpcConf) {
	go func() {
		for {
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
