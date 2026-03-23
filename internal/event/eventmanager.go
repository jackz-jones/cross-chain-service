// Package event 合约事件处理
package event

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackz-jones/cross-chain-service/internal/code"
	"github.com/jackz-jones/cross-chain-service/internal/svc"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	"github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

var (
	// defaultGroupName 订阅组信息
	defaultGroupName = "cross-chain-service" // 电子提单平台

	// consumer 消费者,通过配置不相同的consumer，支持负载
	defaultConsumer = "cross-chain-service-consumer"

	// retryChainListInterval 查询链列表的时间间隔
	retryChainListInterval = time.Duration(3) * time.Second

	// retrySubscribeToStreamInterval 订阅链事件失败时，重新订阅的时间间隔
	retrySubscribeToStreamInterval = time.Duration(10) * time.Second
)

// Manager 事件管理器
// chainConfig 链配置
// redisClient 连接事件redis订阅
type Manager struct {
	svcCtx      *svc.ServiceContext
	eventCtx    context.Context
	chainConfig []*chainCli.ChainAndContractName
	redisClient *event.RedisClient
	groupName   string
	logx.Logger
}

// NewEventManager 实例化事件管理器
func NewEventManager(svcCtx *svc.ServiceContext) *Manager {

	// 初始化时间redis客户端
	client, err := event.NewRedisClient(svcCtx.Config.SubscribeConf.ConfType, svcCtx.Config.SubscribeConf.RedisAddr,
		svcCtx.Config.SubscribeConf.RedisUserName, svcCtx.Config.SubscribeConf.RedisPassword,
		svcCtx.Config.SubscribeConf.MasterName)
	if err != nil {
		panic(err)
	}

	return &Manager{
		eventCtx:    context.Background(),
		svcCtx:      svcCtx,
		redisClient: client,
		groupName:   defaultGroupName,
		Logger:      logx.WithContext(context.Background()),
	}
}

// Process 事件处理器
// 从事件redis订阅中，subscribe合约事件
func (e *Manager) Process() {
	e.Logger.Infof("[event] start process event")
	// 阻塞加载链服务配置信息
	// 如果加载不出来，则3秒后重新尝试
	// 直到正确获取链配置
	func() {
		for {
			if e.svcCtx.ChainInteractiveServiceClient == nil {
				e.Logger.Error("[event] chain interactive client is nil")
				time.Sleep(retryChainListInterval)
				continue
			}

			// 获取链配置
			req := &chainCli.GetAvailableChainAndContractNamesRequest{
				RequestId: "cross-chain-service-query-chain-config",
			}
			resp, err := e.svcCtx.ChainInteractiveServiceClient.GetAvailableChainAndContractNames(context.Background(), req)
			if err != nil {
				e.Logger.Errorf("failed to send GetAvailableChainAndContractNames req: %v", err)
				time.Sleep(retryChainListInterval)
				continue
			}

			// 检查 grpc 返回错误码
			if resp.Code != int32(code.Success) {
				e.Logger.Errorf("failed to execute GetAvailableChainAndContractNames: [%d]%s", resp.Code, resp.Msg)
				time.Sleep(retryChainListInterval)
				continue
			}

			e.Logger.Infof("success to GetAvailableChainAndContractNames: %v", resp.Data)

			// 检查是否有可用的链配置
			if len(resp.Data) == 0 {
				e.Logger.Infof("empty chain config by GetAvailableChainAndContractNames")
				time.Sleep(retryChainListInterval)
				continue
			}

			e.chainConfig = resp.Data
			break
		}
	}()

	// 从配置文件加载合约订阅组名称
	if e.svcCtx.Config.SubscribeConf.GroupName != "" {
		e.groupName = e.svcCtx.Config.SubscribeConf.GroupName
	}

	e.Logger.Infof("start event: groupName[%s]", e.groupName)

	// 启动事件监听
	go e.processEvent()
}

// processEvent 启动事件监听
func (e *Manager) processEvent() {

	// 目前只支持双链相互之间的跨链
	if len(e.chainConfig) != 2 {
		panic(fmt.Sprintf("only supported to cross between 2 chain config,but got %d", len(e.chainConfig)))
	}

	// 遍历链配置，对每一个链启动独立的事件监听
	for _, chainConfig := range e.chainConfig {
		go e.listenChainEvent(e.eventCtx, chainConfig)
	}
}

// listenChainEvent 监听每一个链自己的事件
func (e *Manager) listenChainEvent(ctx context.Context, chainConfig *chainCli.ChainAndContractName) {
	e.Logger.Infof("[event] start listen chain[%s] event", chainConfig.ChainName)

	// 解析另一端链配置作为跨链目标
	var crossTargetChainConf *chainCli.ChainAndContractName
	for _, cc := range e.chainConfig {
		if cc.ChainName != chainConfig.ChainName {
			crossTargetChainConf = cc
			break
		}
	}

	// 事件处理器集合
	eventHandlers := []handler{

		// 企业身份创建事件处理器
		NewEnterpriseNotifiedEventHandler(e.Logger, e.svcCtx, chainConfig.ContractDescs, crossTargetChainConf),

		// 文件事件处理器
		NewFileNotifiedEventHandler(e.Logger, e.svcCtx, chainConfig.ContractDescs, crossTargetChainConf),

		// 跨链转移事件处理器
		NewCrossChainTransferEventHandler(e.Logger, e.svcCtx, chainConfig.ContractDescs, crossTargetChainConf),

		// 跨链铸造事件处理器
		NewCrossChainMintEventHandler(e.Logger, e.svcCtx, chainConfig.ContractDescs, crossTargetChainConf),
	}

	dispatcher := newHandlerDispatcher(eventHandlers, e.Logger)
	contracts := chainConfig.GetContractDescs()
	for _, contract := range contracts {
		contractDesc := contract
		// 异步协程订阅事件
		go func() {
			e.Logger.Infof("[event] subscribe redis, ChainName:%s, ChainType:%s, ContractName:%s, ContractType:%s",
				chainConfig.ChainName, chainConfig.ChainType, contractDesc.ContractName, contractDesc.ContractType)
			err1 := retry.Retry(func(attempt uint) error {
				err := e.redisClient.SubscribeTradeGuardEventFromStream(ctx, strings.ToLower(chainConfig.ChainType.String()),
					chainConfig.ChainName, strings.ToLower(contractDesc.ContractType.String()), contractDesc.ContractName,
					e.groupName, defaultConsumer, dispatcher.dispatchTopicHandler, false, 100, 0)
				logx.WithContext(ctx).Errorf("subscribe to steam error: %s", err)
				// 不是主动取消，则订阅重试
				if !strings.Contains(err.Error(), context.Canceled.Error()) {
					e.Logger.Infof("[event] subscribe redis, ChainName:%s, ChainType:%s, ContractName:%s, ContractType:%s, "+
						"retry times:%d", chainConfig.ChainName, chainConfig.ChainType, contractDesc.ContractName,
						contractDesc.ContractType, attempt)
					return err
				}
				return nil
			}, strategy.Wait(retrySubscribeToStreamInterval))
			if err1 != nil {
				e.Logger.Errorf("[event] subscribe redis, ChainName:%s, ChainType:%s, ContractName:%s, ContractType:%s, err:%s",
					chainConfig.ChainName, chainConfig.ChainType, contractDesc.ContractName, contractDesc.ContractType, err1)
			}
		}()
	}
}
