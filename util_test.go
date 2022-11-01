package sonic

import (
	"fmt"
	"testing"
)

type dummyCodec struct{}

var _ Codec[int, int] = &dummyCodec{}

func (dummyCodec) Encode(_ int, _ *ByteBuffer) error { return nil }
func (dummyCodec) Decode(_ *ByteBuffer) (int, error) { return 0, nil }

func TestGetLowestLayer(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	conn := &sonicConn{}

	codecConn, err := NewCodecConn[int, int](ioc, conn, &dummyCodec{}, NewByteBuffer(), NewByteBuffer())
	if err != nil {
		t.Fatal(err)
	}

	lowest := GetLowestLayer[Conn](codecConn)
	if fmt.Sprintf("%p", lowest) != fmt.Sprintf("%p", conn) {
		t.Fatal("wrong lowest layer")
	}
}
