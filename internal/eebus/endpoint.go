// Package eebus adapts the upstream SHIP/SPINE stack to a strictly loopback transport.
package eebus

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enbility/eebus-go/api"
	"github.com/enbility/eebus-go/service"
	cslpc "github.com/enbility/eebus-go/usecases/cs/lpc"
	cslpp "github.com/enbility/eebus-go/usecases/cs/lpp"
	eglpc "github.com/enbility/eebus-go/usecases/eg/lpc"
	eglpp "github.com/enbility/eebus-go/usecases/eg/lpp"
	"github.com/enbility/eebus-go/usecases/ma/mpc"
	shipapi "github.com/enbility/ship-go/api"
	"github.com/enbility/ship-go/cert"
	shipmodel "github.com/enbility/ship-go/model"
	"github.com/enbility/ship-go/ship"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/spine"
	"github.com/gorilla/websocket"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
)

const HeartbeatInterval = time.Second
const FailsafeAfter = 2500 * time.Millisecond

// spine-go's process-global event handler list is not copied under its lock.
// Serialize registration/removal against our synchronous publisher entry points.
var stackLifecycle sync.RWMutex

func LoopbackAddress(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	p, e := strconv.Atoi(port)
	if ip == nil || !ip.IsLoopback() || e != nil || p < 1 || p > 65535 {
		return errors.New("nur Loopback-IP und gültiger Port erlaubt")
	}
	return nil
}

type Endpoint struct {
	practiceSpeed        atomic.Int32
	Identity             identity.Identity
	Service              *service.Service
	Entity               spineapi.EntityLocalInterface
	LPC                  *eglpc.LPC
	LPP                  *eglpp.LPP
	MPC                  *mpc.MPC
	simLimitsMu          sync.Mutex // protects local approved-limit value snapshots
	simConsumptionLimit  simulatorLimit
	simProductionLimit   simulatorLimit
	CSLPC                *cslpc.LPC
	CSLPP                *cslpp.LPP
	listener             net.Listener
	server               *http.Server
	mu                   sync.Mutex
	spineMu              sync.RWMutex // remote replacement vs readers, event handlers and map lookups
	tlsVerified          bool
	trusted              map[string]bool
	connections          []*ship.ShipConnection
	transports           []*connectionTransport
	peer                 string
	lastHeartbeat        time.Time
	heartbeatMu          sync.Mutex
	heartbeatBlocked     bool
	heartbeatPending     *heartbeatReceipt
	production           bool
	closed               chan struct{}
	closeOnce            sync.Once
	handlers             []spineapi.EventHandlerInterface
	powerMu              sync.Mutex
	powerPending         map[model.MsgCounterType]powerRequest
	lastPowerMeasured    time.Time
	configurationPending map[model.MsgCounterType]configurationRequest
	limitsPending        map[model.MsgCounterType]limitsRequest
	deviceMode           bool
	status               string
	alive                bool
	handshakeState       shipmodel.ShipMessageExchangeState
	options              EndpointOptions
	effectReadyAt        time.Time
	useCasesReceived     bool
	connectionTrace      *connectionTrace
	// remoteSetup/remoteConnection record which transport created the current
	// SPINE remote device. spine-go keeps a remote device after its transport
	// closes; without this, a reconnect could act on the stale device (039 B).
	remoteSetup      bool
	remoteConnection *ship.ShipConnection
}

// NewEndpoint configures a second full library instance for CS when simulator=true.
type EndpointOptions struct {
	// PracticeSpeed is constructor-only for the internal loopback pair.
	// It is rejected for real-device controllers.
	PracticeSpeed int
	DeviceMode    bool
	NoLPC         bool
	NoLPP         bool
	EffectDelay   time.Duration
	InitialPower  float64
}

func NewEndpoint(id identity.Identity, simulator bool) (*Endpoint, error) {
	return NewEndpointWithOptions(id, simulator, EndpointOptions{})
}

