// Package openapi 为 demo-dog HTTP API 生成 OpenAPI 3.1 规范。
// 该规范由本包的路由列表手工构建，
// 我们不内省运行时的处理器，因为希望
// 规范保持稳定、可审阅，并解耦于任何
// 反射机制。
//
// 使用者：
//   * docs/openapi.json（由 gen/main.go 生成）
//   * 客户端 SDK 代码生成（openapi-generator、oapi-codegen）
//   * Postman / Insomnia 导入
//
// 用法：
//
//	go run ./cmd/gen-openapi > docs/openapi.json
package openapi

import "encoding/json"

// Spec 是根 OpenAPI 3.1 文档。
type Spec struct {
	OpenAPI      string                 `json:"openapi"`
	Info         Info                   `json:"info"`
	Servers      []Server               `json:"servers"`
	Paths        map[string]PathItem    `json:"paths"`
	Components   Components             `json:"components"`
	Tags         []Tag                  `json:"tags,omitempty"`
	Security     []map[string][]string  `json:"security,omitempty"`
}

type Info struct {
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Version     string  `json:"version"`
	Contact     *Contact `json:"contact,omitempty"`
	License     *License `json:"license,omitempty"`
}

type Contact struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type License struct {
	Name string `json:"name"`
}

type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type PathItem struct {
	Get        *Op `json:"get,omitempty"`
	Post       *Op `json:"post,omitempty"`
	Put        *Op `json:"put,omitempty"`
	Delete     *Op `json:"delete,omitempty"`
	Parameters []Param `json:"parameters,omitempty"`
}

type Op struct {
	Summary     string  `json:"summary,omitempty"`
	Description string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	OperationID string  `json:"operationId,omitempty"`
	Parameters  []Param `json:"parameters,omitempty"`
	RequestBody *ReqBody `json:"requestBody,omitempty"`
	Responses   map[string]Resp `json:"responses"`
	Security    []map[string][]string `json:"security,omitempty"`
}

type Param struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

type ReqBody struct {
	Description string  `json:"description,omitempty"`
	Content     map[string]MediaType `json:"content"`
	Required    bool    `json:"required,omitempty"`
}

type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

type Resp struct {
	Description string  `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
	Headers     map[string]Header `json:"headers,omitempty"`
}

type Header struct {
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema"`
}

type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Description          string             `json:"description,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	Example              any                `json:"example,omitempty"`
	AdditionalProperties any                `json:"additionalProperties,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	OneOf                []*Schema          `json:"oneOf,omitempty"`
	AnyOf                []*Schema          `json:"anyOf,omitempty"`
	AllOf                []*Schema          `json:"allOf,omitempty"`
	Default              any                `json:"default,omitempty"`
}

type Components struct {
	Schemas         map[string]*Schema `json:"schemas,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
	Parameters      map[string]*Param `json:"parameters,omitempty"`
	Headers         map[string]*Header `json:"headers,omitempty"`
}

type SecurityScheme struct {
	Type         string  `json:"type"`
	Description  string  `json:"description,omitempty"`
	Scheme       string  `json:"scheme,omitempty"`
	BearerFormat string  `json:"bearerFormat,omitempty"`
	In           string  `json:"in,omitempty"`
	Name         string  `json:"name,omitempty"`
	Flows        *Flows  `json:"flows,omitempty"`
}

type Flows struct {
	Implicit *Flow `json:"implicit,omitempty"`
	Password *Flow `json:"password,omitempty"`
	ClientCredentials *Flow `json:"clientCredentials,omitempty"`
	AuthorizationCode *Flow `json:"authorizationCode,omitempty"`
}

type Flow struct {
	AuthorizationURL string  `json:"authorizationUrl,omitempty"`
	TokenURL         string  `json:"tokenUrl,omitempty"`
	RefreshURL       string  `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// New 返回 demo-dog 的 OpenAPI 3.1 规范。
func New() *Spec {
	return &Spec{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       "demo-dog",
			Description: "Lightweight OTel + Prometheus compatible observability backend. Stdlib only, single binary.",
			Version:     "1.0.0",
			Contact:     &Contact{Name: "demo-dog maintainers", URL: "https://github.com/zsy619/demo-dog"},
			License:     &License{Name: "Apache-2.0"},
		},
		Servers: []Server{
			{URL: "http://localhost:8088", Description: "Local development"},
		},
		Tags: []Tag{
			{Name: "ingest", Description: "OTLP and Prometheus ingest"},
			{Name: "query", Description: "Read APIs (PromQL, labels, series, metadata)"},
			{Name: "admin", Description: "Tenant, key, and configuration management"},
			{Name: "alerts", Description: "Rules and firing events"},
			{Name: "observability", Description: "Health, metrics, snapshot"},
		},
		Components: components(),
		Paths:      paths(),
	}
}

