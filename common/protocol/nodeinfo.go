package protocol

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	mrand "math/rand"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/log"
	"github.com/tinyverse-web3/tvbase/common/pb"
	"google.golang.org/protobuf/proto"
)

const (
	RandInfoSize = 32
	timeout      = time.Second * 5
	ID           = "/tinverseInfrasture/nodeinfo/0.0.1"
	ServiceName  = "tinverseInfrasture.nodeinfo"
)

type NodeInfoService struct {
	Host       host.Host
	NodeConfig *config.NodeConfig
}

func NewNodeInfoService(h host.Host, c *config.NodeConfig) *NodeInfoService {
	ps := &NodeInfoService{h, c}
	h.SetStreamHandler(ID, ps.requestHandler)
	return ps
}

func (p *NodeInfoService) requestHandler(s network.Stream) {
	if err := s.Scope().SetService(ServiceName); err != nil {
		log.Logger.Errorf("NodeInfoService->RequestHandler: error attaching stream to nodeinfo service: %s, error: %v", ServiceName, err)
		s.Reset()
		return
	}

	defer func() {
		if s != nil {
			s.Close()
		}
	}()

	randBuf := make([]byte, RandInfoSize)
	_, err := io.ReadFull(s, randBuf)
	if err != nil {
		s.Reset()
		s = nil
		log.Logger.Errorf("NodeInfoService->RequestHandler: error: %v", err)
		return
	}

	nodeType := pb.NodeType_Light
	switch p.NodeConfig.Mode {
	case config.LightMode:
		nodeType = pb.NodeType_Light
	case config.FullMode:
		nodeType = pb.NodeType_Full
	}
	nodeInfo := &pb.NodeInfo{
		NodeType: nodeType,
	}

	nodeInfoBuf, err := proto.Marshal(nodeInfo)
	if err != nil {
		log.Logger.Errorf("NodeInfoService->RequestHandler: error: %v", err)
		return
	}

	buf := append(randBuf, nodeInfoBuf...)
	_, err = s.Write(buf)
	if err != nil {
		s.Reset()
		s = nil
		log.Logger.Errorf("NodeInfoService->RequestHandler: error: %v", err)
		return
	}
}

// Result is a result of a request nodeInfo attempt, either an RTT or an error.
type Result struct {
	NodeInfo *pb.NodeInfo
	Error    error
}

func (ps *NodeInfoService) Request(ctx context.Context, p peer.ID) *Result {
	return RequestNodeInfo(ctx, ps.Host, p)
}

// Request NodeInfo the remote peer until the context is canceled, returning a stream of RTTs or errors.
func RequestNodeInfo(ctx context.Context, h host.Host, p peer.ID) *Result {
	s, err := h.NewStream(network.WithUseTransient(ctx, "tinverseInfrasture/nodeinfo"), p, ID)
	if err != nil {
		return &Result{Error: err}
	}

	if err := s.Scope().SetService(ServiceName); err != nil {
		log.Logger.Errorf("nodeinfo->RequestNodeInfo: error attaching stream to ping service: %s", err)
		s.Reset()
		return &Result{Error: err}
	}

	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		log.Logger.Errorf("nodeinfo->RequestNodeInfo: failed to get cryptographic random: %s", err)
		s.Reset()
		return &Result{Error: err}
	}

	out := make(chan *Result)
	var result *Result
	defer close(out)
	defer s.Reset()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	go request(s, mrand.New(mrand.NewSource(int64(binary.BigEndian.Uint64(b)))), out)

	select {
	case <-timer.C:
		log.Logger.Debug("nodeinfo->RequestNodeInfo: nodeinfo timeout")
	case result = <-out:
		log.Logger.Debug(result)
	}

	return result
}

func request(s network.Stream, randReader io.Reader, result chan *Result) {
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Errorf("nodeinfo->RequestNodeInfo: StreamProtocol->OnResponse: recovered from: %v", r)
		}
	}()

	buf := make([]byte, RandInfoSize)
	if _, err := io.ReadFull(randReader, buf); err != nil {
		result <- &Result{nil, err}
		return
	}

	if _, err := s.Write(buf); err != nil {
		result <- &Result{nil, err}
		return
	}

	rbuf := make([]byte, RandInfoSize)
	if _, err := io.ReadFull(s, rbuf); err != nil {
		result <- &Result{nil, err}
		return
	}

	if !bytes.Equal(buf, rbuf) {
		result <- &Result{nil, errors.New("nodeinfo->request: nodeinfo packet was incorrect")}
		return
	}

	nodeInfoBuf, err := io.ReadAll(s)
	if err != nil {
		result <- &Result{nil, err}
		return
	}
	nodeInfo := &pb.NodeInfo{}
	err = proto.Unmarshal(nodeInfoBuf, nodeInfo)
	if err != nil {
		result <- &Result{nil, err}
		return
	}
	result <- &Result{nodeInfo, err}
}
