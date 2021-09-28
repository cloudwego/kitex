package hotrestart

import (
	"net"
	"testing"
)

// var mockNetwork, mockPrimaryAddr = "tcp", ":8009"
var mockNetwork, mockPrimaryAddr = "unix", "demo/test.primary.sock"

func TestDial(t *testing.T) {
	conn, err := net.Dial(mockNetwork, mockPrimaryAddr)
	if err != nil {
		panic(err)
	}
	conn.Close()
}
