package outbound

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	C "github.com/metacubex/mihomo/constant"
)

var (
	helpersMu     sync.Mutex
	helperStartMu sync.Map
	allHelpers    = make(map[*helperBackedProxy]struct{})
	// helperPIDs tracks OS PIDs of live helper processes for orphan cleanup.
	helperPIDs = make(map[int]*helperBackedProxy)
)

func RegisterHelper(p *helperBackedProxy) {
	helpersMu.Lock()
	defer helpersMu.Unlock()
	allHelpers[p] = struct{}{}
}

func UnregisterHelper(p *helperBackedProxy) {
	if p == nil {
		return
	}
	p.mu.Lock()
	pid := p.helperPID
	p.helperPID = 0
	p.mu.Unlock()

	helpersMu.Lock()
	delete(allHelpers, p)
	if pid > 0 {
		delete(helperPIDs, pid)
	}
	helpersMu.Unlock()
}

func CloseAllHelpers() {
	helpersMu.Lock()
	helpers := make([]*helperBackedProxy, 0, len(allHelpers))
	for p := range allHelpers {
		helpers = append(helpers, p)
	}
	allHelpers = make(map[*helperBackedProxy]struct{})
	// snapshot tracked pids then clear map ownership
	pids := make([]int, 0, len(helperPIDs))
	for pid := range helperPIDs {
		pids = append(pids, pid)
	}
	helperPIDs = make(map[int]*helperBackedProxy)
	helpersMu.Unlock()

	for _, p := range helpers {
		p.forceStop()
	}
	for _, pid := range pids {
		forceKillPID(pid)
	}
	// final sweep for any leftover helper children
	killOrphanHelperProcesses(nil)
}

// helperIdleTTL is intentionally short so delay-test helpers do not linger.
const helperIdleTTL = 30 * time.Second

func init() {
	go helperIdleReaperLoop()
}

func helperIdleReaperLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		func() {
			defer func() { _ = recover() }()
			reapIdleHelpers()
			killOrphanHelperProcesses(activeHelperPIDs())
		}()
	}
}

func activeHelperPIDs() map[int]struct{} {
	helpersMu.Lock()
	defer helpersMu.Unlock()
	out := make(map[int]struct{}, len(helperPIDs))
	for pid := range helperPIDs {
		out[pid] = struct{}{}
	}
	return out
}

func reapIdleHelpers() {
	now := time.Now()
	helpersMu.Lock()
	var stale []*helperBackedProxy
	for p := range allHelpers {
		p.mu.Lock()
		last := p.lastUsed
		pid := p.helperPID
		running := pid > 0 || (p.cmd != nil && p.cmd.Process != nil && p.waitErr == nil)
		p.mu.Unlock()
		// Also treat never-used-but-started helpers as reclaimable after TTL
		// once lastUsed was stamped at process start.
		if running {
			// Prefer lastUsed; if somehow unset, fall back to treating as stale.
			idleFor := time.Duration(0)
			if !last.IsZero() {
				idleFor = now.Sub(last)
			} else {
				idleFor = helperIdleTTL + time.Second
			}
			if idleFor > helperIdleTTL {
				stale = append(stale, p)
			}
		}
	}
	helpersMu.Unlock()
	for _, p := range stale {
		p.forceStop()
		UnregisterHelper(p)
	}
}

func forceKillPID(pid int) {
	if pid <= 0 {
		return
	}
	// Cross-platform hard kill via os.Process.Kill (works on Unix and Windows).
	proc, err := os.FindProcess(pid)
	if err == nil && proc != nil {
		_ = proc.Kill()
	}
}


