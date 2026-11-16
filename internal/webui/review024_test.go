package webui

import (
	"context"
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStateUsesLiveSHIP024(t *testing.T) {
	s, e := session.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := New(s)
	defer a.Close()
	a.connected = true
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:33287/api/state", nil))
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v)
	if v["connected"] != false {
		t.Fatal("cached connection exposed", v["connected"])
	}
	if v["connectionState"] != "disconnected" || v["connectionAt"] == nil {
		t.Fatal("live timestamp absent", v)
	}
}
func TestPreparationListsMissingValues024(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	p := readPreparation(ctx, preparation020{}, &inspection.Capabilities{LPC: true, LPP: true, MPC: true})
	data, _ := json.Marshal(p)
	var v map[string]any
	json.Unmarshal(data, &v)
	if v["values"] == nil {
		t.Fatal("missing per-value evidence", string(data))
	}
}

func TestLiveDisconnectInvalidatesCachedReady024(t *testing.T) {
	s, e := session.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Pair(s.Device.SKI); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e = s.Connect(ctx); e != nil {
		t.Fatal(e)
	}
	a := New(s)
	defer a.Close()
	a.connected = true
	read := func() map[string]any {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:33287/api/state", nil))
		var v map[string]any
		if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	before := read()
	if before["connected"] != true {
		t.Fatal(before)
	}
	s.EG.Disconnect()
	after := read()
	if after["connected"] != false || after["connectionState"] != "disconnected" || after["connectionAt"] == before["connectionAt"] {
		t.Fatal(after)
	}
}