// Keep protocol metadata independent of the product title and transport setup.
func endpointConfiguration(id identity.Identity, simulator bool, port int, heartbeat ...time.Duration) (*api.Configuration, model.EntityTypeType, error) {
	typ, entityType := model.DeviceTypeTypeGeneric, model.EntityTypeTypeGridGuard
	name := "EnergyGuard"
	if simulator {
		entityType = model.EntityTypeTypeInverter
		name = "Simulator"
	}
	interval := HeartbeatInterval
	if len(heartbeat) > 0 {
		interval = heartbeat[0]
	}
	cfg, err := api.NewConfiguration("Stromea", "Stromea", name, id.SKI[:8], typ, []model.EntityTypeType{entityType}, port, id.Certificate, interval)
	return cfg, entityType, err
}

func NewEndpointWithOptions(id identity.Identity, simulator bool, opts EndpointOptions) (*Endpoint, error) {
	if opts.PracticeSpeed != 0 && opts.PracticeSpeed != 1 && opts.PracticeSpeed != 10 || opts.DeviceMode && opts.PracticeSpeed > 1 {
		return nil, errors.New("Zeitraffer ist nur für das interne Übungsgerät erlaubt")
	}
	stackLifecycle.Lock()
	locked := true
	defer func() {
		if locked {
			stackLifecycle.Unlock()
		}
	}()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	e := &Endpoint{Identity: id, listener: l, trusted: map[string]bool{}, closed: make(chan struct{}), deviceMode: opts.DeviceMode, options: opts, production: opts.NoLPC, status: "Geräte-SKI bestätigt; bereit zum Verbinden"}
	speed := 1
	if opts.PracticeSpeed == 10 {
		speed = 10
	}
	cfg, entityType, err := endpointConfiguration(id, simulator, l.Addr().(*net.TCPAddr).Port, HeartbeatInterval/time.Duration(speed))
	if err != nil {
		l.Close()
		return nil, err
	}
	e.practiceSpeed.Store(int32(speed))
	e.Service = service.NewService(cfg, e)
	if err = e.Service.Setup(); err != nil {
		l.Close()
		return nil, err
	}
	e.Entity = e.Service.LocalDevice().EntityForType(entityType)
	if !simulator {
		e.Entity.GetOrAddFeature(model.FeatureTypeTypeDeviceClassification, model.RoleTypeClient)
	}
	if simulator {
		if !opts.NoLPC {
			e.CSLPC = cslpc.NewLPC(e.Entity, e.simEvent)
			e.Service.AddUseCase(e.CSLPC)
			_ = e.CSLPC.SetFailsafeConsumptionActivePowerLimit(900, true)
			_ = e.CSLPC.SetFailsafeDurationMinimum(2*time.Hour, true)
			e.handlers = append(e.handlers, e.CSLPC, e.CSLPC.UseCaseBase)
		}
		if !opts.NoLPP {
			e.CSLPP = cslpp.NewLPP(e.Entity, e.simEvent)
			e.Service.AddUseCase(e.CSLPP)
			_ = e.CSLPP.SetFailsafeProductionActivePowerLimit(900, true)
			_ = e.CSLPP.SetFailsafeDurationMinimum(2*time.Hour, true)
			e.handlers = append(e.handlers, e.CSLPP, e.CSLPP.UseCaseBase)
		}
		if err = e.initMeasurement(); err != nil {
			locked = false
			stackLifecycle.Unlock()
			e.Close()
			return nil, err
		}
	} else {
		e.LPC = eglpc.NewLPC(e.Entity, nil)
		e.Service.AddUseCase(e.LPC)
		e.LPP = eglpp.NewLPP(e.Entity, nil)
		e.Service.AddUseCase(e.LPP)
		e.MPC = mpc.NewMPC(e.Entity, nil)
		// Replace MPC's own subscription while stackLifecycle excludes publishers.
		// Its measurement callbacks are unused; see mpcEventGuard.
		_ = spine.Events.Unsubscribe(e.MPC)
		mpcEvents := &mpcEventGuard{next: e.MPC}
		_ = spine.Events.Subscribe(mpcEvents)
		e.Service.AddUseCase(e.MPC)
		e.handlers = append(e.handlers, e.LPC, e.LPC.UseCaseBase, e.LPP, e.LPP.UseCaseBase, mpcEvents, e.MPC.UseCaseBase)
	}
	for i, handler := range e.handlers {
		_ = spine.Events.Unsubscribe(handler)
		guard := &remoteEvents{endpoint: e, next: handler}
		_ = spine.Events.Subscribe(guard)
		e.handlers[i] = guard
	}
	e.server = &http.Server{Handler: http.HandlerFunc(e.serveSHIP), ReadHeaderTimeout: 5 * time.Second, TLSConfig: &tls.Config{
		MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{id.Certificate}, ClientAuth: tls.RequireAnyClientCert,
		VerifyPeerCertificate: e.verifyTrusted,
	}}
	go func() { _ = e.server.Serve(tls.NewListener(l, e.server.TLSConfig)) }()
	if simulator {
		go e.simulate()
	}
	return e, nil
}