// killOrphanHelperProcesses kills helper binaries that are children of this
// process but are not tracked as active helper PIDs.
func killOrphanHelperProcesses(keep map[int]struct{}) {
	if keep == nil {
		keep = map[int]struct{}{}
	}
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(ent.Name())
		if err != nil || pid <= 1 || pid == self {
			continue
		}
		if _, ok := keep[pid]; ok {
			continue
		}
		// parent must be us
		statBytes, err := os.ReadFile("/proc/" + ent.Name() + "/stat")
		if err != nil {
			continue
		}
		stat := string(statBytes)
		// format: pid (comm) state ppid ...
		rparen := strings.LastIndex(stat, ")")
		if rparen < 0 || rparen+2 >= len(stat) {
			continue
		}
		fields := strings.Fields(stat[rparen+2:])
		if len(fields) < 2 {
			continue
		}
		ppid, _ := strconv.Atoi(fields[1])
		if ppid != self {
			continue
		}
		cmdBytes, err := os.ReadFile("/proc/" + ent.Name() + "/cmdline")
		if err != nil {
			continue
		}
		cmdline := strings.ReplaceAll(string(cmdBytes), "\x00", " ")
		base := filepath.Base(strings.Fields(cmdline + " ")[0])
		name := strings.ToLower(base)
		if strings.Contains(name, "libnaive") || name == "naive" ||
			strings.Contains(name, "juicity") || strings.Contains(cmdline, "libnaive.so") ||
			strings.Contains(cmdline, "juicity") {
			forceKillPID(pid)
		}
	}
}

type helperCommandBuilder func(port int, runtimeDir string) (string, []string, error)

type helperBackedProxy struct {
	*Base
	socks          *Socks5
	dialerProxy    string
	localPort      int
	runtimeDir     string
	commandBuilder helperCommandBuilder

	mu       sync.Mutex
	cmd      *exec.Cmd
	waitDone chan struct{}
	waitErr  error
	lastUsed time.Time
	helperPID int
}

type NaiveProxyOption struct {
	BasicOption
	Name                string   `proxy:"name"`
	Server              string   `proxy:"server"`
	Port                int      `proxy:"port"`
	User                string   `proxy:"user,omitempty"`
	UserName            string   `proxy:"username,omitempty"`
	Password            string   `proxy:"password,omitempty"`
	Scheme              string   `proxy:"scheme,omitempty"`
	Proxy               string   `proxy:"proxy,omitempty"`
	ExtraHeaders        string   `proxy:"extra-headers,omitempty"`
	HostResolverRules   string   `proxy:"host-resolver-rules,omitempty"`
	InsecureConcurrency int      `proxy:"insecure-concurrency,omitempty"`
	TunnelTimeout       int      `proxy:"tunnel-timeout,omitempty"`
	IdleTimeout         int      `proxy:"idle-timeout,omitempty"`
	ALPN                []string `proxy:"alpn,omitempty"`
	LocalPort           int      `proxy:"local-port,omitempty"`
	HelperPath          string   `proxy:"helper-path,omitempty"`
}

func NewNaiveProxy(option NaiveProxyOption) (*helperBackedProxy, error) {
	if option.Name == "" {
		return nil, errors.New("naiveproxy missing name")
	}
	if option.Proxy == "" && (option.Server == "" || option.Port == 0) {
		return nil, errors.New("naiveproxy requires proxy or server/port")
	}
	proxyURL, err := buildNaiveProxyURL(option)
	if err != nil {
		return nil, err
	}
	seed := helperSeed(
		option.Name,
		proxyURL,
		option.ExtraHeaders,
		option.HostResolverRules,
		strconv.Itoa(option.InsecureConcurrency),
		strconv.Itoa(option.TunnelTimeout),
		strconv.Itoa(option.IdleTimeout),
	)
	port, err := chooseLocalPort(seed, option.LocalPort)
	if err != nil {
		return nil, err
	}
	runtimeDir, err := makeHelperRuntimeDir(option.Name, seed)
	if err != nil {
		return nil, err
	}
	builder := func(port int, runtimeDir string) (string, []string, error) {
		exe, err := resolveHelperPath(option.HelperPath, helperBinaryName("naive"))
		if err != nil {
			return "", nil, err
		}
		args := []string{
			"--listen=socks://127.0.0.1:" + strconv.Itoa(port),
			"--proxy=" + proxyURL,
			"--log=" + filepath.Join(runtimeDir, "naive.log"),
		}
		if option.ExtraHeaders != "" {
			args = append(args, "--extra-headers="+option.ExtraHeaders)
		}
		if option.HostResolverRules != "" {
			args = append(args, "--host-resolver-rules="+option.HostResolverRules)
		}
		if option.InsecureConcurrency > 0 {
			args = append(args, "--insecure-concurrency="+strconv.Itoa(option.InsecureConcurrency))
		}
		if option.TunnelTimeout > 0 {
			args = append(args, "--tunnel-timeout="+strconv.Itoa(option.TunnelTimeout))
		}
		if option.IdleTimeout > 0 {
			args = append(args, "--idle-timeout="+strconv.Itoa(option.IdleTimeout))
		}
		return exe, args, nil
	}
	return newHelperBackedProxy(option.Name, option.Server, option.Port, C.NaiveProxy, false, option.BasicOption, port, runtimeDir, builder)
}

