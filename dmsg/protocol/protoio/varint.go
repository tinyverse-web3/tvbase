package protoio

import (
	"bufio"
	"encoding/binary"
	"io"

	"google.golang.org/protobuf/proto"
)

func NewDelimitedWriter(w io.Writer) WriteCloser {
	return &varintWriter{w, make([]byte, binary.MaxVarintLen64), nil}
}

type varintWriter struct {
	w      io.Writer
	lenBuf []byte
	buffer []byte
}

func (v *varintWriter) WriteMsg(msg proto.Message) (err error) {
	var data []byte
	if m, ok := msg.(marshaler); ok {
		n, ok := getSize(m)
		if ok {
			if n+binary.MaxVarintLen64 >= len(v.buffer) {
				v.buffer = make([]byte, n+binary.MaxVarintLen64)
			}
			lenOff := binary.PutUvarint(v.buffer, uint64(n))
			_, err = m.MarshalTo(v.buffer[lenOff:])
			if err != nil {
				return err
			}
			_, err = v.w.Write(v.buffer[:lenOff+n])
			return err
		}
	}

	// fallback
	data, err = proto.Marshal(msg)
	if err != nil {
		return err
	}
	length := uint64(len(data))
	n := binary.PutUvarint(v.lenBuf, length)
	_, err = v.w.Write(v.lenBuf[:n])
	if err != nil {
		return err
	}
	_, err = v.w.Write(data)
	return err
}

func (v *varintWriter) Close() error {
	if closer, ok := v.w.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func NewDelimitedReader(r io.Reader, maxSize int) ReadCloser {
	var closer io.Closer
	if c, ok := r.(io.Closer); ok {
		closer = c
	}
	return &varintReader{bufio.NewReader(r), nil, maxSize, closer}
}

type varintReader struct {
	r       *bufio.Reader
	buf     []byte
	maxSize int
	closer  io.Closer
}

func (v *varintReader) ReadMsg(msg proto.Message) error {
	length64, err := binary.ReadUvarint(v.r)
	if err != nil {
		return err
	}
	length := int(length64)
	if length < 0 || length > v.maxSize {
		return io.ErrShortBuffer
	}
	if len(v.buf) < length {
		v.buf = make([]byte, length)
	}
	buf := v.buf[:length]
	if _, err := io.ReadFull(v.r, buf); err != nil {
		return err
	}
	return proto.Unmarshal(buf, msg)
}

func (v *varintReader) Close() error {
	if v.closer != nil {
		return v.closer.Close()
	}
	return nil
}
