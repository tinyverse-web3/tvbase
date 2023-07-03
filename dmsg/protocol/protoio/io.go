package protoio

import (
	"io"

	"google.golang.org/protobuf/proto"
)

type Writer interface {
	WriteMsg(proto.Message) error
}

type WriteCloser interface {
	Writer
	io.Closer
}

type Reader interface {
	ReadMsg(msg proto.Message) error
}

type ReadCloser interface {
	Reader
	io.Closer
}

type marshaler interface {
	MarshalTo(data []byte) (n int, err error)
}

func getSize(v interface{}) (int, bool) {
	if sz, ok := v.(interface {
		Size() (n int)
	}); ok {
		return sz.Size(), true
	} else if sz, ok := v.(interface {
		ProtoSize() (n int)
	}); ok {
		return sz.ProtoSize(), true
	} else {
		return 0, false
	}
}
