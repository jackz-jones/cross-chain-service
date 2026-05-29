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

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
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

	// 如果启用通用框架模式，初始化通用消息路由引擎
	if c.GenericConf.EnableGenericMode {
		router := message.NewMessageRouter(c.GenericConf.DetailedRoutes,
			logx.WithContext(rootCtx),
			c.GenericConf.UnroutedMessagePolicy)
		svcCtx.GenericRouter = router
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterCrossChainServer(grpcServer, server.NewCrossChainServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
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
