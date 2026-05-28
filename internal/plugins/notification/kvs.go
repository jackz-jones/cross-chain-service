// Package notification Notification 跨链插件
package notification

import (
	"encoding/json"
	"fmt"
	"time"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	notificationConst "github.com/jackz-jones/notification-contract-go/const"
	notificationTypes "github.com/jackz-jones/notification-contract-go/types"
	notificationUtil "github.com/jackz-jones/notification-contract-go/util"
)

// CreateNotifyEnterpriseInfoKvs 构造调用合约方法 NotifyEnterpriseInfo 参数 kvs
func CreateNotifyEnterpriseInfoKvs(id, originHash, address, did string,
	needCrossChain bool) ([]*chainPb.KeyValuePair, error) {

	ei := notificationTypes.EnterpriseInfo{
		ID:         id,
		OriginHash: originHash,
		CreatedAt:  time.Now(),
		Address:    address,
		Did:        did,
	}
	eiBytes, err := json.Marshal(ei)
	if err != nil {
		return nil, fmt.Errorf("marshal EnterpriseInfo: %w", err)
	}

	return []*chainPb.KeyValuePair{
		{
			Key:   notificationConst.ParamEnterpriseInfo,
			Value: eiBytes,
		},
		{
			Key:   notificationConst.ParamNeedCrossChain,
			Value: notificationUtil.BoolToBytes(needCrossChain),
		},
	}, nil
}

// CreateNotifyFileInfoKvs 构造调用合约方法 NotifyFileInfo 参数 kvs
func CreateNotifyFileInfoKvs(id, originHash string, msgType int, needCrossChain bool) ([]*chainPb.KeyValuePair, error) {

	fi := notificationTypes.FileInfo{
		ID:         id,
		OriginHash: originHash,
		CreatedAt:  time.Now(),
		MsgType:    notificationTypes.MessageType(msgType),
	}
	fiBytes, err := json.Marshal(fi)
	if err != nil {
		return nil, fmt.Errorf("marshal FileInfo: %w", err)
	}

	return []*chainPb.KeyValuePair{
		{
			Key:   notificationConst.ParamFileInfo,
			Value: fiBytes,
		},
		{
			Key:   notificationConst.ParamNeedCrossChain,
			Value: notificationUtil.BoolToBytes(needCrossChain),
		},
	}, nil
}

// CreateCallbackKvs 构造调用合约方法 Callback 参数 kvs
func CreateCallbackKvs(id, originHash, msg string, errCode, msgType int) ([]*chainPb.KeyValuePair, error) {

	cc := notificationTypes.CallBackInfo{
		ID:         id,
		OriginHash: originHash,
		Code:       errCode,
		Msg:        msg,
		MsgType:    notificationTypes.MessageType(msgType),
	}
	ccBytes, err := json.Marshal(cc)
	if err != nil {
		return nil, fmt.Errorf("marshal CallBackInfo: %w", err)
	}

	return []*chainPb.KeyValuePair{
		{
			Key:   notificationConst.ParamCallbackInfo,
			Value: ccBytes,
		},
	}, nil
}
