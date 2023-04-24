package sonic

import (
	"log"
	"net"
	"testing"
)

func TestSocket_BindToDeviceIPv4(t *testing.T) {

	iffs, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}

	for _, iff := range iffs {
		sock, err := NewSocket(SocketDomainIPv4, SocketTypeDatagram, SocketProtocolUDP)
		if err != nil {
			t.Fatal(err)
		}

		log.Printf("attempting to bind socket to device=%s", iff.Name)
		if err := sock.BindToDevice(iff.Name); err != nil {
			t.Fatalf("could not bind socket to %s err=%v", iff.Name, err)
		} else if sock.BoundDevice() == nil {
			t.Fatal("should have set bound device")
		} else {
			log.Printf("bound socket fd=%d to device=%s", sock.RawFd(), iff.Name)
		}

		sock.Close()
	}
}