func (e *Endpoint) Address() string { return e.listener.Addr().String() }
func (e *Endpoint) Trust(ski string) error {
	s, err := identity.NormalizeSKI(ski)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.trusted[s] = true
	e.mu.Unlock()
	return nil
}
func (e *Endpoint) verifyTrusted(raw [][]byte, _ [][]*x509.Certificate) error {
	if len(raw) == 0 {
		return errors.New("Peer-Zertifikat fehlt")
	}
	leaf, err := x509.ParseCertificate(raw[0])
	if err != nil {
		return err
	}
	ski, err := cert.SkiFromCertificate(leaf)
	if err != nil {
		return err
	}
	// SHIP derives SKI from the uncompressed EC public key. Pin the key, not only
	// the caller-controlled SubjectKeyId field of a self-signed certificate.
	public, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("SHIP benötigt einen EC-Schlüssel")
	}
	key, err := public.ECDH()
	if err != nil {
		return err
	}
	digest := sha1.Sum(key.Bytes()) // SHA-1 is the mandated SHIP key identifier, not a signature algorithm.
	// 040 F-08: some stacks hash the whole SubjectPublicKeyInfo; that SKI is bound to the key as well.
	spki := sha1.Sum(leaf.RawSubjectPublicKeyInfo)
	if !bytes.Equal(digest[:], leaf.SubjectKeyId) && !bytes.Equal(spki[:], leaf.SubjectKeyId) {
		return errors.New("SKI passt nicht zum öffentlichen Schlüssel")
	}

	if err = leaf.CheckSignature(leaf.SignatureAlgorithm, leaf.RawTBSCertificate, leaf.Signature); err != nil {
		return err
	}
	// 040 F-08: SHIP trusts the confirmed SKI, not the validity period of a self-signed certificate
	// (device clocks without NTP, long storage). Outside the period is noted, never rejected.
	if now := time.Now(); now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		e.mu.Lock()
		trace := e.connectionTrace
		e.mu.Unlock()
		trace.note(fmt.Sprintf("Peer-Zertifikat außerhalb des Gültigkeitszeitraums (%s bis %s, Geräteuhr prüfen); vertraut wird der bestätigten SKI",
			leaf.NotBefore.UTC().Format("2006-01-02"), leaf.NotAfter.UTC().Format("2006-01-02")))
	}
	if !e.IsRemoteServiceForSKIPaired(ski) {
		return errors.New("Peer-SKI nicht freigegeben")
	}
	return nil
}
func (e *Endpoint) serveSHIP(w http.ResponseWriter, r *http.Request) {
	if err := LoopbackAddress(r.RemoteAddr); err != nil {
		http.Error(w, "Loopback erforderlich", 403)
		return
	}
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		http.Error(w, "Zertifikat fehlt", 403)
		return
	}
	ski, _ := cert.SkiFromCertificate(r.TLS.PeerCertificates[0])
	if !e.IsRemoteServiceForSKIPaired(ski) {
		http.Error(w, "SKI nicht freigegeben", 403)
		return
	}
	u := websocket.Upgrader{Subprotocols: []string{"ship"}, CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "" }}
	c, err := u.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	if c.Subprotocol() != "ship" {
		_ = c.Close()
		return
	}
	callbacks := &handshakeCallbacks{Endpoint: e}
	e.mu.Lock()
	h := ship.NewConnectionHandler(callbacks, e.trackTransport(c, ski), ship.ShipRoleServer, e.Identity.SKI, ski, "")
	callbacks.connection = h
	e.connections = append(e.connections, h)
	e.peer = ski
	e.alive = true
	e.handshakeState = shipmodel.CmiStateInitStart
	e.useCasesReceived = false
	e.status = "SHIP-Verbindung: gegenseitiges Vertrauen abwarten"
	e.mu.Unlock()
	h.Run()
}
func (e *Endpoint) Connect(ctx context.Context, addr, ski string) error {
	return e.ConnectOnRoute(ctx, addr, ski, DialRoute{})
}

