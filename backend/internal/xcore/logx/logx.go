// Package logx 结构化日志接口：键值对风格的轻量日志输出。
package logx

// JSON 结构化日志器。
//
// 单行 JSON 输出：ts、level、msg，再加上调用方附加的字段。
// 面向 ship-to-Loki / ship-to-Elastic 等场景设计。
// 仅使用标准库，不引入第三方依赖。
//
// Logger 支持并发安全使用。With* 系列方法返回一个派生 Logger，
// 它会始终输出这些字段。Err / Bool / Int / Str / Dur / Time 辅助函数均可链式调用。
//
// 日志级别：trace < debug < info < warn < error < fatal。
// 最小级别可在每个 Logger 上单独设置；包级最小级别则控制新 Logger 的默认值。

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Level 表示日志级别。
type Level int

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "trace"
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	}
	return "unknown"
}

// ParseLevel 将字符串映射为 Level。空字符串映射为 LevelInfo。
func ParseLevel(s string) Level {
	switch s {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "info", "":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	}
	return LevelInfo
}

// Encoder 将一条日志记录序列化为字节。
type Encoder interface {
	Encode(*Record) ([]byte, error)
}

// Record 表示一条日志事件。
type Record struct {
	Time    time.Time
	Level   Level
	Message string
	Fields  []Field
}

// Field 表示一对键值。
type Field struct {
	Key   string
	Value any
}

// Field 构造函数集合。
func Str(k, v string) Field        { return Field{k, v} }
func Int(k string, v int) Field    { return Field{k, v} }
func Int64(k string, v int64) Field { return Field{k, v} }
func Bool(k string, v bool) Field  { return Field{k, v} }
func Dur(k string, v time.Duration) Field { return Field{k, v.String()} }
func Err(v error) Field            {
	if v == nil {
		return Field{"error", nil}
	}
	return Field{"error", v.Error()}
}
func Time(k string, v time.Time) Field {
	return Field{k, v.UTC().Format(time.RFC3339Nano)}
}
func Any(k string, v any) Field { return Field{k, v} }

// JSONEncoder 是默认的编码器。
type JSONEncoder struct{}

// Encode 将 r 编码为单行 JSON（不含结尾换行，由写入器补上）。
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

// Logger 将记录写入 io.Writer，并按最小级别过滤。
type Logger struct {
	mu      sync.Mutex
	w       io.Writer
	enc     Encoder
	min     atomic.Int32
	fields  []Field
	now     func() time.Time
	pool    sync.Pool
}

// New 返回一个将日志写入 w 的 Logger，并指定最小级别。
func New(w io.Writer, level Level) *Logger {
	l := &Logger{
		w:   w,
		enc: JSONEncoder{},
		now: time.Now,
	}
	l.min.Store(int32(level))
	l.pool.New = func() any { return &Record{} }
	return l
}

// Default 是进程级全局 Logger，向 stdout 输出 info 级别日志。
var Default = New(os.Stdout, LevelInfo)

// SetLevel 改变最小级别。
func (l *Logger) SetLevel(level Level) { l.min.Store(int32(level)) }

// Level 返回当前最小级别。
func (l *Logger) Level() Level { return Level(l.min.Load()) }

// With 返回一个附带额外字段的派生 Logger。
func (l *Logger) With(fields ...Field) *Logger {
	merged := make([]Field, 0, len(l.fields)+len(fields))
	merged = append(merged, l.fields...)
	merged = append(merged, fields...)
	nl := &Logger{
		w:   l.w,
		enc: l.enc,
		now: l.now,
	}
	nl.min.Store(l.min.Load())
	nl.fields = merged
	nl.pool.New = func() any { return &Record{} }
	return nl
}

// WithTime 覆盖时间源（用于测试）。
func (l *Logger) WithTime(now func() time.Time) *Logger {
	l.now = now
	return l
}

// WithEncoder 替换编码器。
func (l *Logger) WithEncoder(enc Encoder) *Logger {
	l.enc = enc
	return l
}

func (l *Logger) enabled(level Level) bool {
	return int32(level) >= l.min.Load()
}

func (l *Logger) log(level Level, msg string, fields []Field) {
	if !l.enabled(level) {
		return
	}
	r := l.pool.Get().(*Record)
	r.Time = l.now()
	r.Level = level
	r.Message = msg
	r.Fields = append(r.Fields[:0], l.fields...)
	r.Fields = append(r.Fields, fields...)
	buf, err := l.enc.Encode(r)
	l.pool.Put(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logx encode: %v\n", err)
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	buf = append(buf, '\n')
	l.w.Write(buf)
}

// Trace、Debug、Info、Warn、Error、Fatal 在对应级别输出一条记录。
// Fatal 不会调用 os.Exit（以保留可测试性），
// 该策略可在 cmd/dog-collector 中按需接入。
func (l *Logger) Trace(msg string, f ...Field) { l.log(LevelTrace, msg, f) }
func (l *Logger) Debug(msg string, f ...Field) { l.log(LevelDebug, msg, f) }
func (l *Logger) Info(msg string, f ...Field)  { l.log(LevelInfo, msg, f) }
func (l *Logger) Warn(msg string, f ...Field)  { l.log(LevelWarn, msg, f) }
func (l *Logger) Error(msg string, f ...Field) { l.log(LevelError, msg, f) }
func (l *Logger) Fatal(msg string, f ...Field) { l.log(LevelFatal, msg, f) }

// Caller 返回直接调用方的函数名与行号（在本包内 skip=1）。
func Caller(skip int) Field {
	pc, _, _, ok := runtime.Caller(skip + 1)
	if !ok {
		return Str("caller", "unknown")
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return Str("caller", "unknown")
	}
	file, line := fn.FileLine(pc)
	return Str("caller", fmt.Sprintf("%s:%d", trimPath(file), line))
}

func trimPath(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

// Stats 表示写入端的计数器（尽力而为）。
type Stats struct {
	MinLevel Level `json:"min_level"`
	Fields   int  `json:"fields"`
}

// Stats 返回 Logger 配置的快照。
func (l *Logger) Stats() Stats {
	return Stats{MinLevel: l.Level(), Fields: len(l.fields)}
}
