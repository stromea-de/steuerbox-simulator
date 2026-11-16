package session

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	shipapi "github.com/enbility/ship-go/api"
	"github.com/enbility/ship-go/ship"
	"github.com/enbility/ship-go/ws"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
	"github.com/gorilla/websocket"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// This peer owns bindings per transport, independently of spine-go's caches.
// Ending a session loses them, as on a device that restarts its SPINE session.
type bindingPeer023b struct {
	*eebus.Endpoint
	reject      bool
	mu          sync.Mutex
	transport   *websocket.Conn
	sessions    []*bindingReader023b
	dropOnLimit bool
}
type bindingReader023b struct {
	next                    shipapi.ShipConnectionDataReaderInterface
	writer                  shipapi.ShipConnectionDataWriterInterface
	reject                  bool
	mu                      sync.Mutex
	bindings, subscriptions map[string]bool
	denied                  int
	after                   func()
}

func featurePair023b(client, server *model.FeatureAddressType) string {
	b, _ := json.Marshal([]*model.FeatureAddressType{client, server})
	return string(b)
}
func (p *bindingPeer023b) SetupRemoteDevice(s string, w shipapi.ShipConnectionDataWriterInterface) shipapi.ShipConnectionDataReaderInterface {
	r := &bindingReader023b{next: p.Endpoint.SetupRemoteDevice(s, w), writer: w, reject: p.reject, bindings: map[string]bool{}, subscriptions: map[string]bool{}}
	r.after = func() {
		p.mu.Lock()
		drop := p.dropOnLimit
		p.dropOnLimit = false
		p.mu.Unlock()
		if drop {
			p.drop()
		}
	}
	p.mu.Lock()
	p.sessions = append(p.sessions, r)
	p.mu.Unlock()
	return r
}
func (r *bindingReader023b) HandleShipPayloadMessage(b []byte) {
	var d model.Datagram
	if json.Unmarshal(b, &d) == nil {
		h := d.Datagram.Header
		r.mu.Lock()
		for _, c := range d.Datagram.Payload.Cmd {
			if v := c.NodeManagementBindingRequestCall; v != nil && v.BindingRequest != nil {
				q := v.BindingRequest
				r.bindings[featurePair023b(q.ClientAddress, q.ServerAddress)] = true
			}
			if v := c.NodeManagementSubscriptionRequestCall; v != nil && v.SubscriptionRequest != nil {
				q := v.SubscriptionRequest
				r.subscriptions[featurePair023b(q.ClientAddress, q.ServerAddress)] = true
			}
		}
		denied := h.CmdClassifier != nil && *h.CmdClassifier == model.CmdClassifierTypeWrite && !r.bindings[featurePair023b(h.AddressSource, h.AddressDestination)]
		if denied {
			r.denied++
		}
		r.mu.Unlock()
		if denied {
			if r.reject {
				reply := model.Datagram{Datagram: model.DatagramType{Header: model.HeaderType{SpecificationVersion: h.SpecificationVersion, AddressSource: h.AddressDestination, AddressDestination: h.AddressSource, MsgCounter: util.Ptr(model.MsgCounterType(999)), MsgCounterReference: h.MsgCounter, CmdClassifier: util.Ptr(model.CmdClassifierTypeResult)}, Payload: model.PayloadType{Cmd: []model.CmdType{{ResultData: &model.ResultDataType{ErrorNumber: util.Ptr(model.ErrorNumberTypeCommandRejected)}}}}}}
				raw, _ := json.Marshal(reply)
				r.writer.WriteShipMessageWithPayload(raw)
			}
			return
		}
	}
	r.next.HandleShipPayloadMessage(b)
	if r.after != nil && d.Datagram.Header.CmdClassifier != nil && *d.Datagram.Header.CmdClassifier == model.CmdClassifierTypeWrite {
		for _, c := range d.Datagram.Payload.Cmd {
			if c.LoadControlLimitListData != nil {
				r.after()
				break
			}
		}
	}
}
func (p *bindingPeer023b) drop() {
	p.mu.Lock()
	c := p.transport
	p.mu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}
