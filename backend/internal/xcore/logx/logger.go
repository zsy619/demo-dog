package logx

// logger.go:Logger 主体与所有输出方法。
//
// Logger 通过 mu 串行化输出,enc 序列化 Record,min 过滤级别。
// With* 系列方法返回派生 Logger,共享父 Logger 的 io.Writer。

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Logger 将记录写入 io.Writer,并按最小级别过滤。
type Logger struct {
	mu     sync.Mutex   // 串行化 w.Write
	w      io.Writer    // 输出目标
	enc    Encoder      // 序列化器
	min    atomic.Int32 // 最小输出级别
	fields []Field      // 派生字段
	now    func() time.Time // 时间源(测试可注入)
	pool   sync.Pool    // Record 对象池
}

// New 返回一个将日志写入 w 的 Logger,并指定最小级别。
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

// Default 是进程级全局 Logger,向 stdout 输出 info 级别日志。
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

// WithTime 覆盖时间源(用于测试)。
func (l *Logger) WithTime(now func() time.Time) *Logger {
	l.now = now
	return l
}

// WithEncoder 替换编码器。
func (l *Logger) WithEncoder(enc Encoder) *Logger {
	l.enc = enc
	return l
}

// enabled 判断给定级别是否会被输出。
func (l *Logger) enabled(level Level) bool {
	return int32(level) >= l.min.Load()
}

// log 是核心输出路径:按级别过滤,池化 Record,串行化写入。
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
// Fatal 不会调用 os.Exit(以保留可测试性),
// 该策略可在 cmd/dog-collector 中按需接入。
func (l *Logger) Trace(msg string, f ...Field) { l.log(LevelTrace, msg, f) }
func (l *Logger) Debug(msg string, f ...Field) { l.log(LevelDebug, msg, f) }
func (l *Logger) Info(msg string, f ...Field)  { l.log(LevelInfo, msg, f) }
func (l *Logger) Warn(msg string, f ...Field)  { l.log(LevelWarn, msg, f) }
func (l *Logger) Error(msg string, f ...Field) { l.log(LevelError, msg, f) }
func (l *Logger) Fatal(msg string, f ...Field) { l.log(LevelFatal, msg, f) }
