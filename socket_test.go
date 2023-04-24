package sonic

import "testing"

func TestSocket_BindToDeviceIPv4(t *testing.T) {
	sock, err := NewSocket(SocketDomainIPv4, SocketTypeDatagram, SocketProtocolUDP)
	if err != nil {
		t.Fatal(err)
	}
	defer sock.Close()

	if err := sock.BindToDevice("lo0"); err != nil {
		t.Fatalf("could not bind socket to lo0 err=%v", err)
	}
}
