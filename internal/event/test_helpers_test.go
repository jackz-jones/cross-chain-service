package event

import (
	"encoding/json"

	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/config"
	"github.com/jackz-jones/cross-chain-service/internal/svc"

	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

func init() {
	// 注册适配器到全局注册表，确保测试中可以使用
	adapter.RegisterAdapter(adapter.NewEthereumAdapter())
	adapter.RegisterAdapter(adapter.NewChainmakerAdapter())
}

// 测试常用标识符
const (
	TestEventID = "id-001"
	TestTxHash  = "0xhash"

	// 测试用链类型
	TestChainTypeEthereum   = "ethereum"
	TestChainTypeChainmaker = "chainmaker"
	TestChainTypeSolana     = "solana"
)

// testSetup 测试环境配置
type testSetup struct {
	svcCtx               *svc.ServiceContext
	mockClient           *MockChainInteractive
	contractConfs        []*chainPb.ContractDesc
	crossTargetChainConf *chainCli.ChainAndContractName
	logger               logx.Logger
}

// newTestSetup 创建测试环境
func newTestSetup() *testSetup {
	// 禁用日志输出
	logx.Disable()

	mockClient := NewMockChainInteractive()

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			SendTxConf: config.SendTxConf{
				WithSyncResult: true,
				TxTimeout:      30,
			},
		},
	}
	svcCtx.SetChainInteractiveClient(mockClient)

	// 模拟当前链的合约配置（包含 ABI）
	contractConfs := []*chainPb.ContractDesc{
		{
			ContractName: "notification-contract",
			ContractType: chainPb.ContractType_Notification,
			Abi:          testNotificationABI,
		},
		{
			ContractName: "nft-contract",
			ContractType: chainPb.ContractType_Nft,
			Abi:          testNFTABI,
		},
	}

	// 模拟跨链目标链配置
	crossTargetChainConf := &chainCli.ChainAndContractName{
		ChainName: "target-chain",
		ChainType: chainPb.ChainType_Chainmaker,
		ContractDescs: []*chainPb.ContractDesc{
			{
				ContractName: "target-notification-contract",
				ContractType: chainPb.ContractType_Notification,
			},
			{
				ContractName: "target-nft-contract",
				ContractType: chainPb.ContractType_Nft,
			},
		},
	}

	return &testSetup{
		svcCtx:               svcCtx,
		mockClient:           mockClient,
		contractConfs:        contractConfs,
		crossTargetChainConf: crossTargetChainConf,
		logger:               logx.WithContext(nil),
	}
}

// buildTradeGuardEvent 构造 TradeGuardEvent 测试数据
func buildTradeGuardEvent(eventName, chainType, chainName, contractType, contractName string, eventData []byte) commonEvent.TradeGuardEvent {
	return commonEvent.TradeGuardEvent{
		EventName:    eventName,
		ChainType:    chainType,
		ChainName:    chainName,
		ContractType: contractType,
		ContractName: contractName,
		EventData:    eventData,
	}
}

// buildChainmakerEventData 构造 Chainmaker 事件数据
func buildChainmakerEventData(eventDataItems []string) []byte {
	type ContractEventInfo struct {
		ContractName    string   `json:"contract_name"`
		ContractVersion string   `json:"contract_version"`
		Topic           string   `json:"topic"`
		TxId            string   `json:"tx_id"`
		EventData       []string `json:"event_data"`
	}

	eventInfo := ContractEventInfo{
		ContractName:    "test-contract",
		ContractVersion: "1.0",
		Topic:           "test-topic",
		TxId:            "test-tx-id",
		EventData:       eventDataItems,
	}

	data, _ := json.Marshal(eventInfo)
	return data
}

// buildValidEthLogJSON 构造一个有效的以太坊 Log JSON（包含所有必需字段）
func buildValidEthLogJSON() []byte {
	ethLog := map[string]interface{}{
		"address":          "0x0000000000000000000000000000000000000000",
		"topics":           []string{},
		"data":             "0x",
		"blockNumber":      "0x1",
		"transactionHash":  "0x0000000000000000000000000000000000000000000000000000000000000000",
		"transactionIndex": "0x0",
		"blockHash":        "0x0000000000000000000000000000000000000000000000000000000000000000",
		"logIndex":         "0x0",
		"removed":          false,
	}
	data, _ := json.Marshal(ethLog)
	return data
}

// 测试用的 ABI（简化版，仅用于测试框架验证）
// 注意：实际测试中如果需要真正解析以太坊事件，需要提供完整的 ABI
const testNotificationABI = `[{"anonymous":false,"inputs":[{"indexed":false,"internalType":"bytes","name":"enterpriseInfo","type":"bytes"},{"indexed":false,"internalType":"bool","name":"needCrossChain","type":"bool"}],"name":"EnterpriseNotified","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"bytes","name":"fileInfo","type":"bytes"},{"indexed":false,"internalType":"bool","name":"needCrossChain","type":"bool"}],"name":"FileNotified","type":"event"}]`

const testNFTABI = `[{"anonymous":false,"inputs":[{"indexed":false,"internalType":"bytes","name":"nftInfo","type":"bytes"},{"indexed":false,"internalType":"address","name":"from","type":"address"},{"indexed":false,"internalType":"address","name":"to","type":"address"}],"name":"CrossChainTransfer","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"bytes","name":"nftInfo","type":"bytes"}],"name":"CrossChainMint","type":"event"}]`
