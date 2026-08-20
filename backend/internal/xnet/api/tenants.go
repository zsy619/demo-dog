package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zsy619/demo-dog/backend/internal/xdata/tenants"
)

// handleTenantsList 返回所有租户。仅限管理员。
func (s *Server) handleTenantsList(w http.ResponseWriter, r *http.Request) {
	if s.tenants == nil {
		writeJSON(w, http.StatusOK, map[string]any{"tenants": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenants": s.tenants.List()})
}

type createTenantReq struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// handleTenantCreate 注册一个新租户。仅限管理员。
func (s *Server) handleTenantCreate(w http.ResponseWriter, r *http.Request) {
	if s.tenants == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("tenants disabled"))
		return
	}
	var req createTenantReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	t, err := s.tenants.CreateTenant(req.ID, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

type mintKeyReq struct {
	Label string `json:"label"`
	Role  string `json:"role"`
}

// handleTenantMintKey 为一个租户生成一个新的 API 密钥。
// 明文仅返回一次。
func (s *Server) handleTenantMintKey(w http.ResponseWriter, r *http.Request) {
	if s.tenants == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("tenants disabled"))
		return
	}
	tenantID := strings.TrimPrefix(r.URL.Path, "/api/tenants/")
	tenantID = strings.TrimSuffix(tenantID, "/keys")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("tenant id required"))
		return
	}
	var req mintKeyReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Label == "" {
		req.Label = "default"
	}
	if req.Role == "" {
		req.Role = "writer"
	}
	k, err := s.tenants.MintKey(tenantID, req.Label, req.Role)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	// 在 auth 层中注册刚刚签发的密钥。
	s.auth.AddForTenant(k.Plaintext, k.Label, tenantID, ParseRole(k.Role))
	writeJSON(w, http.StatusCreated, k)
}

// resolveTenant 检查传入请求并返回调用方
// 正在操作的租户。解析顺序：
//
//   1. 由 auth 中间件盖上的 X-Dog-Tenant 头部（当
//      API 密钥被绑定到某个租户时）。
//   2. ?tenant=... 查询参数（供管理员模拟使用）。
//
// 管理员可以通过查询参数显式指定租户来模拟；
// 非管理员被固定为其密钥所绑定
// 的租户（或在未绑定时为空，属于平台管理员范畴）。
// 
func resolveTenant(r *http.Request) string {
	if t := r.Header.Get("X-Dog-Tenant"); t != "" {
		return t
	}
	return r.URL.Query().Get("tenant")
}

var _ = subtle.ConstantTimeCompare
var _ = tenants.ErrNotFound

// handleTenantsRoute 将 /api/tenants/<id>/keys 分发到
// minter。/api/tenants/ 下其他内容返回 404。
func (s *Server) handleTenantsRoute(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/tenants/")
	if strings.HasSuffix(suffix, "/keys") {
		s.handleTenantMintKey(w, r)
		return
	}
	http.NotFound(w, r)
}

// handleTenantsDispatch 同时服务 /api/tenants（列出 + 创建）和
// /api/tenants/<id>/keys（签发密钥）。Go 标准库的 mux
// 不会在单个 HandleFunc 上对后缀做模式匹配，因此按 URL 形状分发。
func (s *Server) handleTenantsDispatch(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/tenants":
		if r.Method == http.MethodPost {
			s.handleTenantCreate(w, r)
			return
		}
		s.handleTenantsList(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/keys") {
		s.handleTenantMintKey(w, r)
		return
	}
	http.NotFound(w, r)
}

// handleTenantMintKey 中的 TrimPrefix 需要更新后的路径，因为
// 路由前缀可能包含前导空白。在处理器内部
// 从 r.URL.Path 重新派生。
