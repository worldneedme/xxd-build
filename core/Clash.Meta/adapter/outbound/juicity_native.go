package outbound

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"syscall"
	"time"

	N "github.com/metacubex/mihomo/common/net"
	"github.com/metacubex/mihomo/component/dialer"
	"github.com/metacubex/mihomo/component/resolver"
	C "github.com/metacubex/mihomo/constant"

	"github.com/daeuniverse/outbound/netproxy"
	"github.com/daeuniverse/outbound/protocol"
	daeJuicity "github.com/daeuniverse/outbound/protocol/juicity"
)

type Juicity struct {
	*Base
	option *JuicityOption
	dialer netproxy.Dialer
}

type JuicityOption struct {
	BasicOption
	Name                  string `proxy:"name"`
	Server                string `proxy:"server"`
	Port                  int    `proxy:"port"`
	UUID                  string `proxy:"uuid"`
	Password              string `proxy:"password"`
	SNI                   string `proxy:"sni,omitempty"`
	AllowInsecure         bool   `proxy:"allow-insecure,omitempty"`
	SkipCertVerify        bool   `proxy:"skip-cert-verify,omitempty"`
	CongestionControl     string `proxy:"congestion-control,omitempty"`
	CongestionController  string `proxy:"congestion-controller,omitempty"`
	PinnedCertchainSha256 string `proxy:"pinned-certchain-sha256,omitempty"`
	HopPorts              string `proxy:"hop-ports,omitempty"`
	HopInterval           int    `proxy:"hop-interval,omitempty"`
	LogLevel              string `proxy:"log-level,omitempty"`
	UDP                   bool   `proxy:"udp,omitempty"`
}

func firstJuicityHopPort(hopPorts string) (int, error) {
	for _, part := range strings.Split(hopPorts, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		startText := part
		if i := strings.Index(part, "-"); i >= 0 {
			startText = strings.TrimSpace(part[:i])
		}
		port, err := strconv.Atoi(startText)
		if err != nil {
			return 0, err
		}
		if port <= 0 || port > 65535 {
			return 0, fmt.Errorf("port out of range: %d", port)
		}
		return port, nil
	}
	return 0, errors.New("empty hop-ports")
}

func NewJuicity(option JuicityOption) (*Juicity, error) {
	if option.Name == "" {
		return nil, errors.New("juicity missing name")
	}
	if option.Server == "" || option.Port == 0 || option.UUID == "" || option.Password == "" {
		return nil, errors.New("juicity requires server, port, uuid and password")
	}
	if option.CongestionControl == "" && option.CongestionController != "" {
		option.CongestionControl = option.CongestionController
	}
	if option.CongestionControl == "" {
		option.CongestionControl = "bbr"
	}

	displayPort := option.Port
	if option.HopPorts != "" {
		if first, err := firstJuicityHopPort(option.HopPorts); err == nil {
			displayPort = first
		}
	}
	addr := net.JoinHostPort(option.Server, strconv.Itoa(displayPort))
	serverHost := option.Server
	if ip := net.ParseIP(serverHost); ip == nil {
		resolveCtx, resolveCancel := context.WithTimeout(
			context.Background(),
			8*time.Second,
		)
		resolved, resolveErr := resolveIPWithResolver(
			resolveCtx,
			serverHost,
			option.IPVersion,
			resolver.ProxyServerHostResolver,
		)
		resolveCancel()
		if resolveErr == nil {
			serverHost = resolved.String()
		}
	}
	dialAddr := net.JoinHostPort(serverHost, strconv.Itoa(option.Port))

	outbound := &Juicity{
		Base: NewBase(BaseOption{
			Name:         option.Name,
			Addr:         addr,
			Type:         C.Juicity,
			ProviderName: option.ProviderName,
			UDP:          option.UDP,
			TFO:          option.TFO,
			MPTCP:        option.MPTCP,
			Interface:    option.Interface,
			RoutingMark:  option.RoutingMark,
			Prefer:       option.IPVersion,
		}),
		option: &option,
	}
	outbound.Base.dialer = option.NewDialer(outbound.DialOptions())

	tlsConfig, err := buildJuicityTLSConfig(option)
	if err != nil {
		return nil, err
	}
	underlay := &juicityUnderlayDialer{
		dialer: outbound.Base.dialer,
		prefer: option.IPVersion,
	}
	dialer, err := daeJuicity.NewDialer(underlay, protocol.Header{
		ProxyAddress: dialAddr,
		Feature1:     option.CongestionControl,
		TlsConfig:    tlsConfig,
		User:         option.UUID,
		Password:     option.Password,
		IsClient:     true,
		Flags:        0,
	})
	if err != nil {
		return nil, err
	}
	outbound.dialer = dialer
	return outbound, nil
}