func (e *Endpoint) ConnectOnRoute(ctx context.Context, addr, ski string, route DialRoute) error {
	check := LoopbackAddress
	if e.deviceMode {
		check = DeviceAddress
	}
	if err := check(addr); err != nil {
		return err
	}
	ski, err := identity.NormalizeSKI(ski)
	if err != nil {
		return err
	}
	if !e.IsRemoteServiceForSKIPaired(ski) {
		return errors.New("SKI zuerst lokal freigeben")
	}
	e.mu.Lock()
	e.tlsVerified = false
	trace := newConnectionTrace()
	e.connectionTrace = trace
	e.mu.Unlock()
	d := websocket.Dialer{HandshakeTimeout: ConnectionSetupTimeout, Subprotocols: []string{"ship"}, TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
		// SHIP has no CA: always present the own self-signed identity, even if the server's
		// CertificateRequest lists unrelated acceptable CAs (Go would otherwise send no certificate).
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			c := e.Identity.Certificate
			return &c, nil
		},
		// SHIP uses self-signed identities; verifyTrusted and exact SKI pin replace WebPKI.
		InsecureSkipVerify: true, VerifyPeerCertificate: func(raw [][]byte, chains [][]*x509.Certificate) error {
			return e.verifyExpected(raw, chains, ski)
		},
	}}
	d.NetDialContext = func(ctx context.Context, _ string, address string) (net.Conn, error) {
		return dialRoute(ctx, address, route, trace, func(ctx context.Context, dialer *net.Dialer, network, target string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, target)
		})
	}
	target := (&url.URL{Scheme: "wss", Host: addr, Path: "/ship/"}).String()
	c, resp, err := dialSHIPTraced(ctx, &d, target, trace)
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		c.Close()
		return err
	}
	trace.begin("SHIP-Handshake")
	callbacks := &handshakeCallbacks{Endpoint: e}
	e.mu.Lock()
	h := ship.NewConnectionHandler(callbacks, e.trackTransport(c, ski), ship.ShipRoleClient, e.Identity.SKI, ski, "")
	callbacks.connection = h
	e.connections = append(e.connections, h)
	e.peer = ski
	e.alive = true
	e.handshakeState = shipmodel.CmiStateInitStart
	e.useCasesReceived = false
	e.status = "SHIP-Verbindung: gegenseitiges Vertrauen abwarten"
	e.mu.Unlock()
	h.Run()
	return nil
}

