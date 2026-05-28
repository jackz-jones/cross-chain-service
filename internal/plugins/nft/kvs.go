// Package nft NFT 跨链插件
package nft

import (
	"encoding/json"
	"fmt"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	nftconst "github.com/jackz-jones/nft-contract-go/const"
	nftTypes "github.com/jackz-jones/nft-contract-go/types"
)

// CreateCrossChainMintKvs 构造调用合约方法 CrossChainMint 参数 kvs
// 由 chain-interactive-service 服务本身去翻译成以太坊的 input data
func CreateCrossChainMintKvs(id, owner, holder, originHash, data string) ([]*chainPb.KeyValuePair, error) {
	ni := nftTypes.NFTInfo{
		ID:         id,
		Owner:      owner,
		Holder:     holder,
		OriginHash: originHash,
		Data:       data,
	}
	niBytes, err := json.Marshal(ni)
	if err != nil {
		return nil, fmt.Errorf("marshal NFTInfo: %w", err)
	}

	return []*chainPb.KeyValuePair{
		{
			Key:   nftconst.ParamNFTInfo,
			Value: niBytes,
		},
	}, nil
}

// CreateUpdateCrossChainStatusKvs 构造调用合约方法 UpdateCrossChainStatus 参数 kvs
func CreateUpdateCrossChainStatusKvs(tokenId string, state int) ([]*chainPb.KeyValuePair, error) {
	return []*chainPb.KeyValuePair{
		{
			Key:   nftconst.ParamTokenId,
			Value: []byte(tokenId),
		},
		{
			Key:   nftconst.ParamState,
			Value: []byte(fmt.Sprintf("%d", state)),
		},
	}, nil
}