func newHelperBackedProxy(name, server string, remotePort int, adapterType C.AdapterType, udp bool, basic BasicOption, localPort int, runtimeDir string, builder helperCommandBuilder) (*helperBackedProxy, error) {
	socks, err := NewSocks5(Socks5Option{
		Name:   name + "-local-helper",
		Server: "127.0.0.1",
		Port:   localPort,
		UDP:    udp,
	})
	if err != nil {
		return nil, err
	}
	p := &helperBackedProxy{
		Base: NewBase(BaseOption{
			Name:         name,
			Addr:         net.JoinHostPort(server, strconv.Itoa(remotePort)),
			Type:         adapterType,
			ProviderName: basic.ProviderName,
			UDP:          udp,
			TFO:          basic.TFO,
			MPTCP:        basic.MPTCP,
			Interface:    basic.Interface,
			RoutingMark:  basic.RoutingMark,
			Prefer:       basic.IPVersion,
		}),
		socks:          socks,
		dialerProxy:    basic.DialerProxy,
		localPort:      localPort,
		runtimeDir:     runtimeDir,
		commandBuilder: builder,
	}
	RegisterHelper(p)
	return p, nil
}

func (p *helperBackedProxy) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	if err := p.ensureStarted(ctx); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.lastUsed = time.Now()
	p.mu.Unlock()
	return p.socks.DialContext(ctx, metadata)
}

func (p *helperBackedProxy) ListenPacketContext(ctx context.Context, metadata *C.Metadata) (C.PacketConn, error) {
	if !p.SupportUDP() {
		return nil, C.ErrNotSupport
	}
	if err := p.ensureStarted(ctx); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.lastUsed = time.Now()
	p.mu.Unlock()
	return p.socks.ListenPacketContext(ctx, metadata)
}

func (p *helperBackedProxy) ProxyInfo() C.ProxyInfo {
	info := p.Base.ProxyInfo()
	info.DialerProxy = p.dialerProxy
	return info
}

func (p *helperBackedProxy) Close() error {
	p.forceStop()
	UnregisterHelper(p)
	return nil
}

func (p *helperBackedProxy) stopHelper() {
	p.forceStop()
}

func (p *helperBackedProxy) forceStop() {
	p.mu.Lock()
	pid := p.helperPID
	done := p.stopCommandLocked()
	p.helperPID = 0
	p.mu.Unlock()
	waitForHelperCommand(done)
	if pid > 0 {
		forceKillPID(pid)
		helpersMu.Lock()
		delete(helperPIDs, pid)
		helpersMu.Unlock()
	}
}