// Only read our callback snapshot: ship-go v0.6.0's public state getter
// reads fields without the lock used by its Websocket reader.
func (e *Endpoint) handshakeComplete() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.alive && e.handshakeState == shipmodel.SmeStateComplete
}
func (e *Endpoint) Ready(ctx context.Context) (err error) {
	defer func() {
		e.mu.Lock()
		trace, complete := e.connectionTrace, e.handshakeState == shipmodel.SmeStateComplete
		e.mu.Unlock()
		if err == nil || complete {
			return
		}
		if trace != nil {
			err = trace.finish(err)
		}
	}()
	return Poll(ctx, func() bool {
		if e.deviceMode && !e.handshakeComplete() {
			return false
		}
		if !e.RemoteCurrent() {
			return false
		}
		entity := e.RemoteEntity()
		if entity == nil {
			return false
		}
		if e.deviceMode {
			e.mu.Lock()
			received := e.useCasesReceived
			e.mu.Unlock()
			if !received {
				return false
			}
			// Descriptions and values arrive after use-case discovery. Wait only for advertised cases.
			for _, kind := range []string{"LPC", "LPP", "MPC"} {
				r := e.EntityForUseCase(kind)
				if r == nil {
					continue
				}
				switch kind {
				case "LPC":
					if _, err := e.LPC.ConsumptionLimit(r); err != nil {
						return false
					}
					if e.LPC.IsScenarioAvailableAtEntity(r, 2) {
						if _, err := e.LPC.FailsafeConsumptionActivePowerLimit(r); err != nil {
							return false
						}
					}
				case "LPP":
					if _, err := e.LPP.ProductionLimit(r); err != nil {
						return false
					}
					if e.LPP.IsScenarioAvailableAtEntity(r, 2) {
						if _, err := e.LPP.FailsafeProductionActivePowerLimit(r); err != nil {
							return false
						}
					}
				case "MPC":
					if _, _, err := e.powerMetadata(r); err != nil {
						return false
					}
				}
			}
			return true
		}
		if e.MPC == nil {
			return true
		}
		// Practice discovery and measurement data can arrive before the load-
		// control values. Do not accept a connection that cannot yet set limits.
		if _, err := e.LPC.ConsumptionLimit(entity); err != nil {
			return false
		}
		if _, err := e.LPP.ProductionLimit(entity); err != nil {
			return false
		}
		_, err := e.MPC.Power(entity)
		return err == nil && e.LPC.IsScenarioAvailableAtEntity(entity, 1) && e.LPP.IsScenarioAvailableAtEntity(entity, 1) && e.MPC.IsScenarioAvailableAtEntity(entity, 1)
	})
}
func Poll(ctx context.Context, ready func() bool) error {
	t := time.NewTicker(25 * time.Millisecond)
	defer t.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ready() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}
func (e *Endpoint) RemoteEntity() spineapi.EntityRemoteInterface {
	e.spineMu.RLock()
	defer e.spineMu.RUnlock()
	e.mu.Lock()
	peer := e.peer
	e.mu.Unlock()
	d := e.Service.LocalDevice().RemoteDeviceForSki(peer)
	if d == nil {
		return nil
	}
	for _, entity := range d.Entities() {
		if entity.EntityType() != model.EntityTypeTypeDeviceInformation {
			return entity
		}
	}
	return nil
}
func (e *Endpoint) Close() {
	e.closeOnce.Do(func() {
		close(e.closed)
		if e.Entity != nil {
			e.Entity.HeartbeatManager().StopHeartbeat()
		}
		if e.server != nil {
			_ = e.server.Close()
		} else if e.listener != nil {
			_ = e.listener.Close()
		}
		e.mu.Lock()
		connections := append([]*ship.ShipConnection(nil), e.connections...)
		transports := append([]*connectionTransport(nil), e.transports...)
		e.mu.Unlock()
		for _, c := range connections {
			c.CloseConnection(false, 0, "Ende")
		}
		for _, t := range transports {
			t.closeSocket()
		}
		stackLifecycle.Lock()
		defer stackLifecycle.Unlock()
		for _, h := range e.handlers {
			_ = spine.Events.Unsubscribe(h)
		}
	})
}
func (e *Endpoint) IsRemoteServiceForSKIPaired(s string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.trusted[s]
}
func (e *Endpoint) IsAutoAcceptEnabled() bool        { return false }
func (e *Endpoint) AllowWaitingForTrust(string) bool { return e.deviceMode }
func (e *Endpoint) HandleConnectionClosed(c shipapi.ShipConnectionInterface, _ bool) {
	e.mu.Lock()
	if len(e.connections) > 0 && e.connections[len(e.connections)-1] == c {
		e.alive = false
		e.status = "Verbindung getrennt; Freigabe erfordert Wiederverbindung"
	}
	e.mu.Unlock()
}
func (e *Endpoint) ReportServiceShipID(string, string) {}

