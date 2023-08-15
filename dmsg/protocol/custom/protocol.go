package custom

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/log"
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

func (p *CustomProtocol) Marshal(content any) ([]byte, error) {
	data, err := json.Marshal(content)
	if err != nil {
		log.Logger.Errorf("CustomProtocol->Marshal: json marshal error: %v", err)
		return nil, nil
	}
	return data, nil
}

func (p *CustomProtocol) Unmarshal(data []byte, content any) error {
	err := json.Unmarshal(data, &content)
	if err != nil {
		log.Logger.Errorf("CustomProtocol->Unmarshal: json unmarshal error: %v", err)
		return err
	}
	return nil
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

func (p *CustomStreamClientProtocol) HandleResponse(protocolResponse *pb.CustomProtocolRes, responseObject any) error {
	if protocolResponse == nil {
		log.Logger.Errorf("CustomStreamClientProtocol->HandleResponse: response is nil")
		return fmt.Errorf("CustomStreamClientProtocol->HandleResponse: response is nil")
	}
	log.Logger.Debugf("CustomStreamClientProtocol->HandleResponse: response: %v", protocolResponse)
	if protocolResponse.PID != p.PID {
		log.Logger.Errorf("CustomStreamClientProtocol->HandleResponse: response.PID: %v != %v", protocolResponse.PID, p.PID)
		return fmt.Errorf("CustomStreamClientProtocol->HandleResponse: response.PID: %v != %v", protocolResponse.PID, p.PID)
	}
	if protocolResponse.RetCode.Code < 0 {
		log.Logger.Warnf("CustomStreamClientProtocol->HandleResponse: response.RetCode Code < 0: %v", protocolResponse.RetCode)
	}

	err := p.Unmarshal(protocolResponse.Content, responseObject)
	if err != nil {
		log.Logger.Errorf("CustomStreamClientProtocol->HandleResponse: unmarshal error: %v", err)
		return err
	}

	return nil
}

func (p *CustomStreamClientProtocol) Request(peerId string, data any) error {
	if p.Ctx == nil {
		log.Logger.Errorf("CustomStreamClientProtocol->Request: context is nil")
		return fmt.Errorf("CustomStreamClientProtocol->Request: context is nil")
	}

	if p.PID == "" {
		log.Logger.Errorf("CustomStreamClientProtocol->Request: customProtocolID is empty")
		return fmt.Errorf("CustomStreamClientProtocol->Request: customProtocolID is empty")
	}
	if p.Service == nil {
		log.Logger.Errorf("CustomStreamClientProtocol->Request: client service is nil")
		return fmt.Errorf("CustomStreamClientProtocol->Request: client service is nil")
	}

	content, err := p.Marshal(data)
	if err != nil {
		log.Logger.Errorf("CustomStreamProtocol->Request: marshal error: %v", err)
		return fmt.Errorf("CustomStreamProtocol->Request: marshal error: %v", err)
	}

	err = p.Service.Request(peerId, p.PID, content)
	if err != nil {
		log.Logger.Errorf("CustomStreamProtocol->Request: %v", err)
		return err
	}

	return nil
}

// service
func (p *CustomStreamServiceProtocol) Init(customProtocolID string) {
	p.CustomStreamProtocol.Init(customProtocolID)
}

func (p *CustomStreamServiceProtocol) HandleRequest(protocolRequest *pb.CustomProtocolReq, requestObject any) error {
	if protocolRequest == nil {
		log.Logger.Errorf("CustomStreamServiceProtocol->HandleRequest: request is nil")
		return fmt.Errorf("CustomStreamServiceProtocol->HandleRequest: request is nil")
	}
	log.Logger.Debugf("CustomStreamServiceProtocol->HandleRequest: request: %v", protocolRequest)
	if protocolRequest.PID != p.PID {
		log.Logger.Errorf("CustomStreamServiceProtocol->HandleRequest: request.PID: %v != %v", protocolRequest.PID, p.PID)
		return fmt.Errorf("CustomStreamServiceProtocol->HandleRequest: request.PID: %v != %v", protocolRequest.PID, p.PID)
	}

	err := p.Unmarshal(protocolRequest.Content, requestObject)
	if err != nil {
		log.Logger.Errorf("CustomStreamServiceProtocol->HandleRequest: unmarshal error: %v", err)
		return err
	}

	return nil
}

func (p *CustomStreamServiceProtocol) HandleResponse(protocolResponse *pb.CustomProtocolRes, responseObject any) error {
	if protocolResponse == nil {
		log.Logger.Errorf("CustomStreamServiceProtocol->HandleResponse: request is nil")
		return fmt.Errorf("CustomStreamServiceProtocol->HandleResponse: request is nil")
	}

	if protocolResponse.PID != p.PID {
		log.Logger.Errorf("CustomStreamServiceProtocol->HandleResponse: request.PID: %v != %v", protocolResponse.PID, p.PID)
		return fmt.Errorf("CustomStreamServiceProtocol->HandleResponse: request.PID: %v != %v", protocolResponse.PID, p.PID)
	}

	var err error
	protocolResponse.Content, err = p.Marshal(responseObject)
	if err != nil {
		log.Logger.Errorf("CustomStreamServiceProtocol->HandleResponse: unmarshal error: %v", err)
		return err
	}

	return nil
}
