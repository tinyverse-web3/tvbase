package custom

import (
	"context"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/log"
)

type CustomProtocol struct {
	Ctx context.Context
	PID string
}

func (p *CustomProtocol) GetProtocolID() string {
	return p.PID
}

func (p *CustomProtocol) SetCtx(ctx context.Context) {
	p.Ctx = ctx
}

func (p *CustomProtocol) Init(protocolID string) {
	p.PID = protocolID
}

type CustomStreamProtocol struct {
	CustomProtocol
}

type CustomStreamClientProtocol struct {
	CustomStreamProtocol
	Service Service
}

type CustomStreamServiceProtocol struct {
	CustomStreamProtocol
}

// client
func (p *CustomStreamClientProtocol) Init(customProtocolID string) {
	p.CustomStreamProtocol.Init(customProtocolID)
}

func (p *CustomStreamClientProtocol) SetService(service Service) {
	p.Service = service
}

func (p *CustomStreamClientProtocol) Request(peerId string, content []byte) (*pb.CustomProtocolReq, chan any, error) {
	request, responseChan, err := p.Service.Request(peerId, p.PID, content)
	if err != nil {
		log.Logger.Errorf("CustomStreamProtocol->Request: %v", err)
		return nil, nil, err
	}

	return request, responseChan, nil
}

// service
func (p *CustomStreamServiceProtocol) Init(customProtocolID string) {
	p.CustomStreamProtocol.Init(customProtocolID)
}
