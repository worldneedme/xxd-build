package outbound

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNaiveProxyPasswordOnlyAndStablePort(t *testing.T) {
	proxyURL, err := buildNaiveProxyURL(NaiveProxyOption{
		Server:   "example.com",
		Port:     443,
		Password: "token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL != "https://:token@example.com:443" {
		t.Fatalf("proxy URL = %q", proxyURL)
	}

	first, err := chooseLocalPort("NaiveProxy-2", 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := chooseLocalPort("NaiveProxy-2", 0)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("stable port changed: %d != %d", first, second)
	}
}

func TestNaiveProxyH3AlpnUsesQuicScheme(t *testing.T) {
	proxyURL, err := buildNaiveProxyURL(NaiveProxyOption{
		Server: "example.com",
		Port:   443,
		ALPN:   []string{"h3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL != "quic://example.com:443" {
		t.Fatalf("proxy URL = %q", proxyURL)
	}
}

func TestCloseAllHelpersKeepsProxyRegistered(t *testing.T) {
	p := &helperBackedProxy{}

	helpersMu.Lock()
	previousHelpers := allHelpers
	allHelpers = make(map[*helperBackedProxy]struct{})
	helpersMu.Unlock()
	defer func() {
		helpersMu.Lock()
		allHelpers = previousHelpers
		helpersMu.Unlock()
	}()

	RegisterHelper(p)
	CloseAllHelpers()

	helpersMu.Lock()
	_, registeredAfterStop := allHelpers[p]
	helpersMu.Unlock()
	if !registeredAfterStop {
		t.Fatal("CloseAllHelpers unregistered an active helper proxy")
	}

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	helpersMu.Lock()
	_, registeredAfterClose := allHelpers[p]
	helpersMu.Unlock()
	if registeredAfterClose {
		t.Fatal("Close did not unregister helper proxy")
	}
}

func TestHelperBackedProxyStartsOfficialHelpers(t *testing.T) {
	naiveHelper := os.Getenv("FLCLASH_TEST_NAIVE_HELPER")
	if naiveHelper == "" {
		t.Skip("naive helper path is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	naiveProxy, err := NewNaiveProxy(NaiveProxyOption{
		Name:       "naive-helper-smoke",
		Server:     "example.com",
		Port:       443,
		UserName:   "user",
		Password:   "pass",
		HelperPath: naiveHelper,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer naiveProxy.Close()
	if err = naiveProxy.ensureStarted(ctx); err != nil {
		t.Fatalf("start naive helper: %v", err)
	}
}

func TestJuicityNativeConstruction(t *testing.T) {
	juicityProxy, err := NewJuicity(JuicityOption{
		Name:     "juicity-native-smoke",
		Server:   "example.com",
		Port:     443,
		UUID:     "00000000-0000-0000-0000-000000000000",
		Password: "pass",
		SNI:      "example.com",
		UDP:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer juicityProxy.Close()
	if juicityProxy.Type().String() != "Juicity" {
		t.Fatalf("adapter type = %s, want Juicity", juicityProxy.Type().String())
	}
	if juicityProxy.dialer == nil {
		t.Fatal("native juicity dialer was not initialized")
	}
}

func TestJuicityNativeUsesFirstHopPort(t *testing.T) {
	juicityProxy, err := NewJuicity(JuicityOption{
		Name:     "juicity-native-hop",
		Server:   "example.com",
		Port:     443,
		UUID:     "00000000-0000-0000-0000-000000000000",
		Password: "pass",
		SNI:      "example.com",
		HopPorts: "20000-30000",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer juicityProxy.Close()
	if juicityProxy.Addr() != "example.com:20000" {
		t.Fatalf("juicity addr = %q, want example.com:20000", juicityProxy.Addr())
	}
}

func TestJuicityNativeAcceptsCongestionControllerAlias(t *testing.T) {
	juicityProxy, err := NewJuicity(JuicityOption{
		Name:                 "juicity-native-cc-alias",
		Server:               "example.com",
		Port:                 443,
		UUID:                 "00000000-0000-0000-0000-000000000000",
		Password:             "pass",
		SNI:                  "example.com",
		CongestionController: "cubic",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer juicityProxy.Close()
	if juicityProxy.option.CongestionControl != "cubic" {
		t.Fatalf("congestion control = %q, want cubic", juicityProxy.option.CongestionControl)
	}
}
