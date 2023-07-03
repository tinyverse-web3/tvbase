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
	Refer to demo's DemoLightProtocol and DemoServiceProtocol to implement your custom protocol
	## light 2
	import (
		tvLog "github.com/tinyverse-web3/tvnode/common/log"
		"github.com/tinyverse-web3/infrasture/infrasture"
		"github.com/tinyverse-web3/infrasture/dmsg/protocol/custom/demo"
	)
	infrasture, err := infrasture.NewInfrasture()
	if err != nil {
		tvLog.Logger.Fatalf("NewInfrasture error: %v", err)
	}

	demoLightProtocol := demo.GetDemoLightProtocol()

	// regist Light protocol
	err = infrasture.RegistCSCProtocol(demoLightProtocol)
	if err != nil {
		tvLog.Logger.Errorf("infrasture.RegistCSCProtocol error: %v", err)
		return
	}

	// light request
	demoResponse, err := demoLightProtocol.Request(&demo.DemoRequest{
		ID:     "QmTX7d5vWYrmKzj35MwcEJYsrA6P7Uf6ieWWNJf7kdjdX4"
	})
	if err != nil {
		tvLog.Logger.Errorf("demoLightProtocol.Request error: %v", err)
	}
	tvLog.Logger.Infof("demoResponse: %v", demoResponse)

	//Service 3

	import (
		"github.com/tinyverse-web3/infrasture/infrasture"
		"github.com/tinyverse-web3/infrasture/dmsg/protocol/custom/demo"
	)
	infrasture, err := infrasture.NewInfrasture(rootPath, ctx, true)
	if err != nil {
		panic(err)
	}

	// register protocol
	infrasture.RegistCSSProtocol(demo.GetDemoServiceProtocol())
*/

const demoPID = "demo"

type lightCommicateInfo struct {
	data           any
	responseSignal chan any
}

type DemoRequest struct {
	ID string
}

type DemoResponse struct {
	ID string
}

// light
type DemoLightProtocol struct {
	customProtocol.CustomStreamLightProtocol
	commicateInfoList map[string]*lightCommicateInfo
}

var pullCidLightProtocol *DemoLightProtocol
var pullCidServiceProtocol *DemoServiceProtocol

// GetDemoLightProtocol returns the DemoLightProtocol instance, creating it if it does not exist.
//
// No parameters.
// Returns a pointer to a DemoLightProtocol instance.
func GetDemoLightProtocol() *DemoLightProtocol {
	if pullCidLightProtocol == nil {
		pullCidLightProtocol = &DemoLightProtocol{}
		pullCidLightProtocol.Init()
	}
	return pullCidLightProtocol
}

func (p *DemoLightProtocol) Init() {
	p.CustomStreamLightProtocol.Init(demoPID)
	p.commicateInfoList = make(map[string]*lightCommicateInfo)
}

// HandleResponse handles a response for the DemoLightProtocol.
//
// request: a pointer to a CustomProtocolReq.
// response: a pointer to a CustomProtocolRes.
// error: an error if any occurred.
func (p *DemoLightProtocol) HandleResponse(request *pb.CustomProtocolReq, response *pb.CustomProtocolRes) error {
	pullCidResponse := &DemoResponse{}
	err := p.CustomStreamLightProtocol.HandleResponse(response, pullCidResponse)
	if err != nil {
		customProtocol.Logger.Errorf("DemoLightProtocol->HandleResponse: err: %v", err)
		return fmt.Errorf("DemoLightProtocol->HandleResponse: err: %v", err)
	}
	requestInfo := p.commicateInfoList[pullCidResponse.ID]
	if requestInfo == nil {
		customProtocol.Logger.Errorf("DemoLightProtocol->HandleResponse: requestInfo is nil, cid: %s", pullCidResponse.ID)
		return fmt.Errorf("DemoLightProtocol->HandleResponse: requestInfo is nil, cid: %s", pullCidResponse.ID)
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
func (p *DemoLightProtocol) Request(request *DemoRequest, options ...any) (*DemoResponse, error) {
	var defaultTimeout time.Duration = 3 * time.Second
	if len(options) > 0 {
		var ok bool
		defaultTimeout, ok = options[0].(time.Duration)
		if !ok {
			customProtocol.Logger.Errorf("DemoLightProtocol->Request: timeout is not time.Duration")
			return nil, fmt.Errorf("DemoLightProtocol->Request: timeout is not time.Duration")
		}
	}

	requestInfo := &lightCommicateInfo{
		data:           request,
		responseSignal: make(chan any),
	}
	p.commicateInfoList[request.ID] = requestInfo

	err := p.CustomStreamLightProtocol.Request(request)
	if err != nil {
		customProtocol.Logger.Errorf("DemoLightProtocol->Request: err: %v", err)
		return nil, fmt.Errorf("DemoLightProtocol->Request err: %v", err)
	}

	if defaultTimeout <= 0 {
		customProtocol.Logger.Warnf("DemoLightProtocol->Request: timeout <= 0")
		return nil, nil
	}

	select {
	case responseObject := <-requestInfo.responseSignal:
		pullCidResponse, ok := responseObject.(*DemoResponse)
		if !ok {
			customProtocol.Logger.Errorf("DemoLightProtocol->Request: responseData is not DemoResponse")
			return nil, fmt.Errorf("DemoLightProtocol->Request: responseData is not DemoResponse")
		}
		return pullCidResponse, nil
	case <-time.After(defaultTimeout):
		delete(p.commicateInfoList, request.ID)
		return nil, fmt.Errorf("DemoLightProtocol->Request: timeout")
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
		customProtocol.Logger.Errorf("DemoLightProtocol->HandleResponse: err: %v", err)
		return err
	}

	// TODO: your code, for light response
	pullCidResponse := &DemoResponse{
		ID: pullCidRequest.ID,
	}

	err = p.CustomStreamServiceProtocol.HandleResponse(response, pullCidResponse)
	if err != nil {
		customProtocol.Logger.Errorf("DemoLightProtocol->HandleResponse: err: %v", err)
		return err
	}

	return nil
}