func (j *Juicity) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	destination := j.buildDestination(metadata)
	conn, err := j.dialer.DialContext(ctx, "tcp", destination)
	if err != nil {
		return nil, err
	}
	return NewConn(&juicityNetConn{
		Conn:  conn,
		laddr: &net.TCPAddr{},
		raddr: tcpAddrFromMetadata(metadata),
	}, j), nil
}

func (j *Juicity) ListenPacketContext(ctx context.Context, metadata *C.Metadata) (_ C.PacketConn, err error) {
	if !j.SupportUDP() {
		return nil, C.ErrNotSupport
	}
	destination := j.buildDestination(metadata)
	conn, err := j.dialer.DialContext(ctx, "udp", destination)
	if err != nil {
		return nil, err
	}
	pc, ok := conn.(netproxy.PacketConn)
	if !ok {
		return nil, fmt.Errorf("juicity udp dial returned %T, want netproxy.PacketConn", conn)
	}
	return NewPacketConn(N.NewThreadSafePacketConn(&juicityPacketConn{
		PacketConn: pc,
		laddr:      &net.UDPAddr{},
		raddr:      udpAddrFromMetadata(metadata),
	}), j), nil
}

// FIX Bug #2: 不在客户端提前解析目标域名，直接把域名传给 juicity 代理层
// 原代码调用 resolveIPWithResolver 把域名解析成 IP，在 Android 上会产生 DNS 循环依赖
// Juicity 作为代理协议，应由服务端解析目标域名
func (j *Juicity) buildDestination(metadata *C.Metadata) string {
	if metadata.Host != "" {
		return net.JoinHostPort(metadata.Host, strconv.Itoa(int(metadata.DstPort)))
	}
	return metadata.RemoteAddress()
}

func (j *Juicity) ProxyInfo() C.ProxyInfo {
	info := j.Base.ProxyInfo()
	info.DialerProxy = j.option.DialerProxy
	return info
}

func buildJuicityTLSConfig(option JuicityOption) (*tls.Config, error) {
	serverName := option.Server
	if option.SNI != "" {
		serverName = option.SNI
	}
	tlsConfig := &tls.Config{
		NextProtos:         []string{"h3"},
		MinVersion:         tls.VersionTLS13,
		ServerName:         serverName,
		InsecureSkipVerify: option.AllowInsecure || option.SkipCertVerify,
	}
	if option.PinnedCertchainSha256 == "" {
		return tlsConfig, nil
	}
	pinnedHash, err := parseJuicityPinnedHash(option.PinnedCertchainSha256)
	if err != nil {
		return nil, err
	}
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if !bytes.Equal(generateJuicityCertChainHash(rawCerts), pinnedHash) {
			return errors.New("pinned hash of cert chain does not match")
		}
		return nil
	}
	return tlsConfig, nil
}

func parseJuicityPinnedHash(value string) ([]byte, error) {
	if pinnedHash, err := base64.URLEncoding.DecodeString(value); err == nil {
		return pinnedHash, nil
	}
	if pinnedHash, err := base64.StdEncoding.DecodeString(value); err == nil {
		return pinnedHash, nil
	}
	if pinnedHash, err := hex.DecodeString(value); err == nil {
		return pinnedHash, nil
	}
	return nil, errors.New("failed to decode pinned-certchain-sha256")
}

func generateJuicityCertChainHash(rawCerts [][]byte) (chainHash []byte) {
	for _, cert := range rawCerts {
		certHash := sha256.Sum256(cert)
		if chainHash == nil {
			chainHash = certHash[:]
			continue
		}
		newHash := sha256.Sum256(append(chainHash, certHash[:]...))
		chainHash = newHash[:]
	}
	return chainHash
}

type juicityUnderlayDialer struct {
	dialer C.Dialer
	prefer C.DNSPrefer
}

func (d *juicityUnderlayDialer) DialContext(ctx context.Context, network, addr string) (netproxy.Conn, error) {
	magicNetwork, err := netproxy.ParseMagicNetwork(network)
	if err != nil {
		return nil, err
	}
	switch magicNetwork.Network {
	case "tcp":
		conn, err := d.dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
		// Manually protect the underlying TCP socket for Android VPN
		if dialer.DefaultSocketHook != nil {
			if sysConn, ok := conn.(syscall.Conn); ok {
				if rawConn, err2 := sysConn.SyscallConn(); err2 == nil {
					_ = dialer.DefaultSocketHook("tcp", addr, rawConn)
				}
			}
		}
		return conn, nil
	case "udp":
		rAddrPort, err := d.resolveUDPAddrPort(ctx, addr)
		if err != nil {
			return nil, err
		}
		remoteAddr := net.UDPAddrFromAddrPort(rAddrPort)
		sessionCtx, sessionCancel := context.WithCancel(context.Background())
		_ = sessionCancel
		pc, err := d.dialer.ListenPacket(sessionCtx, "udp", "", rAddrPort)
		if err != nil {
			return nil, err
		}

		// Manually protect the underlying UDP socket for Android VPN
		if dialer.DefaultSocketHook != nil {
			if sysConn, ok := pc.(syscall.Conn); ok {
				if rawConn, err := sysConn.SyscallConn(); err == nil {
					_ = dialer.DefaultSocketHook("udp", rAddrPort.String(), rawConn)
				}
			}
		}

		return &juicityUnderlayPacketConn{
			PacketConn: pc,
			remote:     rAddrPort,
			remoteAddr: remoteAddr,
		}, nil
	default:
		return nil, fmt.Errorf("%w: %s", netproxy.UnsupportedTunnelTypeError, magicNetwork.Network)
	}
}

