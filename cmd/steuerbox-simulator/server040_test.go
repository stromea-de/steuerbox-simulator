package main

import (
	"testing"
	"time"
)

// 040 F-03: the local HTTP server bounds the whole request read, not only the header.
func TestHTTPServerReadTimeout040(t *testing.T) {
	s := newHTTPServer(nil)
	if s.ReadTimeout <= 0 || s.ReadTimeout > time.Minute || s.ReadHeaderTimeout <= 0 {
		t.Fatalf("timeouts: read=%s header=%s", s.ReadTimeout, s.ReadHeaderTimeout)
	}
}
