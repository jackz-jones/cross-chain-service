// Package code defines some response code
package code

// RespCode 服务端返回码
type RespCode int

// Success 服务端返回码，200 表示成功，其他表示失败
const (
	Success RespCode = 200000
)

// 500000-599999 表示 cross-chain-service grpc 错误码
const (
	ErrNewRedisClient RespCode = iota + 600000
	ErrGetLatestBlockHeight
)

// 返回码对应具体的信息
var errMsg = map[RespCode]string{
	Success:                 "success",
	ErrNewRedisClient:       "failed to new redis client",
	ErrGetLatestBlockHeight: "failed to get latest block height",
}

func (rc RespCode) String() string {
	return errMsg[rc]
}

const (
	ErrMsgNotMyEvent                      = "not my event"
	ErrMsgEventDataEmpty                  = "event data empty"
	ErrMsgJsonUnmarshal                   = "failed to json unmarshal"
	ErrMsgInvalidEventInfoData            = "invalid event info data"
	ErrMsgUnknownChainType                = "unknown chain type"
	ErrMsgNewEthEventHandler              = "failed to NewEthEventHandler"
	ErrMsgUnpackIntoInterface             = "failed to unpack eth event data into interface"
	ErrMsgInvalidAbi                      = "invalid abi"
	ErrMsgSendSupervisionAuthorization    = "failed to send supervision authorization req"
	ErrMsgExecuteSupervisionAuthorization = "failed to execute supervision authorization req"
	ErrMsgNotInWhiteList                  = "not in white list"
	ErrMsgCheckWhitelist                  = "failed to check whitelist"
	ErrMsgSendNotifyRequestFile           = "failed to send notify request file req"
	ErrMsgExecuteNotifyRequestFile        = "failed to execute notify request file req"
	ErrMsgNotifyFile                      = "failed to notify file"
	ErrMsgSendCallContract                = "failed to send call contract req"
	ErrMsgExecuteCallContract             = "failed to execute call contract req"
	ErrMsgSendCrossChainTx                = "failed to send cross chain tx"
	ErrMsgJsonMarshal                     = "failed to json marshal"
	ErrMsgCreateNotifyEnterpriseInfoKvs   = "failed to create notify enterprise info kvs"
	ErrMsgHandlerCrossChain               = "failed to handler cross chain"
	ErrMsgCreateNotifyFileInfoKvs         = "failed to create notify file info kvs"
	ErrMsgCreateCrossChainMintKvs         = "failed to create cross chain mint kvs"
	ErrMsgCreateUpdateCrossChainStatusKvs = "failed to create update cross chain status kvs"
	ErrMsgCreateCallbackKvs               = "failed to create callback kvs"
)

const (
	MsgSuccessToSendCrossChainTx = "success to send cross chain tx"
)
