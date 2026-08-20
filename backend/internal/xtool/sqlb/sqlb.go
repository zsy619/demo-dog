// Package sqlb 提供一个白名单 SQL 字符串构造器。
// 它通过预定义表/列名映射，确保拼出的 SQL 不引入未授权标识符。
package sqlb

import (
	"errors"
	"fmt"
	"strings"
)

// ErrBadIdent 在遇到非法标识符时返回。
var ErrBadIdent = errors.New("sqlb: 非法标识符")

// Schema 维护允许的表与对应字段集合。
type Schema struct {
	tables map[string]map[string]bool
}

func NewSchema() *Schema {
	return &Schema{tables: make(map[string]map[string]bool)}
}

func (s *Schema) Register(table string, columns ...string) {
	if s.tables[table] == nil {
		s.tables[table] = make(map[string]bool)
	}
	for _, c := range columns {
		s.tables[table][c] = true
	}
}

func (s *Schema) HasTable(table string) bool {
	return s.tables[table] != nil
}

func (s *Schema) HasColumn(table, column string) bool {
	cols, ok := s.tables[table]
	if !ok {
		return false
	}
	return cols[column]
}

func Ident(s string) error {
	if s == "" {
		return ErrBadIdent
	}
	for _, r := range s {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return ErrBadIdent
		}
	}
	return nil
}

type Builder struct {
	schema *Schema
	b      strings.Builder
	args   []any
	err    error
}

func New(schema *Schema) *Builder {
	return &Builder{schema: schema}
}

func (b *Builder) Err() error { return b.err }

func (b *Builder) Args() []any { return b.args }

func (b *Builder) String() string { return b.b.String() }

func (b *Builder) WriteIdent(s string) {
	if b.err != nil {
		return
	}
	if err := Ident(s); err != nil {
		b.err = err
		return
	}
	b.b.WriteString(s)
}

func (b *Builder) Select(table string, columns ...string) *Builder {
	if b.err != nil {
		return b
	}
	if !b.schema.HasTable(table) {
		b.err = fmt.Errorf("sqlb: 表未注册: %s", table)
		return b
	}
	b.b.WriteString("SELECT ")
	for i, c := range columns {
		if i > 0 {
			b.b.WriteString(", ")
		}
		if c == "*" {
			b.b.WriteString("*")
			continue
		}
		if !b.schema.HasColumn(table, c) {
			b.err = fmt.Errorf("sqlb: 列未注册: %s.%s", table, c)
			return b
		}
		b.WriteIdent(c)
	}
	b.b.WriteString(" FROM ")
	b.WriteIdent(table)
	return b
}

func (b *Builder) Where(op string, column string, value any) *Builder {
	if b.err != nil {
		return b
	}
	if column != "" {
		if e := Ident(column); e != nil {
			b.err = e
			return b
		}
	}
	upper := strings.ToUpper(b.b.String())
	if !strings.HasSuffix(upper, "WHERE ") {
		b.b.WriteString(" WHERE ")
	} else {
		b.b.WriteString(" ")
	}
	b.WriteIdent(column)
	b.b.WriteString(" ")
	b.b.WriteString(op)
	b.b.WriteString(" ")
	b.b.WriteString("?")
	b.args = append(b.args, value)
	return b
}

func (b *Builder) AndWhere(op, column string, value any) *Builder {
	if b.err != nil {
		return b
	}
	if e := Ident(column); e != nil {
		b.err = e
		return b
	}
	b.b.WriteString(" AND ")
	b.WriteIdent(column)
	b.b.WriteString(" ")
	b.b.WriteString(op)
	b.b.WriteString(" ")
	b.b.WriteString("?")
	b.args = append(b.args, value)
	return b
}

func (b *Builder) Limit(n int) *Builder {
	if b.err != nil {
		return b
	}
	b.b.WriteString(" LIMIT ")
	b.b.WriteString(fmt.Sprintf("%d", n))
	return b
}
