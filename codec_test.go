package sonic

import (
	"net"
	"testing"
	"time"

	"github.com/talostrading/sonic/sonicerrors"
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
		defer conn.Close()

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

func TestCodecConnAsyncReadNext(t *testing.T) {
	mark := setupCodecTestWriter()
	defer func() { <-mark /* wait for the listener to close*/ }()
	<-mark // wait for the listener to open

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:9090")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	src := NewByteBuffer()
	dst := NewByteBuffer()
	codecConn, err := NewCodecConn[TestItem, TestItem](
		conn, &TestCodec{}, src, dst)
	if err != nil {
		t.Fatal(err)
	}

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

func TestCodecConnReadNext(t *testing.T) {
	mark := setupCodecTestWriter()
	defer func() { <-mark /* wait for the listener to close*/ }()
	<-mark // wait for the listener to open

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:9090")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	src := NewByteBuffer()
	dst := NewByteBuffer()
	codecConn, err := NewCodecConn[TestItem, TestItem](
		conn, &TestCodec{}, src, dst)
	if err != nil {
		t.Fatal(err)
	}

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

func setupCodecTestReader() chan struct{} {
	mark := make(chan struct{}, 1)
	go func() {
		ln, err := net.Listen("tcp", "localhost:9091")
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
		defer conn.Close()
		mark <- struct{}{}

		b := make([]byte, 128)

		<-mark

		// partial read
		n, err := conn.Read(b[:3])
		if err != nil {
			panic(err)
		}
		if n != 3 {
			panic("should have read 3")
		}

		n, err = conn.Read(b[3:])
		if err != nil {
			panic(err)
		}
		if n != 2 {
			panic("should have read 2")
		}

		for i := 0; i < 5; i++ {
			if b[i] != byte(i+1) {
				panic("wrong read")
			}
		}
	}()
	return mark
}

func TestCodecConnAsyncWriteNext(t *testing.T) {
	mark := setupCodecTestReader()
	defer func() { <-mark /* wait for the listener to close*/ }()
	<-mark // wait for the listener to open

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:9091")
	if err != nil {
		t.Fatal(err)
	}

	src := NewByteBuffer()
	dst := NewByteBuffer()
	codecConn, err := NewCodecConn[TestItem, TestItem](
		conn, &TestCodec{}, src, dst)
	if err != nil {
		t.Fatal(err)
	}

	<-mark // wait to connect

	// trigger the reads
	mark <- struct{}{}
	time.Sleep(100 * time.Millisecond)

	wrote := false
	codecConn.AsyncWriteNext(TestItem{V: [5]byte{1, 2, 3, 4, 5}}, func(err error, n int) {
		wrote = true
	})

	// ioc.RunOne() might actually block because 99% of the time
	// the write can go through immediately; so we cover that 1% of the time
	// with the loop below and the 99% with the wrote variable
	for {
		n, err := ioc.PollOne()
		if err != nil && err != sonicerrors.ErrTimeout {
			t.Fatal(err)
		}
		if n > 0 || wrote {
			break
		}
	}

	if !wrote {
		// if we're in the 1%
		t.Fatal("did not read")
	}
}

func TestCodecConnWriteNext(t *testing.T) {
	mark := setupCodecTestReader()
	defer func() { <-mark /* wait for the listener to close*/ }()
	<-mark // wait for the listener to open

	ioc := MustIO()
	defer ioc.Close()

	conn, err := Dial(ioc, "tcp", "localhost:9091")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	src := NewByteBuffer()
	dst := NewByteBuffer()
	codecConn, err := NewCodecConn[TestItem, TestItem](
		conn, &TestCodec{}, src, dst)
	if err != nil {
		t.Fatal(err)
	}

	<-mark // wait to connect

	item := TestItem{V: [5]byte{1, 2, 3, 4, 5}}
	n, err := codecConn.WriteNext(item)
	if err != nil {
		t.Fatal("should not error")
	}
	if n != 5 {
		t.Fatal("did not write 5 bytes")
	}

	// trigger the reads
	mark <- struct{}{}
	time.Sleep(100 * time.Millisecond)

	// At this point the peer has closed the socket.
	// TCP's design around shutdowns is quite poor mostly due to write buffering.
	// So we wait for at most 5 seconds for the write to fail, if it doesn't
	// then something is wrong with sonic.
	timer := time.NewTimer(5 * time.Second)

	for {
		select {
		case <-timer.C:
			t.Fatal("the write did not fail within 5 seconds")
		default:
		}

		_, err = codecConn.WriteNext(item)
		if err != nil {
			timer.Stop()
			break
		}
	}
}
