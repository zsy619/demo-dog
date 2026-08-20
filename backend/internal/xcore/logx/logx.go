// Package logx 结构化日志接口：键值对风格的轻量日志输出。
package logx

// JSON structured logger.
//
// Single-line JSON output: ts, level, msg, plus caller-attached
// fields. Designed for ship-to-Loki / ship-to-Elastic use
// cases. Stdlib-only, no third-party deps.
//
// Logger is safe for concurrent use. The With* family
// returns a derived logger that always emits those fields.
// The Err / Bool / Int / Str / Dur / Time helpers are chainable.
//
// Levels: trace < debug < info < warn < error < fatal. The
// minimum level is settable per logger; the package level
// controls the default minimum for new loggers.

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

// Level is a log level.
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

// ParseLevel maps a string to a Level. Empty -> LevelInfo.
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

// Encoder serialises a record to bytes.
type Encoder interface {
	Encode(*Record) ([]byte, error)
}

// Record is one log event.
type Record struct {
	Time    time.Time
	Level   Level
	Message string
	Fields  []Field
}

// Field is one key/value pair.
type Field struct {
	Key   string
	Value any
}

// Field constructors.
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

// JSONEncoder is the default encoder.
type JSONEncoder struct{}

// Encode marshals r to a single JSON line (no trailing
// newline; the writer adds one).
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

// Logger writes records to an io.Writer with a minimum level.
type Logger struct {
	mu      sync.Mutex
	w       io.Writer
	enc     Encoder
	min     atomic.Int32
	fields  []Field
	now     func() time.Time
	pool    sync.Pool
}

// New returns a Logger that writes to w with the given level.
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

// Default is a process-global logger that writes to stdout at
// info level.
var Default = New(os.Stdout, LevelInfo)

// SetLevel changes the minimum level.
func (l *Logger) SetLevel(level Level) { l.min.Store(int32(level)) }

// Level returns the current minimum level.
func (l *Logger) Level() Level { return Level(l.min.Load()) }

// With returns a derived logger with extra fields.
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

// WithTime overrides the time source (testing).
func (l *Logger) WithTime(now func() time.Time) *Logger {
	l.now = now
	return l
}

// WithEncoder swaps the encoder.
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

// Trace, Debug, Info, Warn, Error, Fatal emit a record at the
// named level. Fatal does not os.Exit (preserves testability);
// the program can wire that policy in cmd/dog-collector.
func (l *Logger) Trace(msg string, f ...Field) { l.log(LevelTrace, msg, f) }
func (l *Logger) Debug(msg string, f ...Field) { l.log(LevelDebug, msg, f) }
func (l *Logger) Info(msg string, f ...Field)  { l.log(LevelInfo, msg, f) }
func (l *Logger) Warn(msg string, f ...Field)  { l.log(LevelWarn, msg, f) }
func (l *Logger) Error(msg string, f ...Field) { l.log(LevelError, msg, f) }
func (l *Logger) Fatal(msg string, f ...Field) { l.log(LevelFatal, msg, f) }

// Caller returns the function name + line of the immediate
// caller (skip=1 inside this package).
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

// Stats returns counters for the writer side (best-effort).
type Stats struct {
	MinLevel Level `json:"min_level"`
	Fields   int  `json:"fields"`
}

// Stats returns the logger configuration snapshot.
func (l *Logger) Stats() Stats {
	return Stats{MinLevel: l.Level(), Fields: len(l.fields)}
}
