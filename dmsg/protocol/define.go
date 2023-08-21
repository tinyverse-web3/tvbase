package protocol

const (
	ProtocolVersion = "0.0.1"
)

const (
	PidCreateMailboxReq   = "/tvbase/dmsg/createMailbox/req/" + ProtocolVersion
	PidCreateMailboxRes   = "/tvbase/dmsg/createMailbox/res/" + ProtocolVersion
	PidReleaseMailboxReq  = "/tvbase/dmsg/releaseMailbox/req/" + ProtocolVersion
	PidReleaseMailboxRes  = "/tvbase/dmsg/releaseMailbox/res/" + ProtocolVersion
	PidReadMailboxMsgReq  = "/tvbase/dmsg/readMailboxMsg/req/" + ProtocolVersion
	PidReadMailboxMsgRes  = "/tvbase/dmsg/readMailboxMsg/res/" + ProtocolVersion
	PidCustomProtocolReq  = "/tvbase/dmsg/customProtocol/req/" + ProtocolVersion
	PidCustomProtocolRes  = "/tvbase/dmsg/customProtocol/res/" + ProtocolVersion
	PidCreateChannelReq   = "/tvbase/dmsg/createChannel/req/" + ProtocolVersion
	PidCreateChannelRes   = "/tvbase/dmsg/createChannel/res/" + ProtocolVersion
	PidCreateMsgPubsubReq = "/tvbase/dmsg/createMsgPubsub/req/" + ProtocolVersion
	PidCreateMsgPubsubRes = "/tvbase/dmsg/createMsgPubsub/res/" + ProtocolVersion
)

const AlreadyExistCode = 1
