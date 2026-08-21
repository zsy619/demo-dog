package logx

// record.go:Record 类型与 Encoder 接口、默认 JSONEncoder。
//
// Record 是一条可编码的日志事件;Encoder 把 Record 序列化为字节。
// 内置 JSONEncoder 输出单行 JSON。

import (
	"encoding/json"
	"time"
)

// Encoder 将一条日志记录序列化为字节。
type Encoder interface {
	Encode(*Record) ([]byte, error)
}

// Record 表示一条日志事件。
type Record struct {
	Time    time.Time // 时间戳
	Level   Level     // 级别
	Message string    // 消息
	Fields  []Field   // 附加字段
}

// JSONEncoder 是默认的编码器。
type JSONEncoder struct{}

// Encode 将 r 编码为单行 JSON(不含结尾换行,由写入器补上)。
func (JSONEncoder) Encode(r *Record) ([]byte, error) {
	m := make(map[string]any, len(r.Fields)+3)
	m["ts"] = r.Time.UTC().Format(time.RFC3339Nano)
	m["level"] = r.Level.String()
	m["msg"] = r.Message
	for _, f := range r.Fields {
		m[f.Key] = f.Value
	}
	return json.Marshal(m)
}
