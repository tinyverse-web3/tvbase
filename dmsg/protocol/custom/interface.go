package customProtocol

import (
	"context"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

type LightService interface {
	RequestCustomProtocol(customProtocolId string, content []byte) error
}

type CustomProtocolLight interface {
	GetProtocolID() string
	SetCtx(ctx context.Context)
	SetService(service LightService)
	HandleResponse(request *pb.CustomProtocolReq, res *pb.CustomProtocolRes) error
}

type CustomProtocolService interface {
	GetProtocolID() string
	SetCtx(ctx context.Context)
	HandleRequest(req *pb.CustomProtocolReq) error
	HandleResponse(req *pb.CustomProtocolReq, res *pb.CustomProtocolRes) error
}
