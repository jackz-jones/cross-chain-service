package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jackz-jones/cross-chain-service/internal"
	"github.com/jackz-jones/cross-chain-service/internal/config"
	"github.com/jackz-jones/cross-chain-service/internal/event"
	"github.com/jackz-jones/cross-chain-service/internal/server"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	pb "github.com/jackz-jones/cross-chain-service/pb"

	"github.com/zeromicro/go-zero/core/conf"
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
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterCrossChainServer(grpcServer, server.NewCrossChainServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	// 异步启动事件处理器
	go event.NewEventManager(ctx).Process()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
