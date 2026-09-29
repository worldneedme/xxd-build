package adapter

import "testing"

func TestParseHelperBackedProtocols(t *testing.T) {
	tests := []struct {
		name        string
		mapping     map[string]any
		adapterType string
	}{
		{
			name: "naive alias",
			mapping: map[string]any{
				"name":     "naive-alias",
				"type":     "naive",
				"server":   "example.com",
				"port":     443,
				"username": "user",
				"password": "pass",
			},
			adapterType: "NaiveProxy",
		},
		{
			name: "naiveproxy",
			mapping: map[string]any{
				"name":   "naiveproxy",
				"type":   "naiveproxy",
				"proxy":  "https://user:pass@example.com:443",
				"server": "example.com",
				"port":   443,
			},
			adapterType: "NaiveProxy",
		},
		{
			name: "juicity",
			mapping: map[string]any{
				"name":     "juicity",
				"type":     "juicity",
				"server":   "example.com",
				"port":     443,
				"uuid":     "00000000-0000-0000-0000-000000000000",
				"password": "pass",
				"sni":      "example.com",
			},
			adapterType: "Juicity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := ParseProxy(tt.mapping)
			if err != nil {
				t.Fatal(err)
			}
			if got := proxy.Type().String(); got != tt.adapterType {
				t.Fatalf("adapter type = %q, want %q", got, tt.adapterType)
			}
		})
	}
}
