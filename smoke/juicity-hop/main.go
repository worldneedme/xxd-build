package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/metacubex/mihomo/config"
	_ "github.com/metacubex/mihomo/hub/executor"
	C "github.com/metacubex/mihomo/constant"
	"gopkg.in/yaml.v3"
)

const (
	newClientUA = "FlClash/v0.8.98 flclash-x juicity-hop/1 clash-verge Platform/windows"
	oldClientUA = "FlClash/v0.8.98 flclash-x"
	chunkSize   = 16384
)

type rawProxy struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Server      string `yaml:"server"`
	Port        int    `yaml:"port"`
	Hop         bool   `yaml:"hop"`
	HopPorts    string `yaml:"hop-ports"`
	HopInterval int    `yaml:"hop-interval"`
}

type rawConfig struct {
	Proxies []rawProxy `yaml:"proxies"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "subscription smoke failed:", err)
		os.Exit(1)
	}
}

func run() error {
	mode := os.Getenv("SMOKE_MODE")
	if mode != "baseline" && mode != "hop" {
		return errors.New("SMOKE_MODE must be baseline or hop")
	}

	geoDir, err := os.MkdirTemp("", "juicity-smoke-geo")
	if err != nil {
		return fmt.Errorf("could not create geo data directory: %w", err)
	}
	defer os.RemoveAll(geoDir)
	C.SetHomeDir(geoDir)

	basePort, err := requiredPort("BASE_PORT")
	if err != nil {
		return err
	}
	tcpTargetPort, err := requiredPort("TARGET_TCP_PORT")
	if err != nil {
		return err
	}
	udpTargetPort, err := requiredPort("TARGET_UDP_PORT")
	if err != nil {
		return err
	}
	captureIPv4 := net.ParseIP(os.Getenv("CAPTURE_SERVER_IPV4"))
	if captureIPv4 == nil || captureIPv4.To4() == nil {
		return errors.New("CAPTURE_SERVER_IPV4 must be an IPv4 address")
	}
	hopStart, hopEnd, err := parseHopRange(os.Getenv("HOP_PORTS"))
	if err != nil {
		return err
	}

	expectedHop := mode == "hop"
	userAgent := oldClientUA
	if expectedHop {
		userAgent = newClientUA
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
	defer cancel()
	body, err := fetchSubscription(ctx, userAgent)
	if err != nil {
		return err
	}
	proxy, raw, err := parseSubscriptionProxy(body, expectedHop, basePort, hopStart, hopEnd)
	if err != nil {
		return err
	}
	defer proxy.Close()
	if !resolvesToIPv4(raw.Server, captureIPv4.To4()) {
		return errors.New("the Juicity server in the subscription does not resolve to the configured capture IPv4")
	}

	chunks, queries := 8, 4
	if expectedHop {
		chunks, queries = 80, 40
	}
	started := float64(time.Now().UnixNano()) / 1e9
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	var tcpBytes int
	var udpOK int
	var maxRTT time.Duration
	wg.Add(2)
	go func() {
		defer wg.Done()
		conn, dialErr := proxy.DialContext(ctx, &C.Metadata{Host: "127.0.0.1", DstPort: uint16(tcpTargetPort), NetWork: C.TCP})
		if dialErr != nil {
			failures <- fmt.Errorf("TCP dial failed: %w", dialErr)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(75 * time.Second))
		if _, writeErr := fmt.Fprintf(conn, "%d\n", chunks); writeErr != nil {
			failures <- fmt.Errorf("TCP request failed: %w", writeErr)
			return
		}
		buffer := make([]byte, chunkSize)
		for index := 0; index < chunks; index++ {
			if _, readErr := io.ReadFull(conn, buffer); readErr != nil {
				failures <- fmt.Errorf("TCP response failed at chunk %d: %w", index, readErr)
				return
			}
			for offset, value := range buffer {
				if value != byte((index+offset)%251) {
					failures <- fmt.Errorf("TCP payload mismatch at chunk %d", index)
					return
				}
			}
			tcpBytes += len(buffer)
		}
		extra := make([]byte, 1)
		count, readErr := conn.Read(extra)
		if count != 0 || readErr != io.EOF {
			failures <- fmt.Errorf("TCP response had an unexpected tail (bytes=%d)", count)
		}
	}()
	go func() {
		defer wg.Done()
		packetConn, dialErr := proxy.ListenPacketContext(ctx, &C.Metadata{Host: "127.0.0.1", DstPort: uint16(udpTargetPort), NetWork: C.UDP})
		if dialErr != nil {
			failures <- fmt.Errorf("UDP dial failed: %w", dialErr)
			return
		}
		defer packetConn.Close()
		target := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: udpTargetPort}
		buffer := make([]byte, 2048)
		for index := 0; index < queries; index++ {
			payload := []byte(fmt.Sprintf("juicity-subscription-echo-%03d", index))
			startedAt := time.Now()
			_ = packetConn.SetDeadline(startedAt.Add(3 * time.Second))
			if _, writeErr := packetConn.WriteTo(payload, target); writeErr != nil {
				failures <- fmt.Errorf("UDP request failed at packet %d: %w", index, writeErr)
				return
			}
			count, _, readErr := packetConn.ReadFrom(buffer)
			if readErr != nil {
				failures <- fmt.Errorf("UDP response failed at packet %d: %w", index, readErr)
				return
			}
			if !bytes.Equal(buffer[:count], payload) {
				failures <- fmt.Errorf("UDP payload mismatch at packet %d", index)
				return
			}
			if rtt := time.Since(startedAt); rtt > maxRTT {
				maxRTT = rtt
			}
			udpOK++
			if wait := time.Until(startedAt.Add(time.Second)); wait > 0 {
				time.Sleep(wait)
			}
		}
	}()
	wg.Wait()
	close(failures)
	var collected []error
	for failure := range failures {
		if failure != nil {
			collected = append(collected, failure)
		}
	}
	finished := float64(time.Now().UnixNano()) / 1e9
	result := map[string]any{
		"mode":           mode,
		"round":          os.Getenv("SMOKE_ROUND"),
		"start":          started,
		"end":            finished,
		"tcp_bytes":      tcpBytes,
		"udp_echo_ok":    udpOK,
		"udp_max_rtt_ms": maxRTT.Milliseconds(),
		"hop_enabled":    expectedHop,
		"base_port":      basePort,
		"effective_port": proxyPort(proxy),
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return errors.New("could not write sanitized smoke result")
	}
	if len(collected) > 0 {
		return errors.Join(collected...)
	}
	return nil
}

func requiredPort(key string) (int, error) {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 || value > 65535 {
		return 0, fmt.Errorf("%s must be a TCP/UDP port in 1..65535", key)
	}
	return value, nil
}

func parseHopRange(value string) (int, int, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, errors.New("HOP_PORTS must be a numeric start-end range")
	}
	start, startErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, endErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if startErr != nil || endErr != nil || start < 1 || end > 65535 || start > end {
		return 0, 0, errors.New("HOP_PORTS must be a valid numeric start-end range")
	}
	if end-start+1 < 4 {
		return 0, 0, errors.New("HOP_PORTS must include at least four ports for rotation evidence")
	}
	return start, end, nil
}

func fetchSubscription(ctx context.Context, userAgent string) ([]byte, error) {
	subscriptionURL := os.Getenv("HKDEV_TEST_SUBSCRIPTION_URL")
	if subscriptionURL == "" {
		return nil, errors.New("the isolated subscription URL secret is not configured")
	}
	parsedURL, err := url.Parse(subscriptionURL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return nil, errors.New("the isolated subscription URL must be a valid HTTPS URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, subscriptionURL, nil)
	if err != nil {
		return nil, errors.New("the isolated subscription URL could not be parsed")
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "*/*")
	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(next *http.Request, previous []*http.Request) error {
			if len(previous) >= 3 || next.URL.Scheme != "https" || !strings.EqualFold(next.URL.Host, parsedURL.Host) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("the isolated subscription request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the isolated subscription returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil || len(body) == 0 || len(body) >= 16<<20 {
		return nil, errors.New("the isolated subscription response was empty, unreadable or oversized")
	}
	return body, nil
}

func parseSubscriptionProxy(body []byte, expectHop bool, basePort, hopStart, hopEnd int) (C.Proxy, rawProxy, error) {
	if !bytes.Contains(body, []byte("geox-url")) {
		body = append([]byte("geox-url:
  mmdb: \"https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb\"
"), body...)
	}
	var shape rawConfig
	if err := yaml.Unmarshal(body, &shape); err != nil {
		return nil, rawProxy{}, errors.New("the real subscription is not valid Clash YAML")
	}
	parsed, err := config.Parse(body)
	if err != nil {
		return nil, rawProxy{}, errors.New("the real subscription failed the core config parser")
	}
	for _, raw := range shape.Proxies {
		if !strings.EqualFold(raw.Type, "juicity") {
			continue
		}
		proxy, ok := parsed.Proxies[raw.Name]
		if !ok || !strings.EqualFold(proxy.Type().String(), "Juicity") {
			return nil, rawProxy{}, errors.New("the core config parser did not construct the exported Juicity proxy")
		}
		if raw.Port != basePort {
			return nil, rawProxy{}, errors.New("the subscription Juicity base port does not match the isolated test service")
		}
		if expectHop {
			if !raw.Hop || raw.HopPorts == "" || raw.HopInterval != 5 {
				return nil, rawProxy{}, errors.New("the new-client export is missing explicit hop capability parameters")
			}
			start, end, rangeErr := parseHopRange(raw.HopPorts)
			if rangeErr != nil || start != hopStart || end != hopEnd {
				return nil, rawProxy{}, errors.New("the exported hop range does not match the isolated test range")
			}
			wantAddr := net.JoinHostPort(raw.Server, strconv.Itoa(hopStart))
			if proxy.Addr() != wantAddr {
				return nil, rawProxy{}, errors.New("the core adapter did not select the first configured hop port")
			}
		} else {
			if raw.Hop || raw.HopPorts != "" || raw.HopInterval != 0 {
				return nil, rawProxy{}, errors.New("the fixed-port export unexpectedly contains hop parameters")
			}
			wantAddr := net.JoinHostPort(raw.Server, strconv.Itoa(basePort))
			if proxy.Addr() != wantAddr {
				return nil, rawProxy{}, errors.New("the core adapter did not keep the fixed base port")
			}
		}
		return proxy, raw, nil
	}
	return nil, rawProxy{}, errors.New("the real subscription contains no Juicity proxy")
}

func proxyPort(proxy C.Proxy) int {
	_, portText, err := net.SplitHostPort(proxy.Addr())
	if err != nil {
		return 0
	}
	port, _ := strconv.Atoi(portText)
	return port
}

func resolvesToIPv4(host string, expected net.IP) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.To4() != nil && ip.To4().Equal(expected)
	}
	addresses, err := net.LookupIP(host)
	if err != nil {
		return false
	}
	for _, address := range addresses {
		if address.To4() != nil && address.To4().Equal(expected) {
			return true
		}
	}
	return false
}
