package protocol

import (
	"time"
)

const (
	ProtocolVersion = "0.0.1"
)

const ProtocolRequestTimeout = 10 * time.Second

const (
	PidCreateMailboxReq  = "/tinverseInfrasture/dmsg/createMailbox/req/" + ProtocolVersion
	PidCreateMailboxRes  = "/tinverseInfrasture/dmsg/createMailbox/res/" + ProtocolVersion
	PidReleaseMailboxReq = "/tinverseInfrasture/dmsg/releaseMailbox/req/" + ProtocolVersion
	PidReleaseMailboxRes = "/tinverseInfrasture/dmsg/releaseMailbox/res/" + ProtocolVersion
	PidReadMailboxMsgReq = "/tinverseInfrasture/dmsg/readMailboxMsg/req/" + ProtocolVersion
	PidReadMailboxMsgRes = "/tinverseInfrasture/dmsg/readMailboxMsg/res/" + ProtocolVersion
	PidCustomProtocolReq = "/tinverseInfrasture/dmsg/customProtocol/req/" + ProtocolVersion
	PidCustomProtocolRes = "/tinverseInfrasture/dmsg/customProtocol/res/" + ProtocolVersion
)

type ReqSubscribe interface {
	HandleRequestData(protocolData []byte) error
}

type ResSubscribe interface {
	HandleResponseData(protocolData []byte) error
}
