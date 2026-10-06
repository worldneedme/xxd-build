package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net"
    "os"
    "sync"
    "time"

    O "github.com/metacubex/mihomo/adapter/outbound"
    C "github.com/metacubex/mihomo/constant"
)

func main() {
    if err := run(); err != nil {
        fmt.Fprintln(os.Stderr, "smoke failed:", err)
        os.Exit(1)
    }
}

func run() error {
    hop := os.Getenv("SMOKE_MODE") == "hop"
    chunks, queries := 8, 4
    if hop { chunks, queries = 80, 40 }
    start := float64(time.Now().UnixNano()) / 1e9
    j, err := O.NewJuicity(O.JuicityOption{
        Name: "isolated-hk-dev-smoke", Server: "69.33.211.38", Port: 10444,
        UUID: os.Getenv("HKDEV_UUID"), Password: os.Getenv("HKDEV_PASSWORD"),
        SNI: "hkdev01.020021.qzz.io", UDP: true,
        Hop: hop, HopPorts: "20000-20020", HopInterval: 5,
    })
    if err != nil { return err }
    ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
    defer cancel()
    var wg sync.WaitGroup
    errs := make(chan error, 2)
    var tcpBytes int
    var udpOK int
    var maxRTT time.Duration
    wg.Add(2)
    go func() {
        defer wg.Done()
        c, err := j.DialContext(ctx, &C.Metadata{Host: "127.0.0.1", DstPort: 18082, NetWork: C.TCP})
        if err != nil { errs <- fmt.Errorf("TCP dial: %w", err); return }
        defer c.Close()
        c.SetDeadline(time.Now().Add(75*time.Second))
        if _, err = fmt.Fprintf(c, "%d\n", chunks); err != nil { errs <- err; return }
        buf := make([]byte, 16384)
        for i := 0; i < chunks; i++ {
            if _, err := io.ReadFull(c, buf); err != nil { errs <- fmt.Errorf("TCP chunk %d: %w", i, err); return }
            for k, b := range buf {
                if b != byte((i+k)%251) { errs <- fmt.Errorf("TCP corruption at chunk %d byte %d", i, k); return }
            }
            tcpBytes += len(buf)
        }
        extra := make([]byte, 1)
        n, err := c.Read(extra)
        if n != 0 || err != io.EOF { errs <- fmt.Errorf("TCP tail: n=%d err=%v", n, err) }
    }()
    go func() {
        defer wg.Done()
        pc, err := j.ListenPacketContext(ctx, &C.Metadata{Host: "127.0.0.1", DstPort: 15354, NetWork: C.UDP})
        if err != nil { errs <- fmt.Errorf("UDP dial: %w", err); return }
        defer pc.Close()
        addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 15354}
        buf := make([]byte, 2048)
        for i := 0; i < queries; i++ {
            payload := []byte(fmt.Sprintf("hk-dev-hop-echo-%03d", i))
            t := time.Now()
            pc.SetDeadline(t.Add(3*time.Second))
            if _, err := pc.WriteTo(payload, addr); err != nil { errs <- fmt.Errorf("UDP write %d: %w", i, err); return }
            n, _, err := pc.ReadFrom(buf)
            if err != nil { errs <- fmt.Errorf("UDP read %d: %w", i, err); return }
            if !bytes.Equal(buf[:n], payload) { errs <- fmt.Errorf("UDP corruption at %d", i); return }
            if rtt := time.Since(t); rtt > maxRTT { maxRTT = rtt }
            udpOK++
            time.Sleep(time.Until(t.Add(time.Second)))
        }
    }()
    wg.Wait()
    close(errs)
    for err := range errs { if err != nil { return err } }
    return json.NewEncoder(os.Stdout).Encode(map[string]any{
        "mode": os.Getenv("SMOKE_MODE"), "round": os.Getenv("SMOKE_ROUND"),
        "start": start, "end": float64(time.Now().UnixNano()) / 1e9,
        "tcp_bytes": tcpBytes, "udp_echo_ok": udpOK, "udp_max_rtt_ms": maxRTT.Milliseconds(),
    })
}
