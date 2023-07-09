package dmsg

import (
	"crypto/ecdsa"
	"time"
)

const (
	DiscoveryConnTimeout = time.Second * 30
)

type SendMsgData struct {
	SrcUserPubkeyHex  string
	DestUserPubkeyHex string
	// SrcUserPrikey     *ecdsa.PrivateKey
	SrcUserPubkey *ecdsa.PublicKey
	Sign          []byte
	MsgContent    []byte
}

const (
	MsgPrefix              = "/dmsg/"
	MsgKeyDelimiter        = "/"
	MsgFieldsLen           = 7
	MsgSrcUserPubKeyIndex  = 2
	MsgDestUserPubKeyIndex = 3
	MsgDirectionIndex      = 4
	MsgIDIndex             = 5
	MsgTimeStampIndex      = 6
)

type MsgDirectionStruct struct {
	From string
	To   string
}

var MsgDirection = MsgDirectionStruct{
	From: "from",
	To:   "to",
}