func (p *helperBackedProxy) ensureStarted(ctx context.Context) error {
	if p.hasRunningCommand() {
		if err := p.waitUntilReady(ctx, 1*time.Millisecond); err == nil {
			return nil
		}
	}
	unlockStart := lockHelperStart(p.helperKey())
	defer unlockStart()
	if p.hasRunningCommand() {
		return nil
	}
	if err := probeLocalPort(p.localPort); err == nil {
		// Port is already bound. If we own a live command it is our healthy
		// helper and there is nothing to do. Otherwise a stale/orphan helper
		// holds the port: kill it and fall through to (re)start. We must never
		// return success while the port is still held by a helper we do not
		// control -- doing so hands traffic to a dead tunnel whose local SOCKS
		// port is still open, and every request silently times out
		// (ERR_TUNNEL_CONNECTION_FAILED). Falling through surfaces a real bind
		// error instead, so the caller can retry.
		if p.hasRunningCommand() {
			return nil
		}
		killListenersOnPort(p.localPort)
	}
	p.mu.Lock()
	if p.commandRunningLocked() {
		p.mu.Unlock()
		if err := p.waitUntilReady(ctx, 5*time.Second); err != nil {
			_ = p.Close()
			return fmt.Errorf("%s helper not ready: %w", p.Name(), err)
		}
		return nil
	}
	if p.cmd != nil && p.cmd.Process != nil {
		done := p.stopCommandLocked()
		p.mu.Unlock()
		waitForHelperCommand(done)
		p.mu.Lock()
	}
	if err := os.MkdirAll(p.runtimeDir, 0700); err != nil {
		p.mu.Unlock()
		return err
	}
	exe, args, err := p.commandBuilder(p.localPort, p.runtimeDir)
	if err != nil {
		p.mu.Unlock()
		return err
	}
	cmd := exec.CommandContext(context.Background(), exe, args...)
	cmd.Dir = filepath.Dir(exe)
	setHideWindow(cmd)
	logFile, err := os.OpenFile(filepath.Join(p.runtimeDir, "helper-process.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err = cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		p.mu.Unlock()
		return err
	}
	done := make(chan struct{})
	p.cmd = cmd
	p.lastUsed = time.Now()
	p.waitDone = done
	p.waitErr = nil
	var startedPID int
	if cmd.Process != nil {
		startedPID = cmd.Process.Pid
		p.helperPID = startedPID
	}
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		if p.cmd == cmd {
			p.waitErr = err
		}
		p.mu.Unlock()
		if logFile != nil {
			_ = logFile.Close()
		}
		close(done)
	}()
	p.mu.Unlock()
	if startedPID > 0 {
		helpersMu.Lock()
		helperPIDs[startedPID] = p
		helpersMu.Unlock()
	}
	if err = p.waitUntilReady(ctx, 5*time.Second); err != nil {
		_ = p.Close()
		return fmt.Errorf("%s helper not ready: %w", p.Name(), err)
	}
	return nil
}

func (p *helperBackedProxy) helperKey() string {
	return p.runtimeDir
}

