package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackz-jones/cross-chain-service/internal"
	"github.com/jackz-jones/cross-chain-service/internal/config"
	"github.com/jackz-jones/cross-chain-service/internal/event"
	"github.com/jackz-jones/cross-chain-service/internal/message"
	"github.com/jackz-jones/cross-chain-service/internal/server"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	pb "github.com/jackz-jones/cross-chain-service/pb"

	commonGrpc "github.com/jackz-jones/common/grpc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/crosschain.yaml", "the config file")

func main() {
	flag.Parse()

	// version 选项打印当前版本信息
	args := flag.Args()
	if len(args) > 0 && args[0] == "version" {
		fmt.Println(internal.VersionInfo())
		os.Exit(0)
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 创建带取消的根 context，监听 SIGINT/SIGTERM
	rootCtx, rootCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer rootCancel()

	svcCtx := svc.NewServiceContext(rootCtx, c)

	// 初始化消息路由引擎
	router := message.NewMessageRouter(c.RouteConf.DetailedRoutes,
		logx.WithContext(rootCtx),
		c.RouteConf.UnroutedMessagePolicy)
	svcCtx.Router = router

	// 初始化 grpc 服务注册器
	register := func(grpcServer *grpc.Server) {
		pb.RegisterCrossChainServer(grpcServer, server.NewCrossChainServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	}

	// 创建 grpc 服务
	s, err := commonGrpc.CreateGRPCServer(c.RpcServerConf, register, c.GrpcConf.CaCertFile, c.GrpcConf.ServerCertFile,
		c.GrpcConf.ServerKeyFile, c.GrpcConf.MaxRecvMsgSize, c.GrpcConf.MaxSendMsgSize)
	if err != nil {
		panic(fmt.Errorf("failed to CreateGRPCServer,error: %v", err))
	}

	defer s.Stop()

	// 异步启动事件处理器，传递根 context
	go event.NewEventManager(rootCtx, svcCtx).Process()

	// 监听退出信号
	go func() {
		<-rootCtx.Done()
		logx.Info("[main] received shutdown signal, waiting for graceful shutdown...")
		// 给予子协程 5 秒时间完成清理
		time.Sleep(5 * time.Second)
		logx.Info("[main] graceful shutdown timeout, forcing exit")
		os.Exit(0)
	}()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
