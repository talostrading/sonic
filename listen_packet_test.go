package sonic

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestPacketUDPAsyncRead(t *testing.T) {
	marker := make(chan struct{}, 1)
	defer close(marker)
	go func() {
		udpAddr, err := net.ResolveUDPAddr("udp", "localhost:8085")
		if err != nil {
			panic(err)
		}
		udp, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			panic(err)
		}
		defer udp.Close()

		<-marker

		for i := 0; i < 1000; i++ {
			udp.Write([]byte("hello"))
			time.Sleep(time.Millisecond)
		}

		marker <- struct{}{}
	}()

	ioc := MustIO()
	defer ioc.Close()

	conn, err := ListenPacket(ioc, "udp", "localhost:8085")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	nread := 0
	b := make([]byte, 128)
	var onRead AsyncReadCallbackPacket
	onRead = func(err error, n int, fromAddr net.Addr) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
		} else {
			b = b[:n]
			if string(b) != "hello" {
				t.Fatal("wrong message")
			}

			nread++

			select {
			case <-marker:
			default:
				b = b[:cap(b)]
				conn.AsyncReadFrom(b, onRead)
			}
		}
	}

	conn.AsyncReadFrom(b, onRead)
	marker <- struct{}{}

	now := time.Now()
	for nread < 5 && time.Now().Sub(now) < 2*time.Second {
		ioc.PollOne()
	}
	if nread == 0 {
		t.Fatal("did not read anything")
	}
	<-marker
}
