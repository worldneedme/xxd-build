package config_test

import (
	"testing"

	"github.com/metacubex/mihomo/config"
	_ "github.com/metacubex/mihomo/hub/executor"
)

func TestParseHelperBackedProtocolConfig(t *testing.T) {
	const cfg = `
proxies:
  - name: naive-test
    type: naiveproxy
    server: example.com
    port: 443
    username: user
    password: pass
  - name: juicity-test
    type: juicity
    server: example.com
    port: 443
    uuid: 00000000-0000-0000-0000-000000000000
    password: pass
    sni: example.com
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - naive-test
      - juicity-test
      - DIRECT
rules:
  - MATCH,PROXY
`

	parsed, err := config.Parse([]byte(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Proxies["naive-test"].Type().String(); got != "NaiveProxy" {
		t.Fatalf("naive-test type = %q, want NaiveProxy", got)
	}
	if got := parsed.Proxies["juicity-test"].Type().String(); got != "Juicity" {
		t.Fatalf("juicity-test type = %q, want Juicity", got)
	}
}
