package protocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	ProtocolVersion = "0.0.1"
)

const (
	PidCreateMailboxReq  = "/tvbase/dmsg/createMailbox/req/" + ProtocolVersion
	PidCreateMailboxRes  = "/tvbase/dmsg/createMailbox/res/" + ProtocolVersion
	PidReleaseMailboxReq = "/tvbase/dmsg/releaseMailbox/req/" + ProtocolVersion
	PidReleaseMailboxRes = "/tvbase/dmsg/releaseMailbox/res/" + ProtocolVersion
	PidReadMailboxMsgReq = "/tvbase/dmsg/readMailboxMsg/req/" + ProtocolVersion
	PidReadMailboxMsgRes = "/tvbase/dmsg/readMailboxMsg/res/" + ProtocolVersion
	PidCustomProtocolReq = "/tvbase/dmsg/customProtocol/req/" + ProtocolVersion
	PidCustomProtocolRes = "/tvbase/dmsg/customProtocol/res/" + ProtocolVersion
	PidCreateChannelReq  = "/tvbase/dmsg/createPubChannel/req/" + ProtocolVersion
	PidCreateChannelRes  = "/tvbase/dmsg/createPubChannel/res/" + ProtocolVersion
)

const AlreadyExistCode = 1

type RequestInfo struct {
	ProtoMessage    protoreflect.ProtoMessage
	CreateTimestamp int64
	DoneChan        chan any
}

type Protocol struct {
	Ctx             context.Context
	Host            host.Host
	RequestInfoList map[string]*RequestInfo
	Service         ProtocolService
	Adapter         ProtocolAdapter
}
