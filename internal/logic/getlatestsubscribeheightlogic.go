package logic

import (
	"context"
	"strings"

	"github.com/jackz-jones/cross-chain-service/internal/code"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	pb "github.com/jackz-jones/cross-chain-service/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// GetLatestSubscribeHeightLogic logic
type GetLatestSubscribeHeightLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// NewGetLatestSubscribeHeightLogic new logic
func NewGetLatestSubscribeHeightLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLatestSubscribeHeightLogic {
	return &GetLatestSubscribeHeightLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetLatestSubscribeHeight 获取最新订阅高度
func (l *GetLatestSubscribeHeightLogic) GetLatestSubscribeHeight(
	in *pb.GetLatestSubscribeHeightRequest) (*pb.GetLatestSubscribeHeightResponse, error) {

	// 日志通用信息
	logger := l.Logger.WithFields([]logx.LogField{
		{Key: "requestId", Value: in.RequestId},
		{Key: "chainName", Value: in.ChainName},
		{Key: "contractName", Value: in.ContractName},
		{Key: "chainType", Value: in.ChainType},
		{Key: "contractType", Value: in.ContractType},
	}...)
	logger.Info("receive GetLatestSubscribeHeight request")

	// 复用 ServiceContext 中的共享 Redis 客户端，避免每次请求都建立新连接
	redisClient := l.svcCtx.SharedRedisClient
	if redisClient == nil {
		logger.Error(code.ErrNewRedisClient.String() + ": shared redis client is nil")
		return l.errorResponse(code.ErrNewRedisClient, nil), nil
	}

	// 获取最新区块高度
	height, err := redisClient.GetLatestBlockHeight(l.ctx, strings.Join([]string{in.ChainType, in.ChainName,
		in.ContractType, in.ContractName}, "#"))
	if err != nil {
		logger.WithFields(logx.LogField{Key: "error", Value: err}).Error(code.ErrGetLatestBlockHeight.String())
		return l.errorResponse(code.ErrGetLatestBlockHeight, err), nil
	}

	// 返回成功
	return l.successResponse(&pb.SubscribeHeight{
		ChainType:    in.ChainType,
		ChainName:    in.ChainName,
		ContractType: in.ContractType,
		ContractName: in.ContractName,
		Height:       height,
	}), nil
}

// errorResponse returns the error response.
func (l *GetLatestSubscribeHeightLogic) errorResponse(code code.RespCode,
	err error) *pb.GetLatestSubscribeHeightResponse {
	msg := code.String()
	if err != nil {
		msg = err.Error()
	}

	return &pb.GetLatestSubscribeHeightResponse{
		Code: int32(code),
		Msg:  msg,
	}
}

// successResponse returns the success response.
func (l *GetLatestSubscribeHeightLogic) successResponse(data *pb.SubscribeHeight) *pb.GetLatestSubscribeHeightResponse {
	return &pb.GetLatestSubscribeHeightResponse{
		Code: int32(code.Success),
		Msg:  code.Success.String(),
		Data: data,
	}
}
