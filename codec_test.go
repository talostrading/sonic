package sonic

import (
	"github.com/talostrading/sonic/sonicerrors"
	"net"
	"testing"
	"time"
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
	n, err := dst.Write(item.V[:])
	dst.Commit(n)
	if err != nil {
		dst.Consume(n) // TODO not really happy about this (same for websocket)
		return err
	}
	return nil
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

	err := codec.Encode(TestItem{V: [5]byte{0x1, 0x2, 0x3, 0x4, 0x5}}, buf)
	if err != nil {
		t.Fatal(err)
	}

	if buf.WriteLen() != 0 {
		t.Fatal("write area should be zero")
	}

	if buf.ReadLen() != 5 || len(buf.Data()) != 5 || buf.Len() != 5 {
		t.Fatal("read area should be 5")
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

func setupCodecTestWriter() chan struct{} {
	mark := make(chan struct{}, 1)
	go func() {
		ln, err := net.Listen("tcp", "localhost:9090")
		if err != nil {
			panic(err)
		}
		defer func() {
			ln.Close()
			mark <- struct{}{}
		}()
		mark <- struct{}{}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		<-mark // send a partial
		n, err := conn.Write([]byte{1, 2, 3})
		if err != nil {
			panic(err)
		}
		if n != 3 {
			panic("did not write 3")
		}

		<-mark // send the rest and one extra byte
		n, err = conn.Write([]byte{
			4,
			5,
			6 /* this is extra and should show up in the src buffer */})
		if err != nil {
			panic(err)
		}
		if n != 3 {
			panic("did not write 3")
		}
	}()
	return mark
}

func TestNonblockingCodecConnAsyncReadNext(t *testing.T) {
	mark := setupCodecTestWriter()
	defer func() { <-mark /* wait for the listener to close*/ }()
	<-mark // wait for the listener to open

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:9090")
	if err != nil {
		t.Fatal(err)
	}

	src := NewByteBuffer()
	dst := NewByteBuffer()
	codecConn, err := NewNonblockingCodecConn[TestItem, TestItem](
		conn, &TestCodec{}, src, dst)

	codecConn.AsyncReadNext(func(err error, item TestItem) {
		if err != nil {
			t.Fatal(err)
		} else {
			for i := 0; i < 5; i++ {
				if item.V[i] != byte(i+1) {
					t.Fatal("wrong decoding")
				}
			}
		}
	})

	// we need 2 reads for a full decode
	mark <- struct{}{} // write first partial chunk
	ioc.RunOne()       // read it

	mark <- struct{}{} // write second chunk + extra byte
	ioc.RunOne()       // read it

	if src.WriteLen() != 1 {
		t.Fatal("should have read the extra byte (6)")
	}

	src.Commit(1)
	if src.Data()[:1][0] != 6 {
		t.Fatal("wrong extra byte")
	}
}

func TestNonblockingCodecConnReadNext(t *testing.T) {
	mark := setupCodecTestWriter()
	defer func() { <-mark /* wait for the listener to close*/ }()
	<-mark // wait for the listener to open

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:9090")
	if err != nil {
		t.Fatal(err)
	}

	src := NewByteBuffer()
	dst := NewByteBuffer()
	codecConn, err := NewNonblockingCodecConn[TestItem, TestItem](
		conn, &TestCodec{}, src, dst)

	item, err := codecConn.ReadNext()
	if err != sonicerrors.ErrWouldBlock {
		t.Fatal("should block")
	}
	for _, v := range item.V {
		if v != 0 {
			t.Fatal("failed read did not return an empty item")
		}
	}

	// we need 2 reads for a full decode
	mark <- struct{}{}                // write first partial chunk
	time.Sleep(10 * time.Millisecond) // wait for the other goroutine to write
	item, err = codecConn.ReadNext()
	if err != sonicerrors.ErrWouldBlock {
		// we got the first chunk but that's a partial and a subsequent
		// read should just block
		t.Fatal("should block")
	}
	for _, v := range item.V {
		if v != 0 {
			t.Fatal("failed read did not return an empty item")
		}
	}

	mark <- struct{}{}                // write second chunk + extra byte
	time.Sleep(10 * time.Millisecond) // wait for the other goroutine to write
	item, err = codecConn.ReadNext()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if item.V[i] != byte(i+1) {
			t.Fatal("wrong decoding")
		}
	}

	if src.WriteLen() != 1 {
		t.Fatal("should have read the extra byte (6)")
	}

	src.Commit(1)
	if src.Data()[:1][0] != 6 {
		t.Fatal("wrong extra byte")
	}
}
