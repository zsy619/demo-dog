// Package pprofx 提供一个轻量的 pprof HTTP 端点包装。
// 它绑定 /debug/pprof/* 路由并允许运行时启动/停止。
package pprofx

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/pprof"
	"sync"
	"sync/atomic"
)

// Server 是 pprof HTTP 服务包装。
type Server struct {
	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
	port     string
	running  atomic.Bool
}

// New 创建一个未启动的 pprof 服务。
func New() *Server {
	return &Server{}
}

// Start 在 addr 上启动 pprof 端点。
func (s *Server) Start(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running.Load() {
		return errors.New("pprofx: 已在运行")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	s.server = &http.Server{Handler: mux}
	s.listener = ln
	s.port = ln.Addr().String()
	s.running.Store(true)
	go func() {
		_ = s.server.Serve(ln)
		s.running.Store(false)
	}()
	return nil
}

// Stop 关闭 HTTP 服务。
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running.Load() {
		return nil
	}
	err := s.server.Shutdown(context.Background())
	s.running.Store(false)
	return err
}

// Running 返回服务是否在运行。
func (s *Server) Running() bool { return s.running.Load() }

// Addr 返回监听地址。
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// Handler 返回注册的 *http.ServeMux，便于嵌入现有服务。
func Handler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}