func (d *juicityUnderlayDialer) resolveUDPAddrPort(ctx context.Context, addr string) (netip.AddrPort, error) {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return netip.AddrPort{}, err
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return netip.AddrPort{}, err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		ip, err = resolveIPWithResolver(ctx, host, d.prefer, resolver.ProxyServerHostResolver)
		if err != nil {
			return netip.AddrPort{}, fmt.Errorf("resolve juicity underlay addr %s: %w", host, err)
		}
	}
	return netip.AddrPortFrom(ip.Unmap(), uint16(port)), nil
}

type juicityUnderlayPacketConn struct {
	net.PacketConn
	remote     netip.AddrPort
	remoteAddr net.Addr
}

func (c *juicityUnderlayPacketConn) Read(b []byte) (int, error) {
	n, _, err := c.ReadFrom(b)
	return n, err
}

func (c *juicityUnderlayPacketConn) Write(b []byte) (int, error) {
	return c.PacketConn.WriteTo(b, c.remoteAddr)
}

func (c *juicityUnderlayPacketConn) ReadFrom(b []byte) (int, netip.AddrPort, error) {
	n, addr, err := c.PacketConn.ReadFrom(b)
	if err != nil {
		return n, netip.AddrPort{}, err
	}
	addrPort, err := addrToAddrPort(addr)
	if err != nil {
		return n, netip.AddrPort{}, err
	}
	return n, addrPort, nil
}

func (c *juicityUnderlayPacketConn) WriteTo(b []byte, addr string) (int, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return 0, err
	}
	return c.PacketConn.WriteTo(b, udpAddr)
}

type juicityNetConn struct {
	netproxy.Conn
	laddr net.Addr
	raddr net.Addr
}

func (c *juicityNetConn) LocalAddr() net.Addr {
	return c.laddr
}

func (c *juicityNetConn) RemoteAddr() net.Addr {
	return c.raddr
}

type juicityPacketConn struct {
	netproxy.PacketConn
	laddr net.Addr
	raddr net.Addr
}

func (c *juicityPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(b)
	if err != nil {
		return n, nil, err
	}
	return n, net.UDPAddrFromAddrPort(addr), nil
}

func (c *juicityPacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	return c.PacketConn.WriteTo(b, addr.String())
}

func (c *juicityPacketConn) LocalAddr() net.Addr {
	return c.laddr
}

func (c *juicityPacketConn) RemoteAddr() net.Addr {
	return c.raddr
}

func addrToAddrPort(addr net.Addr) (netip.AddrPort, error) {
	if addr, ok := addr.(interface{ AddrPort() netip.AddrPort }); ok {
		return addr.AddrPort(), nil
	}
	host, portText, err := net.SplitHostPort(addr.String())
	if err != nil {
		return netip.AddrPort{}, err
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return netip.AddrPort{}, err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.AddrPort{}, err
	}
	return netip.AddrPortFrom(ip.Unmap(), uint16(port)), nil
}

func tcpAddrFromMetadata(metadata *C.Metadata) net.Addr {
	addr := &net.TCPAddr{Port: int(metadata.DstPort)}
	if ip, err := netip.ParseAddr(metadata.Host); err == nil {
		addr.IP = ip.AsSlice()
	}
	return addr
}

func udpAddrFromMetadata(metadata *C.Metadata) net.Addr {
	addr := &net.UDPAddr{Port: int(metadata.DstPort)}
	if ip, err := netip.ParseAddr(metadata.Host); err == nil {
		addr.IP = ip.AsSlice()
	}
	return addr
}

var _ net.Conn = (*juicityNetConn)(nil)
var _ net.PacketConn = (*juicityPacketConn)(nil)
var _ netproxy.Dialer = (*juicityUnderlayDialer)(nil)
var _ netproxy.PacketConn = (*juicityUnderlayPacketConn)(nil)
var _ netproxy.Conn = (*juicityUnderlayPacketConn)(nil)
