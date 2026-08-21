package replica

import (
	"encoding/json"
	"net/http"
)

// ReplicaServer 组合 primary 端的 HTTP handler 与认证
// and the cluster state. Operators mount ServeReplica on their main
// 多路复用器 or run a dedicated listener for /副本/*.
type ReplicaServer struct {
	Auth    *Auth
	Primary *PrimaryState
	Node    *Node
}

// Handler 返回带认证包装的处理函数树。
func (s *ReplicaServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/replica/ack", s.Primary.AckHandler())
	mux.HandleFunc("/replica/state", s.stateHandler)
	if s.Node != nil {
		mux.Handle("/replica/wal", s.Node.Handler())
	}
	mux.HandleFunc("/replica/offset", s.offsetHandler)
	if s.Auth != nil && s.Auth.Enabled() {
		return s.Auth.Middleware(mux)
	}
	return mux
}

func (s *ReplicaServer) offsetHandler(w http.ResponseWriter, r *http.Request) {
	off, _, _, _ := s.Primary.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"offset": off})
}

func (s *ReplicaServer) stateHandler(w http.ResponseWriter, r *http.Request) {
	off, followers, dropped, acks := s.Primary.Snapshot()
	for i := range followers {
		followers[i].Lag = off - followers[i].AckOffset
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"offset":    off,
		"followers": followers,
		"dropped":   dropped,
		"acks":      acks,
	})
}