// Bind callbacks to their transport. After reconnect, an old reader/timer may
// still report a state for the same SKI; it must not change the new snapshot.
// The constructor starts the Websocket reader. Construct and assign connection
// under Endpoint.mu so its callbacks cannot read a partially registered handler.
type handshakeCallbacks struct {
	*Endpoint
	connection *ship.ShipConnection
}

func (c *handshakeCallbacks) SetupRemoteDevice(s string, w shipapi.ShipConnectionDataWriterInterface) shipapi.ShipConnectionDataReaderInterface {
	return c.Endpoint.setupRemoteDevice(s, w, c.connection)
}

func (c *handshakeCallbacks) HandleShipHandshakeStateUpdate(ski string, s shipmodel.ShipState) {
	e := c.Endpoint
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.connections) == 0 || e.connections[len(e.connections)-1] != c.connection {
		return
	}
	e.updateHandshakeLocked(ski, s)
}

func (e *Endpoint) HandleShipHandshakeStateUpdate(ski string, s shipmodel.ShipState) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.updateHandshakeLocked(ski, s)
}

func (e *Endpoint) updateHandshakeLocked(ski string, s shipmodel.ShipState) {
	if ski != e.peer || !e.alive {
		return
	}
	e.handshakeState = s.State
	switch s.State {
	case shipmodel.SmeHelloStatePendingListen, shipmodel.SmeHelloStatePendingInit:
		e.status = "Warte auf Vertrauen am Gerät: eigene SKI dort freigeben"
	case shipmodel.SmeHelloStateRejected, shipmodel.SmeHelloStateRemoteAbortDone, shipmodel.SmeHelloStateAbortDone:
		e.status = "Vertrauen abgelehnt oder Kopplung abgebrochen"
		e.connectionTrace.finish(errors.New(e.status))
	case shipmodel.SmeStateComplete:
		e.connectionTrace.finish(nil)
		e.status = "SHIP verbunden; SPINE-Geräteinformationen werden gelesen"
	}
	if s.Error != nil {
		e.status = "SHIP-Fehler: " + s.Error.Error()
		e.connectionTrace.finish(s.Error)
	}
}
func (e *Endpoint) SetupRemoteDevice(s string, w shipapi.ShipConnectionDataWriterInterface) shipapi.ShipConnectionDataReaderInterface {
	return e.setupRemoteDevice(s, w, nil)
}

func (e *Endpoint) setupRemoteDevice(s string, w shipapi.ShipConnectionDataWriterInterface, connection *ship.ShipConnection) shipapi.ShipConnectionDataReaderInterface {
	// Removal also changes upstream's global event registry and its unguarded
	// remote map. Exclude our publishers, lookups and queued application events.
	stackLifecycle.Lock()
	defer stackLifecycle.Unlock()
	e.spineMu.Lock()
	defer e.spineMu.Unlock()
	e.mu.Lock()
	if connection != nil && !e.currentConnectionLocked(connection) {
		e.mu.Unlock()
		return discardedReader{}
	}
	e.useCasesReceived = false
	e.remoteSetup = true
	e.remoteConnection = connection
	e.mu.Unlock()
	// A transport owns its SPINE bindings/subscriptions. SetupRemoteDevice alone
	// only replaces the remote map entry in spine-go v0.7.0. Remove the old device
	// first, including local feature caches and use-case removal notifications.
	local := e.Service.LocalDevice()
	if old := local.RemoteDeviceForSki(s); old != nil {
		// Upstream's device removal event has no Entity. Its UseCaseBase only
		// removes scenarios when given an entity; clear each old entity now,
		// before asynchronous discovery for the replacement can be handled.
		for _, entity := range old.Entities() {
			p := spineapi.EventPayload{Ski: s, Device: old, Entity: entity, EventType: spineapi.EventTypeEntityChange, ChangeType: spineapi.ElementChangeRemove}
			for _, h := range e.handlers {
				if g, ok := h.(*remoteEvents); ok {
					g.next.HandleEvent(p)
				}
			}
		}
		local.RemoveRemoteDeviceConnection(s)
	}
	e.heartbeatMu.Lock()
	e.heartbeatPending = nil
	e.heartbeatMu.Unlock()
	return measurementReader{endpoint: e, ski: s, connection: connection, next: local.SetupRemoteDevice(s, heartbeatWriter{endpoint: e, ski: s, next: w})}
}

