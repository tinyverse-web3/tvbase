package protocol

const (
	ProtocolVersion = "0.0.1"
)

const (
	PidCreateMailboxReq    = "/tvbase/dmsg/createMailbox/req/" + ProtocolVersion
	PidCreateMailboxRes    = "/tvbase/dmsg/createMailbox/res/" + ProtocolVersion
	PidReleaseMailboxReq   = "/tvbase/dmsg/releaseMailbox/req/" + ProtocolVersion
	PidReleaseMailboxRes   = "/tvbase/dmsg/releaseMailbox/res/" + ProtocolVersion
	PidReadMailboxMsgReq   = "/tvbase/dmsg/readMailboxMsg/req/" + ProtocolVersion
	PidReadMailboxMsgRes   = "/tvbase/dmsg/readMailboxMsg/res/" + ProtocolVersion
	PidCustomProtocolReq   = "/tvbase/dmsg/customProtocol/req/" + ProtocolVersion
	PidCustomProtocolRes   = "/tvbase/dmsg/customProtocol/res/" + ProtocolVersion
	PidCreatePubChannelReq = "/tvbase/dmsg/createPubChannel/req/" + ProtocolVersion
	PidCreatePubChannelRes = "/tvbase/dmsg/createPubChannel/res/" + ProtocolVersion
)

type ReqSubscribe interface {
	HandleRequestData(protocolData []byte) error
}

type ResSubscribe interface {
	HandleResponseData(protocolData []byte) error
}
