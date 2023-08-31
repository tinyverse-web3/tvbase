package customProtocol

import (
	"context"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

type ClientService interface {
	RequestCustomStreamProtocol(customProtocolId string, peerId string, content []byte) (*pb.CustomProtocolReq, chan any, error)
}

type CustomStreamProtocolClient interface {
	GetProtocolID() string
	SetCtx(ctx context.Context)
	SetService(service ClientService)
}

type CustomPubsubProtocolClient interface {
	GetProtocolID() string
	SetCtx(ctx context.Context)
	SetService(service ClientService)
	HandleResponse(request *pb.CustomProtocolReq, res *pb.CustomProtocolRes) error
}

type CustomStreamProtocolService interface {
	GetProtocolID() string
	SetCtx(ctx context.Context)
	HandleRequest(req *pb.CustomProtocolReq) error
	HandleResponse(req *pb.CustomProtocolReq, res *pb.CustomProtocolRes) error
}

type CustomPubsubProtocolService interface {
	GetProtocolID() string
	SetCtx(ctx context.Context)
	HandleRequest(req *pb.CustomProtocolReq) error
	HandleResponse(req *pb.CustomProtocolReq, res *pb.CustomProtocolRes) error
}