// RemoteCurrent reports whether the SPINE remote device was set up by the live
// transport. After Disconnect the remote device and its cached data remain in
// spine-go; writes to it go to a closed transport and never get a reply.
func (e *Endpoint) RemoteCurrent() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.remoteSetup {
		return false
	}
	if e.remoteConnection == nil {
		return e.alive
	}
	return e.currentConnectionLocked(e.remoteConnection)
}

func (e *Endpoint) currentConnectionLocked(c *ship.ShipConnection) bool {
	return e.alive && len(e.connections) > 0 && e.connections[len(e.connections)-1] == c
}

type discardedReader struct{}

func (discardedReader) HandleShipPayloadMessage([]byte) {}

// Application events are queued asynchronously upstream. After replacement,
// an old discovery event must not reintroduce a binding to the previous device.
type remoteEvents struct {
	endpoint *Endpoint
	next     spineapi.EventHandlerInterface
}

func (g *remoteEvents) HandleEvent(p spineapi.EventPayload) {
	g.endpoint.spineMu.RLock()
	defer g.endpoint.spineMu.RUnlock()
	if p.Device != nil && g.endpoint.Service.LocalDevice().RemoteDeviceForSki(p.Ski) != p.Device {
		return
	}
	g.next.HandleEvent(p)
}
func (e *Endpoint) RemoteSKIConnected(api.ServiceInterface, string)                            {}
func (e *Endpoint) RemoteSKIDisconnected(api.ServiceInterface, string)                         {}
func (e *Endpoint) VisibleRemoteServicesUpdated(api.ServiceInterface, []shipapi.RemoteService) {}
func (e *Endpoint) ServiceShipIDUpdate(string, string)                                         {}
func (e *Endpoint) ServicePairingDetailUpdate(string, *shipapi.ConnectionStateDetail)          {}

// DeviceAddress accepts a single literal local-network destination; never DNS or a proxy.
func DeviceAddress(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ipText, zone, _ := strings.Cut(host, "%")
	ip := net.ParseIP(ipText)
	p, e := strconv.Atoi(port)
	if (zone != "" && (ip.To4() != nil || !ip.IsLinkLocalUnicast())) || ip == nil || e != nil || p < 1 || p > 65535 || ip.IsUnspecified() || ip.IsMulticast() || !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return errors.New("lokale Unicast-IP und gültiger Port erforderlich (kein DNS)")
	}
	return nil
}
func (e *Endpoint) ConnectionState() string { e.mu.Lock(); defer e.mu.Unlock(); return e.status }
func (e *Endpoint) Alive() bool             { e.mu.Lock(); defer e.mu.Unlock(); return e.alive }

// Disconnect only closes transport; the remote device keeps its limits until release/expiry.
func (e *Endpoint) Disconnect() {
	cs, transports := e.detachConnections()
	closeTransports(cs, transports)
}

func (e *Endpoint) EntityForUseCase(kind string) spineapi.EntityRemoteInterface {
	r := e.RemoteEntity()
	if r == nil {
		return nil
	}
	for _, entity := range r.Device().Entities() {
		var supported bool
		switch kind {
		case "LPC":
			supported = e.LPC.IsScenarioAvailableAtEntity(entity, 1)
		case "LPP":
			supported = e.LPP.IsScenarioAvailableAtEntity(entity, 1)
		case "MPC":
			supported = e.MPC.IsScenarioAvailableAtEntity(entity, 1)
		}
		if supported {
			return entity
		}
	}
	return nil
}
