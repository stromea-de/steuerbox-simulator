package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// 041 (Nutzer 2026-11-14): „Simulator beenden“ in der Oberfläche beendet genau diesen Dienst.
func TestShutdownFromUIEndsServe041(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan int, 1)
	go func() { result <- serveListener(context.Background(), s, l, io.Discard, io.Discard) }()
	base := "http://" + l.Addr().String()
	var page []byte
	for i := 0; i < 50; i++ {
		if r, e := http.Get(base + "/"); e == nil {
			page, _ = io.ReadAll(r.Body)
			r.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	token := regexp.MustCompile(`name="session-token" content="([0-9a-f]+)"`).FindSubmatch(page)
	if token == nil {
		t.Fatal("no session token")
	}
	req, _ := http.NewRequest("POST", base+"/api/shutdown", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stromea-Token", string(token[1]))
	r, err := http.DefaultClient.Do(req)
	if err != nil || r.StatusCode != http.StatusAccepted {
		t.Fatalf("shutdown request: %v %v", r, err)
	}
	r.Body.Close()
	select {
	case code := <-result:
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("serve did not end after shutdown")
	}
}