// JSON 将规范渲染为缩进的 JSON。
func (s *Spec) JSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

func components() Components {
	return Components{
		SecuritySchemes: map[string]*SecurityScheme{
			"ApiKey": {
				Type:         "apiKey",
				In:           "header",
				Name:         "X-API-Key",
				Description:  "demo-dog API key, configured via /api/keys",
			},
			"Bearer": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "opaque",
				Description:  "Bearer token for /replica/* endpoints",
			},
		},
		Schemas: map[string]*Schema{
			"Error": {
				Type: "object",
				Properties: map[string]*Schema{
					"error": {Type: "string"},
				},
				Required: []string{"error"},
			},
			"Health": {
				Type: "object",
				Properties: map[string]*Schema{
					"status": {Type: "string", Enum: []any{"ok", "degraded"}},
					"uptime_seconds": {Type: "number"},
					"cardinality": {
						Type: "object",
						Properties: map[string]*Schema{
							"current": {Type: "integer"},
							"cap":     {Type: "integer"},
							"dropped": {Type: "integer"},
						},
					},
				},
			},
			"QueryResponse": {
				Type: "object",
				Properties: map[string]*Schema{
					"status": {Type: "string"},
					"data": {
						Type: "object",
						Properties: map[string]*Schema{
							"resultType": {Type: "string", Enum: []any{"vector", "matrix", "scalar"}},
							"result":     {Type: "array", Items: &Schema{Type: "object", AdditionalProperties: true}},
						},
					},
				},
			},
			"RulesGroup": {
				Type: "object",
				Properties: map[string]*Schema{
					"name":  {Type: "string"},
					"file":  {Type: "string"},
					"rules": {Type: "array", Items: &Schema{Type: "object", AdditionalProperties: true}},
				},
			},
		},
	}
}

