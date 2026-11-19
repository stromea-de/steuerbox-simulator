package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/enbility/eebus-go/features/server"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// This helper is a separate OS process. All SHIP/SPINE traffic crosses TLS/TCP;
// stdin/stdout only expose its address and independent post-test limit state.
func TestForeignDeviceProcess(t *testing.T) {
	if os.Getenv("EEBUS_006_FOREIGN") != "1" {
		return
	}
	id, err := identity.LoadOrCreate(os.Getenv("EEBUS_006_DIR"))
	if err != nil {
		t.Fatal(err)
	}
	opts := eebus.EndpointOptions{NoLPC: os.Getenv("EEBUS_013_NO_LPC") == "1", NoLPP: os.Getenv("EEBUS_006_NO_LPP") == "1"}
	if os.Getenv("EEBUS_006_SLOW") == "1" {
		opts.EffectDelay = 750 * time.Millisecond
	} else {
		opts.InitialPower = 100
	}
	ep, err := eebus.NewEndpointWithOptions(id, true, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer ep.Close()
	if os.Getenv("EEBUS_013_PRECISE") == "1" {
		dc, err := server.NewDeviceConfiguration(ep.Entity)
		if err != nil {
			t.Fatal(err)
		}
		err = dc.UpdateKeyValueDataForFilter(model.DeviceConfigurationKeyValueDataType{Value: &model.DeviceConfigurationKeyValueValueType{ScaledNumber: &model.ScaledNumberType{Number: util.Ptr(model.NumberType(900123456)), Scale: util.Ptr(model.ScaleType(-6))}}}, nil, model.DeviceConfigurationKeyValueDescriptionDataType{KeyName: util.Ptr(model.DeviceConfigurationKeyNameTypeFailsafeProductionActivePowerLimit)})
		if err != nil {
			t.Fatal(err)
		}
		err = dc.UpdateKeyValueDataForFilter(model.DeviceConfigurationKeyValueDataType{Value: &model.DeviceConfigurationKeyValueValueType{Duration: util.Ptr(model.DurationType("PT180M"))}}, nil, model.DeviceConfigurationKeyValueDescriptionDataType{KeyName: util.Ptr(model.DeviceConfigurationKeyNameTypeFailsafeDurationMinimum)})
		if err != nil {
			t.Fatal(err)
		}
	}
	if revision := os.Getenv("EEBUS_006_FIRMWARE"); revision != "" {
		entity := ep.Service.LocalDevice().EntityForType(model.EntityTypeTypeDeviceInformation)
		feature := entity.FeatureOfTypeAndRole(model.FeatureTypeTypeDeviceClassification, model.RoleTypeServer)
		data := feature.DataCopy(model.FunctionTypeDeviceClassificationManufacturerData).(*model.DeviceClassificationManufacturerDataType)
		data.SoftwareRevision = util.Ptr(model.DeviceClassificationStringType(revision))
		feature.SetData(model.FunctionTypeDeviceClassificationManufacturerData, data)
	}

	if os.Getenv("EEBUS_006_DENY") != "1" {
		if err = ep.Trust(os.Getenv("EEBUS_006_TRUST")); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := json.Marshal(discovery.Device{Name: "Fremdprozess", Address: ep.Address(), SKI: id.SKI})
	fmt.Println("DEVICE " + string(data))
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		switch scanner.Text() {
		case "drop":
			ep.Disconnect()
			fmt.Println("DROPPED")
		case "state":
			on := false
			if ep.CSLPC != nil {
				a, e := ep.CSLPC.ConsumptionLimit()
				on = e == nil && a.IsActive
			}
			b := false
			if ep.CSLPP != nil {
				v, err := ep.CSLPP.ProductionLimit()
				b = err == nil && v.IsActive
			}
			fmt.Printf("STATE %t %t\n", on, b)
		case "failsafe-raw":
			dc, err := server.NewDeviceConfiguration(ep.Entity)
			if err != nil {
				t.Fatal(err)
			}
			v, err := dc.GetKeyValueDataForFilter(model.DeviceConfigurationKeyValueDescriptionDataType{KeyName: util.Ptr(model.DeviceConfigurationKeyNameTypeFailsafeProductionActivePowerLimit)})
			if err != nil {
				t.Fatal(err)
			}
			duration, err := dc.GetKeyValueDataForFilter(model.DeviceConfigurationKeyValueDescriptionDataType{KeyName: util.Ptr(model.DeviceConfigurationKeyNameTypeFailsafeDurationMinimum)})
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf("RAW %d %d %s\n", *v.Value.ScaledNumber.Number, *v.Value.ScaledNumber.Scale, *duration.Value.Duration)
		case "lpp-value":
			v, err := ep.CSLPP.ProductionLimit()
			fmt.Printf("LPPVALUE %.3f %v\n", v.Value, err)
		case "failsafe":
			power, _, err := ep.CSLPP.FailsafeProductionActivePowerLimit()
			minimum, _, e2 := ep.CSLPP.FailsafeDurationMinimum()
			fmt.Printf("FAILSAFE %.3f %s %v %v\n", power, minimum, err, e2)
		case "exit":
			return
		}
	}
}

type foreign struct {
	cmd   *exec.Cmd
	in    io.WriteCloser
	lines chan string
}

func (f *foreign) line(t *testing.T, prefix string) string {
	t.Helper()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case s, ok := <-f.lines:
			if !ok {
				t.Fatal("Fremdprozess endete vor " + prefix)
			}
			if strings.HasPrefix(s, prefix) {
				return strings.TrimPrefix(s, prefix)
			}
			t.Log("Fremdprozess:", s)
		case <-timer.C:
			t.Fatal("Fremdprozess-Zeitlimit " + prefix)
		}
	}
}
func startForeign(t *testing.T, s *Session, flags ...string) (*foreign, discovery.Device) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestForeignDeviceProcess$")
	cmd.Env = append(os.Environ(), "EEBUS_006_FOREIGN=1", "EEBUS_006_DIR="+t.TempDir(), "EEBUS_006_TRUST="+s.EG.Identity.SKI)
	cmd.Env = append(cmd.Env, flags...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	f := &foreign{cmd: cmd, in: stdin, lines: make(chan string, 128)}
	go func() {
		defer close(f.lines)
		scan := bufio.NewScanner(stdout)
		for scan.Scan() {
			f.lines <- scan.Text()
		}
	}()
	t.Cleanup(func() {
		fmt.Fprintln(stdin, "exit")
		stdin.Close()
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Fremdprozess: %v", err)
			}
		case <-time.After(3 * time.Second):
			cmd.Process.Kill()
			<-done
		}
	})
	var d discovery.Device
	if err = json.Unmarshal([]byte(f.line(t, "DEVICE ")), &d); err != nil {
		t.Fatal(err)
	}
	return f, d
}
func foreignConfig() inspection.Config {
	c := inspection.DefaultDeviceConfig()
	c.LPP = 3000 // Explicit fixture, no universal device default.
	c.Business = "Prozesstest"
	c.Inspector = "Loopback"
	c.Authorized = true
	c.StepTimeout = 8 * time.Second
	c.EffectTimeout = 3 * time.Second
	c.CleanupTimeout = 8 * time.Second
	// 046: the foreign test device is the simulated device (2.5 s timeout), not a
	// real one; its scaled timeout replaces the 120 s protocol minimum.
	c.FailsafeTimeout = 3 * time.Second
	c.TestHeartbeatTimeout = c.FailsafeTimeout
	return c
}
func TestForeignTCPMissingLPPAndClassification(t *testing.T) {
	s, err := NewController(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, d := startForeign(t, s, "EEBUS_006_NO_LPP=1")
	d.Model = "Opta" // Selected model must survive SPINE instance metadata.
	if err = s.SelectDevice(d, true); err != nil {
		t.Fatal(err)
	}
	r, err := s.Run(context.Background(), foreignConfig())
	if err != nil {
		t.Fatal(err)
	}
	var lpp bool
	for _, v := range r.Steps {
		if strings.HasPrefix(v.Name, "LPP") && v.Status == inspection.NotChecked {
			lpp = true
		}
		if !v.Passed && v.Status != inspection.NotChecked {
			t.Fatal(v)
		}
	}
	if !lpp || r.Passed || r.Device.Serial != d.SKI[:8] || r.Device.Brand != "Stromea" || r.Device.Firmware != "" || r.Device.Model != "Opta" {
		t.Fatalf("missing capability/metadata: %+v", r)
	}
	fmt.Fprintln(f.in, "state")
	if got := f.line(t, "STATE "); got != "false false" {
		t.Fatal("Grenze aktiv:", got)
	}
}
func TestForeignTCPTrustDenied(t *testing.T) {
	s, err := NewController(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, d := startForeign(t, s, "EEBUS_006_DENY=1")
	if err = s.SelectDevice(d, true); err != nil {
		t.Fatal(err)
	}
	c := foreignConfig()
	c.StepTimeout = 500 * time.Millisecond
	r, err := s.Run(context.Background(), c)
	if err != nil || r.Passed || len(r.Steps) != 1 || r.Steps[0].Passed {
		t.Fatal(r, err)
	}
	fmt.Fprintln(f.in, "state")
	if got := f.line(t, "STATE "); got != "false false" {
		t.Fatal(got)
	}
}

type interruptedBackend struct {
	*Session
	foreign *foreign
	t       *testing.T
	cancel  context.CancelFunc
	drop    bool
	once    bool
}

func (b *interruptedBackend) Limit(ctx context.Context, k string, v float64, on bool) (string, error) {
	ack, err := b.Session.Limit(ctx, k, v, on)
	if on && err == nil && b.drop && !b.once {
		b.once = true
		fmt.Fprintln(b.foreign.in, "drop")
		b.foreign.line(b.t, "DROPPED")
		b.cancel()
		time.Sleep(100 * time.Millisecond)
	}
	return ack, err
}
func (b *interruptedBackend) Heartbeat(ctx context.Context, on bool) error {
	if !on && !b.drop {
		b.cancel()
	}
	return b.Session.Heartbeat(ctx, on)
}
func TestForeignTCPSlowEffectAndDisconnectRelease(t *testing.T) {
	for _, drop := range []bool{false, true} {
		t.Run(fmt.Sprint(drop), func(t *testing.T) {
			s, err := NewController(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			f, d := startForeign(t, s, "EEBUS_006_NO_LPP=1", "EEBUS_006_SLOW=1")
			if err = s.SelectDevice(d, true); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			s.limitDuration = foreignConfig().LimitLease()
			b := &interruptedBackend{Session: s, foreign: f, t: t, cancel: cancel, drop: drop}
			r, err := inspection.RunOnSite(ctx, foreignConfig(), inspection.Device{SKI: d.SKI}, b)
			if err != nil {
				t.Fatal(err)
			}
			if r.Passed {
				t.Fatal("Abbruch wurde als Erfolg gewertet")
			}
			if !drop {
				effect := false
				for _, v := range r.Steps {
					if v.Name == "LPC gemessene Wirkung" {
						effect = v.Passed
					}
				}
				if !effect {
					t.Fatal("Langsame Wirkung nicht abgewartet", r)
				}
			}
			last := r.Steps[len(r.Steps)-1]
			if last.Name != "Grenzen aufheben" || !last.Passed {
				t.Fatal("Freigabe nach Abbruch fehlt", last)
			}
			fmt.Fprintln(f.in, "state")
			if got := f.line(t, "STATE "); got != "false false" {
				t.Fatal("Grenze im Fremdprozess aktiv:", got)
			}
		})
	}
}

// Firmware is optional SPINE data. Preserve it when present; the no-firmware
// case above must remain empty rather than inventing a version from mDNS.
func TestForeignTCPFirmwareClassification(t *testing.T) {
	s, err := NewController(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, d := startForeign(t, s, "EEBUS_006_FIRMWARE=fixture-006b")
	if err = s.SelectDevice(d, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err = s.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	info := s.DeviceInfo()
	if info.Type != "Generic" || info.Brand != "Stromea" || info.Serial != d.SKI[:8] || info.Firmware != "fixture-006b" {
		t.Fatalf("classification lost: %+v", info)
	}
}
