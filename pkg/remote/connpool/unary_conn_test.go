package connpool

import "testing"

func TestUnaryConnIDNoAlloc(t *testing.T) {
	for _, c := range []interface{ ConnID() uint64 }{new(longConn), new(shortConn)} {
		id := c.ConnID()
		if id == 0 {
			t.Fatal("missing connection ID")
		}
		if n := testing.AllocsPerRun(1000, func() {
			if c.ConnID() != id {
				t.Fatal("connection ID changed")
			}
		}); n != 0 {
			t.Fatalf("%T: %v allocations per connection ID read", c, n)
		}
	}
}
