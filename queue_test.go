package sonic

import (
	"github.com/talostrading/sonic/internal"
	"testing"
)

func Test(t *testing.T) {
	var tests = []struct {
		name  string
		input []*file
	}{
		{},
	}
	for _, tt := range tests {
		ans, _ := tt.name
		t.Run(tt.name, func(t *testing.T) {
			if tt.expected != len(ans) {
				t.Errorf("got %v, expected %v", len(ans), tt.expectedLen)
			}
		})
	}
}

//func newTestFile() *file {
//	fd, localAddr, remoteAddr, err := internal.ConnectTimeout(network, addr, timeout, opts...)
//	if err != nil {
//		return nil, err
//	}
//
//	return newConn(ioc, fd, localAddr, remoteAddr), nil
//}
