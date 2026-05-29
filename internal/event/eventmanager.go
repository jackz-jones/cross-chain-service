// Package event 合约事件处理
package event

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/code"
	"github.com/jackz-jones/cross-chain-service/internal/message"
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

// RouteTable 路由表，定义链之间的跨链目标关系
// key: 源链名称, value: 目标链配置列表
type RouteTable map[string][]*chainCli.ChainAndContractName

// Manager 事件管理器
// chainConfig 链配置
// redisClient 连接事件redis订阅
type Manager struct {
	svcCtx      *svc.ServiceContext
	eventCtx    context.Context
	chainConfig []*chainCli.ChainAndContractName
	routeTable  RouteTable
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
		routeTable:  make(RouteTable),
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

	// 构建路由表
	e.buildRouteTable()

	e.Logger.Infof("start event: groupName[%s], chains: %d", e.groupName, len(e.chainConfig))

	// 启动事件监听
	go e.processEvent()
}

// buildRouteTable 构建路由表
// 默认策略：每条链的跨链目标是除自身外的所有其他链
// 支持未来通过配置文件自定义路由规则
func (e *Manager) buildRouteTable() {
	// 检查是否有自定义路由配置
	if len(e.svcCtx.Config.RouteConf) > 0 {
		e.buildCustomRouteTable()
		return
	}

	// 默认路由：每条链 → 其他所有链
	for _, source := range e.chainConfig {
		var targets []*chainCli.ChainAndContractName
		for _, target := range e.chainConfig {
			if target.ChainName != source.ChainName {
				targets = append(targets, target)
			}
		}
		e.routeTable[source.ChainName] = targets
	}
}

// buildCustomRouteTable 根据配置构建自定义路由表
func (e *Manager) buildCustomRouteTable() {
	chainMap := make(map[string]*chainCli.ChainAndContractName)
	for _, cc := range e.chainConfig {
		chainMap[cc.ChainName] = cc
	}

	for _, route := range e.svcCtx.Config.RouteConf {
		var targets []*chainCli.ChainAndContractName
		for _, targetName := range route.TargetChains {
			if target, ok := chainMap[targetName]; ok {
				targets = append(targets, target)
			} else {
				e.Logger.Errorf("[event] route config: target chain '%s' not found in available chains", targetName)
			}
		}
		e.routeTable[route.SourceChain] = targets
	}
}

// getTargetChains 获取指定源链的跨链目标
func (e *Manager) getTargetChains(sourceChainName string) []*chainCli.ChainAndContractName {
	targets, ok := e.routeTable[sourceChainName]
	if !ok {
		return nil
	}
	return targets
}

// processEvent 启动事件监听
func (e *Manager) processEvent() {

	// 验证路由表有效性
	if len(e.chainConfig) < 2 {
		e.Logger.Errorf("[event] warning: less than 2 chains configured (%d), cross-chain functionality may be limited",
			len(e.chainConfig))
	}

	// 遍历链配置，对每一个链启动独立的事件监听
	for _, chainConfig := range e.chainConfig {
		go e.listenChainEvent(e.eventCtx, chainConfig)
	}
}

// listenChainEvent 监听每一个链自己的事件
func (e *Manager) listenChainEvent(ctx context.Context, chainConfig *chainCli.ChainAndContractName) {
	e.Logger.Infof("[event] start listen chain[%s] event", chainConfig.ChainName)

	// 通过路由表获取跨链目标
	targetChains := e.getTargetChains(chainConfig.ChainName)
	if len(targetChains) == 0 {
		e.Logger.Errorf("[event] no cross-chain target found for chain '%s', skipping event handlers",
			chainConfig.ChainName)
		return
	}

	// 使用第一个目标链作为主要跨链目标（兼容现有双链逻辑）
	// 未来可扩展为多目标广播
	primaryTarget := targetChains[0]
	if len(targetChains) > 1 {
		e.Logger.Infof("[event] chain '%s' has %d targets, using '%s' as primary target",
			chainConfig.ChainName, len(targetChains), primaryTarget.ChainName)
	}

	// 事件处理器集合
	// 通用框架模式：从配置读取插件处理器
	eventHandlers := e.createGenericHandlers(chainConfig, primaryTarget)

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

// GetRouteTable 获取路由表（用于测试和调试）
func (e *Manager) GetRouteTable() RouteTable {
	return e.routeTable
}

// GetChainConfig 获取链配置（用于测试和调试）
func (e *Manager) GetChainConfig() []*chainCli.ChainAndContractName {
	return e.chainConfig
}

// String 路由表字符串表示
func (rt RouteTable) String() string {
	var sb strings.Builder
	for source, targets := range rt {
		targetNames := make([]string, 0, len(targets))
		for _, t := range targets {
			targetNames = append(targetNames, t.ChainName)
		}
		sb.WriteString(fmt.Sprintf("%s → [%s]\n", source, strings.Join(targetNames, ", ")))
	}
	return sb.String()
}

// createGenericHandlers 创建通用框架模式的事件处理器
func (e *Manager) createGenericHandlers(
	chainConfig *chainCli.ChainAndContractName,
	primaryTarget *chainCli.ChainAndContractName,
) []handler {
	var handlers []handler

	// 从全局注册表获取已注册的通用处理器
	registry := message.GlobalHandlerRegistry()
	allHandlers := registry.GetAll()

	for name, genericHandler := range allHandlers {
		adapter := &genericHandlerAdapter{
			svcCtx:               e.svcCtx,
			logger:               e.Logger,
			processor:            genericHandler,
			registry:             adapter.GlobalRegistry(),
			contractConfs:        chainConfig.ContractDescs,
			crossTargetChainConf: primaryTarget,
		}
		// 使用通用处理器的事件名作为 handler 的事件名
		handlers = append(handlers, adapter)
		e.Logger.Infof("[event] registered generic handler: %s", name)
	}

	if len(handlers) == 0 {
		e.Logger.Infof("[event] no generic handlers registered")
	}

	return handlers
}
