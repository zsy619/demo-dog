// Package logx 结构化日志接口:键值对风格的轻量日志输出。
//
// JSON 结构化日志器。
//
// 单行 JSON 输出:ts、level、msg,再加上调用方附加的字段。
// 面向 ship-to-Loki / ship-to-Elastic 等场景设计。
// 仅使用标准库,不引入第三方依赖。
//
// Logger 支持并发安全使用。With* 系列方法返回一个派生 Logger,
// 它会始终输出这些字段。Err / Bool / Int / Str / Dur / Time 辅助函数均可链式调用。
//
// 日志级别:trace < debug < info < warn < error < fatal。
// 最小级别可在每个 Logger 上单独设置;包级最小级别则控制新 Logger 的默认值。
//
// 文件职责拆分:
//   - level.go   Level 类型 + 字符串互转
//   - field.go   Field 类型 + 构造函数
//   - record.go  Record + Encoder 接口 + JSONEncoder
//   - logger.go  Logger 主体与日志输出
//   - caller.go  Caller + trimPath
//   - stats.go   Stats + Stats()
package logx

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

// String 返回 Level 的小写字符串形式。
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
