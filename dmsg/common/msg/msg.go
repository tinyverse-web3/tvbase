package msg

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

type ReceiveMsg struct {
	ID         string
	ReqPubkey  string
	DestPubkey string
	Content    []byte
	TimeStamp  int64
	Direction  string
}

type RespondMsg struct {
	ReqMsgID      string
	ReqPubkey     string
	ReqDestPubkey string
	ReqTimeStamp  int64
	RespMsgID     string
	RespPubkey    string
	RespContent   []byte
	RespTimeStamp int64
}

type OnReceiveMsg func(msg *ReceiveMsg) ([]byte, error)

type OnRespondMsg func(msg *RespondMsg) ([]byte, error)
