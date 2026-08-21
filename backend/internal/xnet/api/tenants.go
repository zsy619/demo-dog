package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// handleTenantMintKey 为一个 tenant 生成一个新的 API key。
// 明文密钥仅返回一次。
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
	// 同时在 admin store 中留一份,使 listTenantKeys
	// /rotateTenantKey/revokeTenantKey 能够工作。
	if s.adminKeys != nil {
		if _, entry, aerr := s.adminKeys.CreateKey(req.Role, tenantID, nil, 0); aerr == nil && entry != nil {
			k.KeyID = entry.KeyID
			k.CreatedAt = entry.CreatedAt
		}
	}
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

// handleTenantsRoute 将 /api/tenants/<id>/keys 分发到 minter。
// /api/tenants/ 下的其他任何路径都返回 404。
func (s *Server) handleTenantsRoute(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/tenants/")
	if strings.HasSuffix(suffix, "/keys") {
		s.handleTenantMintKey(w, r)
		return
	}
	http.NotFound(w, r)
}

// handleTenantsDispatch 同时服务于 /api/tenants (列表 + 创建) 和
// /api/tenants/<id>/keys (mint)。Go stdlib mux 不能在单个 HandleFunc 上
// 模式匹配后缀，因此我们按 URL 形状分发。
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
		// GET /api/tenants/<id>/keys   -> 列出属于该 tenant 的 key
		// POST /api/tenants/<id>/keys  -> 签发新 key
		if r.Method == http.MethodGet {
			s.handleTenantListKeys(w, r)
			return
		}
		s.handleTenantMintKey(w, r)
		return
	}
	if strings.Contains(r.URL.Path, "/keys/") {
		path := strings.TrimPrefix(r.URL.Path, "/api/tenants/")
		parts := strings.Split(path, "/")
		// POST /api/tenants/<id>/keys/<keyId>/rotate -> 轮换
		// DELETE /api/tenants/<id>/keys/<keyId>     -> 撤销
		if len(parts) >= 4 && parts[3] == "rotate" {
			s.handleTenantRotateKey(w, r)
			return
		}
		if len(parts) == 3 {
			s.handleTenantRevokeKey(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

// handleTenantListKeys 返回属于指定 tenant 的 key 列表。
// 数据来源:adminKeys AdminStore,以 Tenant 字段过滤。
func (s *Server) handleTenantListKeys(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimPrefix(r.URL.Path, "/api/tenants/")
	tenantID = strings.TrimSuffix(tenantID, "/keys")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("tenant id required"))
		return
	}
	if s.tenants == nil {
		writeJSON(w, http.StatusOK, map[string]any{"keys": []any{}})
		return
	}
	if _, err := s.tenants.Get(tenantID); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	out := []map[string]any{}
	if s.adminKeys != nil {
		for _, k := range s.adminKeys.ListKeys() {
			if k.Tenant != tenantID {
				continue
			}
			out = append(out, map[string]any{
				"id":         k.KeyID,
				"label":      k.Identity,
				"role":       k.Identity,
				"tenant":     k.Tenant,
				"scopes":     k.Scopes,
				"created_at": k.CreatedAt.Format(time.RFC3339Nano),
				"expires_at": formatExpiresAtForTenant(k.ExpiresAt),
				"disabled":   k.Disabled,
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": out})
}

// handleTenantRevokeKey 永久删除一个 tenant 下的 key。
func (s *Server) handleTenantRevokeKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("DELETE only"))
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/tenants/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("key id required"))
		return
	}
	keyID := parts[2]
	if s.adminKeys == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("admin keys not initialised"))
		return
	}
	if err := s.adminKeys.DeleteKey(keyID); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": keyID})
}

// handleTenantRotateKey 颁发一个新的 tenant key(替换指定 id 的旧 key)。
func (s *Server) handleTenantRotateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/tenants/")
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[3] != "rotate" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("path must end with /rotate"))
		return
	}
	keyID := parts[2]
	graceNs, _ := strconv.ParseInt(r.URL.Query().Get("grace_ns"), 10, 64)
	if s.adminKeys == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("admin keys not initialised"))
		return
	}
	plaintext, _, newEntry, err := s.adminKeys.RotateKey(keyID, time.Duration(graceNs))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key_id":    newEntry.KeyID,
		"plaintext": plaintext,
	})
}

func formatExpiresAtForTenant(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

// handleTenantMintKey 中的 TrimPrefix 需要更新后的路径，因为
// 路由前缀可能包含前导空白。在处理器内部
// 从 r.URL.Path 重新派生。
