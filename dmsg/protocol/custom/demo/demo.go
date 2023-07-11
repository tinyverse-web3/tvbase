package demo

import (
	"fmt"
	"time"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
)

/*
    usage
	## implement 1
	Refer to demo's DemoClientProtocol and DemoServiceProtocol to implement your custom protocol
	## client 2
	import (
		tvLog "github.com/tinyverse-web3/tvnode/common/log"
		"github.com/tinyverse-web3/tvbase/tvbase"
		"github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/demo"
	)
	tvbase, err := tvbase.NewTvbase()
	if err != nil {
		tvLog.Logger.Fatalf("NewInfrasture error: %v", err)
	}

	demoClientProtocol := demo.GetDemoClientProtocol()

	// regist Client protocol
	err = tvbase.RegistCSCProtocol(demoClientProtocol)
	if err != nil {
		tvLog.Logger.Errorf("tvbase.RegistCSCProtocol error: %v", err)
		return
	}

	// client request
	demoResponse, err := demoClientProtocol.Request(&demo.DemoRequest{
		ID:     "QmTX7d5vWYrmKzj35MwcEJYsrA6P7Uf6ieWWNJf7kdjdX4"
	})
	if err != nil {
		tvLog.Logger.Errorf("demoClientProtocol.Request error: %v", err)
	}
	tvLog.Logger.Infof("demoResponse: %v", demoResponse)

	//Service 3

	import (
		"github.com/tinyverse-web3/tvbase/tvbase"
		"github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/demo"
	)
	tvbase, err := tvbase.NewInfrasture(rootPath, ctx, true)
	if err != nil {
		panic(err)
	}

	// register protocol
	tvbase.RegistCSSProtocol(demo.GetDemoServiceProtocol())
*/

const demoPID = "demo"

type clientCommicateInfo struct {
	data           any
	responseSignal chan any
}

type DemoRequest struct {
	ID string
}

type DemoResponse struct {
	ID string
}

// client
type DemoClientProtocol struct {
	customProtocol.CustomStreamClientProtocol
	commicateInfoList map[string]*clientCommicateInfo
}

var pullCidClientProtocol *DemoClientProtocol
var pullCidServiceProtocol *DemoServiceProtocol

// GetDemoClientProtocol returns the DemoClientProtocol instance, creating it if it does not exist.
//
// No parameters.
// Returns a pointer to a DemoClientProtocol instance.
func GetDemoClientProtocol() *DemoClientProtocol {
	if pullCidClientProtocol == nil {
		pullCidClientProtocol = &DemoClientProtocol{}
		pullCidClientProtocol.Init()
	}
	return pullCidClientProtocol
}

func (p *DemoClientProtocol) Init() {
	p.CustomStreamClientProtocol.Init(demoPID)
	p.commicateInfoList = make(map[string]*clientCommicateInfo)
}

// HandleResponse handles a response for the DemoClientProtocol.
//
// request: a pointer to a CustomProtocolReq.
// response: a pointer to a CustomProtocolRes.
// error: an error if any occurred.
func (p *DemoClientProtocol) HandleResponse(request *pb.CustomProtocolReq, response *pb.CustomProtocolRes) error {
	pullCidResponse := &DemoResponse{}
	err := p.CustomStreamClientProtocol.HandleResponse(response, pullCidResponse)
	if err != nil {
		customProtocol.Logger.Errorf("DemoClientProtocol->HandleResponse: err: %v", err)
		return fmt.Errorf("DemoClientProtocol->HandleResponse: err: %v", err)
	}
	requestInfo := p.commicateInfoList[pullCidResponse.ID]
	if requestInfo == nil {
		customProtocol.Logger.Errorf("DemoClientProtocol->HandleResponse: requestInfo is nil, cid: %s", pullCidResponse.ID)
		return fmt.Errorf("DemoClientProtocol->HandleResponse: requestInfo is nil, cid: %s", pullCidResponse.ID)
	}

	requestInfo.responseSignal <- pullCidResponse
	delete(p.commicateInfoList, pullCidResponse.ID)

	return nil
}

