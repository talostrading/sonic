package sonic

import (
	"fmt"
	"github.com/talostrading/sonic/internal"
	"net"
	"syscall"
	"testing"
	"time"
)

func runMulticast(addr string, n int, msg []byte, pause time.Duration) error {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	sockAddr := internal.ToSockaddr(udpAddr)
	if err := syscall.Bind(fd, internal.ToSockaddr(udpAddr)); err != nil {
		return err
	}

	for i := 0; i < n; i++ {
		if err := syscall.Sendto(fd, msg, 0, sockAddr); err != nil {
			return err
		}
		time.Sleep(pause)
	}

	return nil
}

func TestMulticastClientIPv4(t *testing.T) {
	marker := make(chan struct{}, 1)
	defer close(marker)

	//go func() {
	//	<-marker
	//	if err := runMulticast("224.0.1.0:40000", 500, []byte("hello"), time.Millisecond); err != nil {
	//		panic(err)
	//	}
	//}()

	ioc := MustIO()
	defer ioc.Close()

	iff, err := net.InterfaceByName("en0")
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewMulticastClient(ioc, iff)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	marker <- struct{}{}

	if err := client.Join("udp", "224.0.1.0:40000"); err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 128)
	var onRead AsyncReadCallbackPacket
	onRead = func(err error, n int, addr net.Addr) {
		if err != nil {
			t.Fatal(err)
		} else {
			b = b[:n]
			fmt.Println(string(b), addr)
			client.AsyncReadFrom(b, onRead)
		}
	}
	client.AsyncReadFrom(b, onRead)

	ioc.Run()
}
