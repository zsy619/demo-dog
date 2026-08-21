package logx

// stats.go:Stats 类型与 Logger.Stats()。

// Stats 表示写入端的计数器(尽力而为)。
type Stats struct {
	MinLevel Level `json:"min_level"` // 最小输出级别
	Fields   int    `json:"fields"`    // 派生 Logger 的固定字段数
}

// Stats 返回 Logger 配置的快照。
func (l *Logger) Stats() Stats {
	return Stats{MinLevel: l.Level(), Fields: len(l.fields)}
}