// Request sends a DemoRequest and waits for a DemoResponse or a timeout to occur.
//
// The function takes a DemoRequest as its first parameter and an optional list of
// options. The options can include a time.Duration for setting the timeout.
// The function returns a DemoResponse and an error type.
func (p *DemoClientProtocol) Request(peerID string, request *DemoRequest, options ...any) (*DemoResponse, error) {
	var defaultTimeout time.Duration = 3 * time.Second
	if len(options) > 0 {
		var ok bool
		defaultTimeout, ok = options[0].(time.Duration)
		if !ok {
			customProtocol.Logger.Errorf("DemoClientProtocol->Request: timeout is not time.Duration")
			return nil, fmt.Errorf("DemoClientProtocol->Request: timeout is not time.Duration")
		}
	}

	requestInfo := &clientCommicateInfo{
		data:           request,
		responseSignal: make(chan any),
	}
	p.commicateInfoList[request.ID] = requestInfo

	err := p.CustomStreamClientProtocol.Request(peerID, request)
	if err != nil {
		customProtocol.Logger.Errorf("DemoClientProtocol->Request: err: %v", err)
		return nil, fmt.Errorf("DemoClientProtocol->Request err: %v", err)
	}

	if defaultTimeout <= 0 {
		customProtocol.Logger.Warnf("DemoClientProtocol->Request: timeout <= 0")
		return nil, nil
	}

	select {
	case responseObject := <-requestInfo.responseSignal:
		pullCidResponse, ok := responseObject.(*DemoResponse)
		if !ok {
			customProtocol.Logger.Errorf("DemoClientProtocol->Request: responseData is not DemoResponse")
			return nil, fmt.Errorf("DemoClientProtocol->Request: responseData is not DemoResponse")
		}
		return pullCidResponse, nil
	case <-time.After(defaultTimeout):
		delete(p.commicateInfoList, request.ID)
		return nil, fmt.Errorf("DemoClientProtocol->Request: timeout")
	case <-p.Ctx.Done():
		delete(p.commicateInfoList, request.ID)
		return nil, p.Ctx.Err()
	}
}

// service
type DemoServiceProtocol struct {
	customProtocol.CustomStreamServiceProtocol
}

// GetDemoServiceProtocol returns a pointer to a DemoServiceProtocol instance.
//
// This function takes no parameters and returns a pointer to DemoServiceProtocol.
func GetDemoServiceProtocol() *DemoServiceProtocol {
	if pullCidServiceProtocol == nil {
		pullCidServiceProtocol = &DemoServiceProtocol{}
		pullCidServiceProtocol.Init()
	}
	return pullCidServiceProtocol
}

func (p *DemoServiceProtocol) Init() {
	p.CustomStreamServiceProtocol.Init(demoPID)
}

// HandleRequest handles the incoming request by parsing the request into a DemoRequest struct and logging any errors.
//
// request: a pointer to a CustomProtocolReq struct containing the incoming request.
// error: returns an error if the request cannot be parsed or if there is an error in the CustomStreamServiceProtocol.
func (p *DemoServiceProtocol) HandleRequest(request *pb.CustomProtocolReq) error {
	pullCidRequest := &DemoRequest{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, pullCidRequest)
	if err != nil {
		customProtocol.Logger.Errorf("DemoServiceProtocol->HandleRequest: err: %v", err)
		return err
	}

	//TODO:	demo nothing here, but your code

	return nil
}

// HandleResponse handles the response for the DemoServiceProtocol.
//
// request: *pb.CustomProtocolReq - the request received.
// response: *pb.CustomProtocolRes - the response sent.
// error - Returns an error if the response handling fails.
func (p *DemoServiceProtocol) HandleResponse(request *pb.CustomProtocolReq, response *pb.CustomProtocolRes) error {
	pullCidRequest := &DemoRequest{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, pullCidRequest)
	if err != nil {
		customProtocol.Logger.Errorf("DemoClientProtocol->HandleResponse: err: %v", err)
		return err
	}

	// TODO: your code, for Client response
	pullCidResponse := &DemoResponse{
		ID: pullCidRequest.ID,
	}

	err = p.CustomStreamServiceProtocol.HandleResponse(response, pullCidResponse)
	if err != nil {
		customProtocol.Logger.Errorf("DemoClientProtocol->HandleResponse: err: %v", err)
		return err
	}

	return nil
}
