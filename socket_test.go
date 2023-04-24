package sonic

import (
	"log"
	"net"
	"testing"
)

func TestSocket_BindToDeviceIPv4(t *testing.T) {
	sock, err := NewSocket(SocketDomainIPv4, SocketTypeDatagram, SocketProtocolUDP)
	if err != nil {
		t.Fatal(err)
	}
	defer sock.Close()

	iffs, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}

	for _, iff := range iffs {
		log.Printf("attempting to bind socket to interface=%s", iff.Name)
		if err := sock.BindToDevice(iff.Name); err != nil {
			t.Fatalf("could not bind socket to lo0 err=%v", err)
		} else if sock.BoundInterface() == nil {
			t.Fatal("should have set bound interface")
		} else {
			log.Printf("bound socket fd=%d to interface=%s", sock.RawFd(), iff.Name)
		}
	}
}
