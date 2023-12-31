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
	"github.com/tinyverse-web3/tvbase/common/protocol/pb"
	"google.golang.org/protobuf/proto"
)

const (
	RandInfoSize = 32
	timeout      = time.Second * 5
	ID           = "/tvbase/nodeinfo/0.0.1"
	ServiceName  = "tvbase.nodeinfo"
)

type NodeInfoService struct {
	Host     host.Host
	nodeMode config.NodeMode
}

func NewNodeInfoService(h host.Host, m config.NodeMode) *NodeInfoService {
	ps := &NodeInfoService{h, m}
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
	switch p.nodeMode {
	case config.LightMode:
		nodeType = pb.NodeType_Light
	case config.ServiceMode:
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
	return requestNodeInfo(ctx, ps.Host, p)
}

// Request NodeInfo the remote peer until the context is canceled, returning a stream of RTTs or errors.
func requestNodeInfo(ctx context.Context, h host.Host, p peer.ID) *Result {
	s, err := h.NewStream(network.WithUseTransient(ctx, "tvbase/nodeinfo"), p, ID)
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
	defer s.Reset()
	defer close(out)

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	stopRquestSignal := make(chan bool)
	go request(s, mrand.New(mrand.NewSource(int64(binary.BigEndian.Uint64(b)))), out, stopRquestSignal)

	select {
	case <-timer.C:
		stopRquestSignal <- true
		log.Logger.Debug("nodeinfo->RequestNodeInfo: nodeinfo timeout")
	case result = <-out:
		// log.Logger.Debug(result)
	}

	return result
}

func request(s network.Stream, randReader io.Reader, result chan *Result, stopRquestSignal chan bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Errorf("nodeinfo->request: recovered from: r: %v", r)
		}
	}()

	select {
	case <-stopRquestSignal:
		return
	default:
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
}
