// Package code defines some response code
package code

// RespCode 服务端返回码
type RespCode int

// Success 服务端返回码，200 表示成功，其他表示失败
const (
	Success RespCode = 200000
)

// 600000-699999 表示 cross-chain-service grpc 错误码
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
	ErrMsgNotMyEvent           = "not my event"
	ErrMsgEventDataEmpty       = "event data empty"
	ErrMsgJsonUnmarshal        = "failed to json unmarshal"
	ErrMsgInvalidEventInfoData = "invalid event info data"
	ErrMsgUnknownChainType     = "unknown chain type"
	ErrMsgNewEthEventHandler   = "failed to NewEthEventHandler"
	ErrMsgUnpackIntoInterface  = "failed to unpack eth event data into interface"
	ErrMsgInvalidAbi           = "invalid abi"
	ErrMsgJsonMarshal          = "failed to json marshal"
)