func lockHelperStart(key string) func() {
	value, _ := helperStartMu.LoadOrStore(key, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (p *helperBackedProxy) stopCommandLocked() chan struct{} {
	done := p.waitDone
	if p.cmd != nil && p.cmd.Process != nil {
		pid := p.cmd.Process.Pid
		if p.helperPID == 0 {
			p.helperPID = pid
		}
		_ = p.cmd.Process.Kill()
		forceKillPID(pid)
	} else if p.helperPID > 0 {
		forceKillPID(p.helperPID)
	}
	p.cmd = nil
	p.waitDone = nil
	p.waitErr = nil
	return done
}

func waitForHelperCommand(done chan struct{}) {
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func (p *helperBackedProxy) hasRunningCommand() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.commandRunningLocked()
}

func (p *helperBackedProxy) commandRunningLocked() bool {
	if p.cmd == nil || p.cmd.Process == nil || p.waitErr != nil {
		return false
	}
	if p.cmd.ProcessState != nil {
		return false
	}
	err := probeLocalPort(p.localPort)
	return err == nil
}


func killListenersOnPort(port int) {
	if port <= 0 {
		return
	}
	// Best-effort: kill tracked helpers bound to this port, then orphan sweep.
	helpersMu.Lock()
	var targets []*helperBackedProxy
	for p := range allHelpers {
		if p.localPort == port {
			targets = append(targets, p)
		}
	}
	helpersMu.Unlock()
	for _, p := range targets {
		p.forceStop()
		UnregisterHelper(p)
	}
	killOrphanHelperProcesses(activeHelperPIDs())
}
func probeLocalPort(port int) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 200*time.Millisecond)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

func (p *helperBackedProxy) waitUntilReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p.localPort)), 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		p.mu.Lock()
		waitErr := p.waitErr
		p.mu.Unlock()
		if waitErr != nil {
			return waitErr
		}
		if time.Now().After(deadline) {
			return lastErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func chooseLocalPort(seed string, port int) (int, error) {
	if port > 0 {
		return port, nil
	}
	sum := sha1.Sum([]byte(seed))
	hashValue := int(sum[0])<<8 | int(sum[1])
	return 20000 + hashValue%30000, nil
}

func makeHelperRuntimeDir(name, seed string) (string, error) {
	sum := sha1.Sum([]byte(seed))
	if runtime.GOOS == "android" {
		return filepath.Join(C.Path.HomeDir(), "protocol-helpers", "runtime", safeFilePart(name)+"-"+hex.EncodeToString(sum[:])[:8]), nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "protocol-helpers", "runtime", safeFilePart(name)+"-"+hex.EncodeToString(sum[:])[:8]), nil
}

func helperSeed(parts ...string) string {
	return strings.Join(parts, "\x00")
}

func resolveHelperPath(configured string, names ...string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		}
		return "", fmt.Errorf("helper not found: %s", configured)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	base := filepath.Dir(exe)
	dirs := []string{}

	// Android 涓婁紭鍏堜粠绯荤粺娉ㄥ叆鐨?nativeLibraryDir 鐜鍙橀噺鏌ユ壘
	// 杩欎釜鍊肩敱 Flutter/Android 灞傚湪鍚姩鏃堕€氳繃鐜鍙橀噺浼犲叆
	if nativeLibDir := os.Getenv("NATIVE_LIBRARY_DIR"); nativeLibDir != "" {
		dirs = append(dirs, nativeLibDir)
	}

	// Android 鍏滃簳锛氶€氳繃 /proc/self/maps 鑷彂鐜?native library 鐩綍
	if runtime.GOOS == "android" {
		if dir := androidNativeLibraryDir(); dir != "" {
			dirs = append(dirs, dir)
		}
	}

	// 鍘熸湁璺緞淇濇寔涓嶅彉
	dirs = append(dirs,
		filepath.Join(base, "protocol-helpers"),
		filepath.Join(base, "data", "protocol-helpers"),
		base,
	)
	for _, dir := range dirs {
		for _, name := range names {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("helper executable not found: %s", strings.Join(names, ", "))
}

func buildNaiveProxyURL(option NaiveProxyOption) (string, error) {
	if option.Proxy != "" {
		return option.Proxy, nil
	}
	scheme := naiveProxyScheme(option)
	u := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(option.Server, strconv.Itoa(option.Port)),
	}
	username := option.UserName
	if username == "" {
		username = option.User
	}
	if username != "" {
		u.User = url.UserPassword(username, option.Password)
	} else if option.Password != "" {
		u.User = url.UserPassword("", option.Password)
	}
	return u.String(), nil
}

func naiveProxyScheme(option NaiveProxyOption) string {
	if option.Scheme != "" {
		return option.Scheme
	}
	for _, alpn := range option.ALPN {
		switch strings.ToLower(strings.TrimSpace(alpn)) {
		case "h3", "http/3", "quic":
			return "quic"
		}
	}
	return "https"
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func safeFilePart(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "proxy"
	}
	return b.String()
}

// helperBinaryName returns the platform-appropriate binary filename.
// On Windows it appends ".exe"; on all other platforms (Android, Linux, macOS) it returns the name as-is.
func helperBinaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	if runtime.GOOS == "android" && base == "naive" {
		return "libnaive.so"
	}
	return base
}

func androidNativeLibraryDir() string {
	data, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "libplugin.so") ||
			strings.Contains(line, "libclash.so") ||
			strings.Contains(line, "libgojni.so") {
			fields := strings.Fields(line)
			if len(fields) >= 6 && strings.HasPrefix(fields[len(fields)-1], "/") {
				return filepath.Dir(fields[len(fields)-1])
			}
		}
	}
	return ""
}


