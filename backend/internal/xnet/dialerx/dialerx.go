// Package dialerx 提供 Dialer 构建辅助。
package dialerx

import (
	"context"
	"net"
	"time"
)

// Config 是 Dialer 配置。
type Config struct {
	Timeout   time.Duration
	KeepAlive time.Duration
	LocalAddr string
}

// Dialer 返回一个配置好的 net.Dialer。
func Dialer(cfg Config) *net.Dialer {
	d := &net.Dialer{
		Timeout:   cfg.Timeout,
		KeepAlive: cfg.KeepAlive,
	}
	if cfg.LocalAddr != "" {
		if addr, err := resolveAddr(cfg.LocalAddr); err == nil {
			d.LocalAddr = addr
		}
	}
	return d
}

func resolveAddr(addr string) (net.Addr, error) {
	host, port, err := splitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if !hasPort(addr) {
		return nil, nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, nil
		}
		ip = ips[0]
	}
	return &net.TCPAddr{IP: ip, Port: port}, nil
}

func splitHostPort(addr string) (string, int, error) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			port := 0
			for _, ch := range addr[i+1:] {
				if ch < '0' || ch > '9' {
					return addr[:i], 0, nil
				}
				port = port*10 + int(ch-'0')
			}
			return addr[:i], port, nil
		}
	}
	return addr, 0, nil
}

func hasPort(addr string) bool {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return true
		}
	}
	return false
}

// DialTCP 拨号 TCP（基于 Dialer）。
func DialTCP(ctx context.Context, d *net.Dialer, network, addr string) (net.Conn, error) {
	return d.DialContext(ctx, network, addr)
}
