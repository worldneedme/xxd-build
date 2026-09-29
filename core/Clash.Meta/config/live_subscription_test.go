package config_test

import (
	"os"
	"testing"

	"github.com/metacubex/mihomo/config"
	C "github.com/metacubex/mihomo/constant"
	_ "github.com/metacubex/mihomo/hub/executor"
)

func TestParseLiveSubscriptionFile(t *testing.T) {
	path := os.Getenv("FLCLASH_TEST_SUBSCRIPTION_FILE")
	if path == "" {
		t.Skip("live subscription file is not configured")
	}
	homeDir := os.Getenv("FLCLASH_TEST_HOME_DIR")
	if homeDir != "" {
		C.SetHomeDir(homeDir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := config.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed.Proxies["NaiveProxy-2"]; !ok {
		t.Fatal("NaiveProxy-2 was not parsed")
	}
	if got := parsed.Proxies["NaiveProxy-2"].Type().String(); got != "NaiveProxy" {
		t.Fatalf("NaiveProxy-2 type = %q, want NaiveProxy", got)
	}
}