func newBindingPeer023b(t *testing.T, reject bool, localSKI string) (*bindingPeer023b, string) {
	t.Helper()
	id, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ep, err := eebus.NewEndpoint(id, true)
	if err != nil {
		t.Fatal(err)
	}
	p := &bindingPeer023b{Endpoint: ep, reject: reject}
	if err = p.Trust(localSKI); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := (&websocket.Upgrader{Subprotocols: []string{"ship"}}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		p.mu.Lock()
		p.transport = c
		p.mu.Unlock()
		h := ship.NewConnectionHandler(p, ws.NewWebsocketConnection(c, localSKI), ship.ShipRoleServer, id.SKI, localSKI, "")
		h.Run()
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{id.Certificate}, ClientAuth: tls.RequireAnyClientCert}
	server.StartTLS()
	t.Cleanup(func() { p.drop(); server.Close(); ep.Close() })
	return p, server.Listener.Addr().String()
}
func TestReconnectFreshBindings023b(t *testing.T) {
	for _, reject := range []bool{true, false} {
		name := "silent"
		if reject {
			name = "commandRejected"
		}
		t.Run(name, func(t *testing.T) {
			s, err := NewController(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			peer, address := newBindingPeer023b(t, reject, s.EG.Identity.SKI)
			d := s.Device
			d.SKI = peer.Identity.SKI
			d.Address = address
			if err = s.SelectDevice(d, true); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err = s.Connect(ctx); err != nil {
				t.Fatal(err)
			}
			check := func() {
				t.Helper()
				for _, kind := range []string{"LPC", "LPP"} {
					short, stop := context.WithTimeout(ctx, 500*time.Millisecond)
					_, err := s.Limit(short, kind, 1000, true)
					stop()
					if err != nil {
						t.Fatalf("%s write after connection: %v", kind, err)
					}
					v, err := s.ReadFailsafe(ctx, kind)
					if err != nil {
						t.Fatal(err)
					}
					if err = s.WriteFailsafe(ctx, kind, "minimum", v); err != nil {
						t.Fatal(err)
					}
				}
			}
			check()
			// A hard TCP loss, followed by the automatic in-run recovery path on the same endpoint.
			before := s.EG
			peer.drop()
			if err = eebus.Poll(ctx, func() bool { return !s.EG.Alive() }); err != nil {
				t.Fatal(err)
			}
			if err = s.Recover(ctx); err != nil {
				t.Fatal(err)
			}
			if before != s.EG {
				t.Fatal("automatic recovery replaced endpoint instead of exercising stale caches")
			}
			check()
			// The explicit action uses a new controller, but the same peer identity/address.
			if err = s.Rebuild(ctx); err != nil {
				t.Fatal(err)
			}
			check()
			peer.mu.Lock()
			sessions := append([]*bindingReader023b(nil), peer.sessions...)
			peer.mu.Unlock()
			if len(sessions) != 3 {
				t.Fatalf("sessions: %d", len(sessions))
			}
			sessions[0].mu.Lock()
			expectedBindings, expectedSubs := len(sessions[0].bindings), len(sessions[0].subscriptions)
			sessions[0].mu.Unlock()
			for i, r := range sessions {
				r.mu.Lock()
				bindings, subs, denied := len(r.bindings), len(r.subscriptions), r.denied
				r.mu.Unlock()
				if bindings != expectedBindings || subs != expectedSubs || bindings < 2 || subs < 3 || denied != 0 {
					t.Fatalf("session %d: bindings=%d subscriptions=%d unbound writes=%d", i, bindings, subs, denied)
				}
			}
			// Verify the peer's two refusal modes independently of the reconnect assertion.
			last := sessions[len(sessions)-1]
			last.mu.Lock()
			last.bindings = map[string]bool{}
			last.mu.Unlock()
			short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
			_, err = s.Limit(short, "LPC", 1200, true)
			stop()
			if reject {
				if err == nil || !strings.Contains(err.Error(), "vom Gerät abgelehnt") || s.ControlError() != nil {
					t.Fatalf("explicit denial: %v", err)
				}
			} else if !errors.Is(err, inspection.ErrControlUnresponsive) {
				t.Fatalf("silent denial: %v", err)
			}
		})
	}
}
func TestCommandRejectedIsDeviceRejection023b(t *testing.T) {
	for _, counter := range []model.MsgCounterType{7, 23} {
		s := &Session{}
		ch := make(chan model.ResultDataType, 1)
		ch <- model.ResultDataType{ErrorNumber: util.Ptr(model.ErrorNumberTypeCommandRejected)}
		err := s.waitControlResult(context.Background(), counter, ch)
		if err == nil || !strings.Contains(err.Error(), "vom Gerät abgelehnt") || !strings.Contains(err.Error(), "keine Bindung/abgelehnt") || !strings.Contains(err.Error(), "commandRejected") || errors.Is(err, context.DeadlineExceeded) || s.ControlError() != nil {
			t.Fatalf("rejection misreported: %v", err)
		}
	}
}

// Loss during a real running inspection must cancel it, then use an independent
// cleanup context to reconnect, release and restore with fresh peer bindings.
func TestReconnectDuringRunCleanup023b(t *testing.T) {
	for _, reject := range []bool{true, false} {
		name := "silent"
		if reject {
			name = "commandRejected"
		}
		t.Run(name, func(t *testing.T) {
			s, err := NewController(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			peer, address := newBindingPeer023b(t, reject, s.EG.Identity.SKI)
			d := s.Device
			d.SKI = peer.Identity.SKI
			d.Address = address
			if err = s.SelectDevice(d, true); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			if err = s.Connect(ctx); err != nil {
				t.Fatal(err)
			}
			peer.mu.Lock()
			peer.dropOnLimit = true
			peer.mu.Unlock()
			c := inspection.DefaultDeviceConfig()
			c.Business = "Elektro Muster"
			c.Inspector = "Erika Muster"
			c.Authorized = true
			c.Scope = inspection.ScopeConsumption
			c.StepTimeout = 3 * time.Second
			c.EffectTimeout = time.Second
			c.CleanupTimeout = 3 * time.Second
			c.PauseLowLoad = false
			r, err := s.Run(ctx, c)
			if err != nil {
				t.Fatal(err)
			}
			if r.Passed || r.ControlUnresponsive {
				t.Fatal("transport loss must remain failed without timeout latch", r.Passed, r.ControlUnresponsive)
			}
			found := map[string]bool{}
			for _, step := range r.Steps {
				if strings.Contains(step.Name, "zurückstellen") || step.Name == "Grenzen aufheben" {
					if !step.Passed {
						t.Fatalf("cleanup did not recover: %s: %s", step.Name, step.Detail)
					}
					found[step.Name] = true
				}
			}
			if len(found) != 3 {
				t.Fatal("missing confirmed release/restoration", found)
			}
			if len(r.Events) < 2 {
				t.Fatal("connection loss/recovery absent from report", r.Events)
			}
			peer.mu.Lock()
			count := len(peer.sessions)
			peer.mu.Unlock()
			if count != 2 {
				t.Fatalf("expected initial and cleanup transport, got %d", count)
			}
		})
	}
}
