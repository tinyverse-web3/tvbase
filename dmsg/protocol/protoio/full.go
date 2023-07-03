package protoio

import (
	"io"

	"github.com/libp2p/go-libp2p/core/network"
	"google.golang.org/protobuf/proto"
)

func NewFullWriter(w io.Writer) WriteCloser {
	return &fullWriter{w, nil}
}

type fullWriter struct {
	w      io.Writer
	buffer []byte
}

func (w *fullWriter) WriteMsg(msg proto.Message) (err error) {
	var data []byte
	if m, ok := msg.(marshaler); ok {
		n, ok := getSize(m)
		if !ok {
			_, err = proto.Marshal(msg)
			if err != nil {
				return err
			}
		}
		if n >= len(w.buffer) {
			w.buffer = make([]byte, n)
		}
		_, err = m.MarshalTo(w.buffer)
		if err != nil {
			return err
		}
		data = w.buffer[:n]
	} else {
		data, err = proto.Marshal(msg)
		if err != nil {
			return err
		}
	}
	_, err = w.w.Write(data)
	return err
}

func (w *fullWriter) Close() error {
	if closer, ok := w.w.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

type fullReader struct {
	r io.Reader
}

func NewFullReader(r io.Reader) ReadCloser {
	return &fullReader{r: r}
}

func (r *fullReader) ReadMsg(msg proto.Message) error {
	buf, err := io.ReadAll(r.r)
	if err != nil {
		if stream, ok := r.r.(network.Stream); ok {
			return stream.Reset()
		}
		return err
	}
	err = proto.Unmarshal(buf, msg)
	if err != nil {
		return err
	}
	return nil
}

func (r *fullReader) Close() error {
	if closer, ok := r.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
