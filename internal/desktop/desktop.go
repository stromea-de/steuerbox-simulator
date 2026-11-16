// Package desktop owns the local desktop lifecycle. It never discovers LAN peers.
package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const Address = "127.0.0.1:33287"
const Instance = "stromea-steuerbox-simulator/desktop-v1"

// Claim binds before loading identities or starting a simulation. A second launch
// only probes the loopback application marker, never another service's content.
func Claim(addr string) (net.Listener, bool, error) {
	l, err := net.Listen("tcp", addr)
	if err == nil {
		return l, false, nil
	}
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Get("http://" + addr + "/api/instance")
	if e == nil {
		defer response.Body.Close()
		var marker struct {
			Application string `json:"application"`
		}
		if response.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&marker) == nil && marker.Application == Instance {
			return nil, true, nil
		}
	}
	return nil, false, fmt.Errorf("Die lokale Adresse %s ist belegt oder nicht verfügbar. Beende die andere Anwendung oder starte den Simulator mit serve --listen 127.0.0.1:33288. Details: %w", addr, err)
}
func Open(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Run()
}
func openLog() (*os.File, string, error) {
	base, e := os.UserConfigDir()
	if e != nil {
		return nil, "", e
	}
	dir := filepath.Join(base, "stromea-steuerbox-simulator", "logs")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return nil, "", e
	}
	path := filepath.Join(dir, "simulator.log")
	if info, e := os.Stat(path); e == nil && info.Size() > 5<<20 {
		if e = os.Rename(path, path+".previous"); e != nil {
			return nil, dir, e
		}
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	return f, dir, e
}

// Run's serve callback must return only after cancellation and cleanup complete.
func Run(ctx context.Context, version string, serve func(context.Context, net.Listener, io.Writer) int) int {
	l, existing, e := Claim(Address)
	if e != nil {
		Alert(e.Error())
		return 1
	}
	url := "http://" + Address + "/?launch=1"
	if existing {
		if e = Open(url); e != nil {
			Alert("Öffne " + url + " im Browser. " + e.Error())
		}
		return 0
	}
	defer l.Close()
	f, dir, e := openLog()
	if e != nil {
		Alert("Protokolldatei konnte nicht geöffnet werden: " + e.Error())
		return 1
	}
	defer f.Close()
	log.SetOutput(f)
	os.Stdout = f
	os.Stderr = f
	fmt.Fprintln(f, "Stromea Steuerbox-Simulator", version)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan int, 1)
	ready := make(chan struct{})
	go func() { result <- serve(runCtx, l, f); close(ready) }()
	open := func(target string) {
		if e := Open(target); e != nil {
			fmt.Fprintln(f, e)
			Alert("Öffnen fehlgeschlagen: " + target)
		}
	}
	menu(runCtx, version, func() { open(url) }, func() { open(dir) }, cancel, ready)
	code := <-result
	if code != 0 {
		Alert("Der Simulator konnte nicht starten. Details stehen in " + dir)
	}
	return code
}

func exitMessage(busy bool) string {
	message := "Simulator beenden?\n\nDas Schließen des Browserfensters beendet den Simulator nicht. Er läuft über das Symbol im Infobereich weiter."
	if busy {
		message += "\n\nEine Prüfung oder Verbindung ist aktiv. Sie wird abgebrochen; Testgrenzen werden aufgehoben und Notbetriebs-Werte zurückgestellt. Bitte den Abschluss abwarten und unbestätigte Freigaben am Gerät prüfen."
	}
	return message
}
func activeInstance() bool {
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r, e := client.Get("http://" + Address + "/api/instance")
	if e != nil {
		return true
	}
	defer r.Body.Close()
	var marker struct {
		Busy bool `json:"busy"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&marker) != nil {
		return true
	}
	return marker.Busy
}
