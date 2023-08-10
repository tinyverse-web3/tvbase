package dmsg

import (
	"time"
)

const (
	DiscoveryConnTimeout = time.Second * 30
)

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

type Msg struct {
	ID         string
	SrcPubkey  string
	DestPubkey string
	Content    []byte
	TimeStamp  int64
	Direction  string
}

type OnReceiveMsg func(srcUserPubkey string, destUserPubkey string, msgContent []byte, timeStamp int64, msgID string, direction string)
