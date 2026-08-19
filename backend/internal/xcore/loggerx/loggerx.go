// Package loggerx 提供一个轻量的结构化日志接口。
// 仅打印到 stderr，可由调用方包装为业务日志实现。
package loggerx

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level 是日志级别。
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String 返回级别名称。
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	}
	return "unknown"
}

// Logger 是 JSON 结构化日志输出器。
type Logger struct {
	mu     sync.Mutex
	out    io.Writer
	level  Level
	fields map[string]any
}

// New 创建一个输出到 stderr 的 Logger。
func New() *Logger {
	return &Logger{out: os.Stderr, level: LevelInfo, fields: map[string]any{}}
}

// SetOutput 设置输出流。
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	l.out = w
	l.mu.Unlock()
}

// SetLevel 设置级别阈值。
func (l *Logger) SetLevel(lv Level) {
	l.mu.Lock()
	l.level = lv
	l.mu.Unlock()
}

// With 返回带默认字段的子 Logger（共享输出）。
func (l *Logger) With(fields map[string]any) *Logger {
	cp := make(map[string]any, len(l.fields)+len(fields))
	for k, v := range l.fields {
		cp[k] = v
	}
	for k, v := range fields {
		cp[k] = v
	}
	return &Logger{out: l.out, level: l.level, fields: cp}
}

// Debug 输出 debug 级。
func (l *Logger) Debug(msg string, fields map[string]any) { l.log(LevelDebug, msg, fields) }

// Info 输出 info 级。
func (l *Logger) Info(msg string, fields map[string]any) { l.log(LevelInfo, msg, fields) }

// Warn 输出 warn 级。
func (l *Logger) Warn(msg string, fields map[string]any) { l.log(LevelWarn, msg, fields) }

// Error 输出 error 级。
func (l *Logger) Error(msg string, fields map[string]any) { l.log(LevelError, msg, fields) }

func (l *Logger) log(lv Level, msg string, fields map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lv < l.level {
		return
	}
	out := map[string]any{
		"ts":    time.Now().Format(time.RFC3339Nano),
		"level": lv.String(),
		"msg":   msg,
	}
	for k, v := range l.fields {
		out[k] = v
	}
	for k, v := range fields {
		out[k] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(l.out, "{\"level\":\"error\",\"msg\":\"marshal\",\"err\":\"%s\"}\n", err)
		return
	}
	l.out.Write(b)
	l.out.Write([]byte("\n"))
}
