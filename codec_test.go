package sonic

import (
	"testing"
)

var _ Codec[TestItem, TestItem] = &TestCodec{}

type TestItem struct {
	V [5]byte
}

type TestCodec struct {
	item      TestItem
	emptyItem TestItem
}

func (t *TestCodec) Encode(item TestItem, dst *ByteBuffer) error {
	_, err := dst.Write(item.V[:])
	return err
}

func (t *TestCodec) Decode(src *ByteBuffer) (TestItem, error) {
	if err := src.PrepareRead(5); err != nil {
		return t.emptyItem, err
	}

	item := TestItem{}
	copy(item.V[:], src.Data()[:5])
	src.Consume(5)
	return item, nil
}

func TestSimpleCodec(t *testing.T) {
	codec := &TestCodec{}

	buf := NewByteBuffer()
	buf.Reserve(128)

	if err := codec.Encode(TestItem{V: [5]byte{0x1, 0x2, 0x3, 0x4, 0x5}}, buf); err != nil {
		t.Fatal(err)
	}

	item, err := codec.Decode(buf)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(item.V); i++ {
		if item.V[i] != byte(i+1) {
			t.Fatal("wrong decoding")
		}
	}
}