func paths() map[string]PathItem {
	mk := func(op *Op) PathItem {
		p := PathItem{}
		if op != nil {
			if op.Responses == nil {
				op.Responses = map[string]Resp{
					"200": {Description: "OK"},
				}
			}
			if op.Method() == "GET" {
				p.Get = op
			} else if op.Method() == "POST" {
				p.Post = op
			} else if op.Method() == "PUT" {
				p.Put = op
			} else if op.Method() == "DELETE" {
				p.Delete = op
			}
		}
		return p
	}

	p := map[string]PathItem{}

	// 健康检查。
	p["/api/health"] = mk(&Op{
		Summary:     "Health check",
		Tags:        []string{"observability"},
		OperationID: "getHealth",
		Responses: map[string]Resp{
			"200": {Description: "OK", Content: map[string]MediaType{
				"application/json": {Schema: ref("#/components/schemas/Health")},
			}},
		},
	})

	// 查询。
	p["/api/v1/query"] = mk(&Op{
		Summary:     "PromQL query (instant)",
		Tags:        []string{"query"},
		OperationID: "queryInstant",
		Parameters: []Param{{
			Name: "query", In: "query", Required: true,
			Schema: &Schema{Type: "string"},
		}, {
			Name: "time", In: "query", Required: false,
			Schema: &Schema{Type: "string", Format: "date-time"},
		}},
		Responses: map[string]Resp{
			"200": {Description: "OK", Content: map[string]MediaType{
				"application/json": {Schema: ref("#/components/schemas/QueryResponse")},
			}},
		},
	})

	// 序列。
	p["/api/v1/series"] = mk(&Op{
		Summary:     "Series discovery",
		Tags:        []string{"query"},
		OperationID: "getSeries",
		Parameters: []Param{{
			Name: "match[]", In: "query", Required: true,
			Schema: &Schema{Type: "array", Items: &Schema{Type: "string"}},
		}},
		Responses: map[string]Resp{
			"200": {Description: "OK"},
		},
	})

	// 规则。
	p["/api/v1/rules"] = mk(&Op{
		Summary:     "Active alerting rules",
		Tags:        []string{"alerts"},
		OperationID: "getRules",
		Responses: map[string]Resp{
			"200": {Description: "OK", Content: map[string]MediaType{
				"application/json": {Schema: &Schema{
					Type: "object",
					Properties: map[string]*Schema{
						"groups": {Type: "array", Items: ref("#/components/schemas/RulesGroup")},
					},
				}},
			}},
		},
	})

	// Ingest。
	p["/v1/logs"] = mk(&Op{
		Summary:     "OTLP/HTTP logs ingest",
		Tags:        []string{"ingest"},
		OperationID: "ingestLogs",
		RequestBody: &ReqBody{Required: true, Content: map[string]MediaType{
			"application/json": {Schema: &Schema{Type: "object", AdditionalProperties: true}},
			"application/x-protobuf": {Schema: &Schema{Type: "string", Format: "binary"}},
		}},
		Responses: map[string]Resp{
			"200": {Description: "Accepted"},
		},
	})
	p["/v1/metrics"] = mk(&Op{
		Summary:     "OTLP/HTTP metrics ingest",
		Tags:        []string{"ingest"},
		OperationID: "ingestMetrics",
		RequestBody: &ReqBody{Required: true, Content: map[string]MediaType{
			"application/json": {Schema: &Schema{Type: "object", AdditionalProperties: true}},
		}},
		Responses: map[string]Resp{
			"200": {Description: "Accepted"},
		},
	})
	p["/v1/traces"] = mk(&Op{
		Summary:     "OTLP/HTTP traces ingest",
		Tags:        []string{"ingest"},
		OperationID: "ingestTraces",
		RequestBody: &ReqBody{Required: true, Content: map[string]MediaType{
			"application/json": {Schema: &Schema{Type: "object", AdditionalProperties: true}},
		}},
		Responses: map[string]Resp{
			"200": {Description: "Accepted"},
		},
	})
	p["/api/v1/write"] = mk(&Op{
		Summary:     "Prometheus Remote Write",
		Tags:        []string{"ingest"},
		OperationID: "promWrite",
		RequestBody: &ReqBody{Required: true, Content: map[string]MediaType{
			"application/x-protobuf": {Schema: &Schema{Type: "string", Format: "binary"}},
		}},
		Responses: map[string]Resp{
			"200": {Description: "Accepted"},
		},
	})

	// 租户管理。
	p["/api/tenants"] = mk(&Op{
		Summary:     "List / create tenants",
		Tags:        []string{"admin"},
		OperationID: "tenantsList",
		Responses:  map[string]Resp{"200": {Description: "OK"}},
		Security:    []map[string][]string{{"ApiKey": {}}},
	})
	p["/api/keys"] = mk(&Op{
		Summary:     "List API keys",
		Tags:        []string{"admin"},
		OperationID: "keysList",
		Responses:  map[string]Resp{"200": {Description: "OK"}},
		Security:    []map[string][]string{{"ApiKey": {}}},
	})
	p["/api/snapshot"] = mk(&Op{
		Summary:     "Take engine snapshot",
		Tags:        []string{"observability"},
		OperationID: "snapshot",
		Responses:  map[string]Resp{"200": {Description: "OK"}},
		Security:    []map[string][]string{{"ApiKey": {}}},
	})

	// 副本。
	p["/replica/ack"] = mk(&Op{
		Summary:     "Follower offset ack",
		Tags:        []string{"admin"},
		OperationID: "replicaAck",
		RequestBody: &ReqBody{Required: true, Content: map[string]MediaType{
			"application/json": {Schema: &Schema{
				Type: "object",
				Properties: map[string]*Schema{
					"id":     {Type: "string"},
					"offset": {Type: "integer"},
				},
				Required: []string{"id", "offset"},
			}},
		}},
		Responses:  map[string]Resp{"200": {Description: "OK"}},
		Security:    []map[string][]string{{"Bearer": {}}},
	})
	p["/replica/state"] = mk(&Op{
		Summary:     "Cluster state",
		Tags:        []string{"admin"},
		OperationID: "replicaState",
		Responses:  map[string]Resp{"200": {Description: "OK"}},
		Security:    []map[string][]string{{"Bearer": {}}},
	})
	p["/replica/wal"] = mk(&Op{
		Summary:     "Stream WAL records (NDJSON)",
		Tags:        []string{"admin"},
		OperationID: "replicaWal",
		Parameters: []Param{{
			Name: "from", In: "query", Required: true,
			Schema: &Schema{Type: "integer"},
		}},
		Responses:  map[string]Resp{"200": {Description: "OK"}},
		Security:    []map[string][]string{{"Bearer": {}}},
	})

	return p
}

// Method 是一个微型 shim，用于保持 mk() 辅助函数紧凑。
func (o *Op) Method() string {
	switch {
	case o.OperationID == "queryInstant" || o.OperationID == "getSeries" || o.OperationID == "getRules" ||
		o.OperationID == "getHealth" || o.OperationID == "tenantsList" || o.OperationID == "keysList" ||
		o.OperationID == "snapshot" || o.OperationID == "replicaState" || o.OperationID == "replicaWal":
		return "GET"
	}
	return "POST"
}

func ref(s string) *Schema { return &Schema{Ref: s} }
