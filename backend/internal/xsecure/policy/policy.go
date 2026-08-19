// Package policy 提供一个简易的策略表达式求值器：
// 支持等值比较与 AND/OR/NOT。
package policy

import (
	"fmt"
	"strconv"
	"strings"
)

// Node 是 AST 节点。
type Node struct {
	Kind     Kind
	Children []*Node
	Key      string
	Value    any
}

// Kind 是节点种类。
type Kind int

const (
	KindAnd Kind = iota
	KindOr
	KindNot
	KindEq
	KindIn
)

// Parser 接收字段集合和策略字符串并返回 Node。
type Parser struct{}

// New 创建一个 Parser。
func New() *Parser { return &Parser{} }

// Parse 把策略表达式解析为 AST。
//
// 语法：
//   - `key == value`   等值
//   - `key in [v1, v2]`
//   - `expr AND expr`
//   - `expr OR expr`
//   - `NOT expr`
//   - `( expr )`
func (p *Parser) Parse(s string) (*Node, error) {
	toks, err := tokenize(s)
	if err != nil {
		return nil, err
	}
	p2 := &parser{toks: toks}
	n, err := p2.parseExpr()
	if err != nil {
		return nil, err
	}
	if p2.pos != len(p2.toks) {
		return nil, fmt.Errorf("policy: 末尾有多余 token")
	}
	return n, nil
}

// Eval 在 ctx 字段集合上求值。
func Eval(n *Node, ctx map[string]any) bool {
	if n == nil {
		return true
	}
	switch n.Kind {
	case KindAnd:
		for _, c := range n.Children {
			if !Eval(c, ctx) {
				return false
			}
		}
		return true
	case KindOr:
		for _, c := range n.Children {
			if Eval(c, ctx) {
				return true
			}
		}
		return false
	case KindNot:
		return !Eval(n.Children[0], ctx)
	case KindEq:
		v, ok := ctx[n.Key]
		if !ok {
			return false
		}
		return reflectEq(v, n.Value)
	case KindIn:
		v, ok := ctx[n.Key]
		if !ok {
			return false
		}
		for _, c := range n.Children {
			if reflectEq(v, c.Value) {
				return true
			}
		}
		return false
	}
	return false
}

func reflectEq(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

type tokenKind int

const (
	tIdent tokenKind = iota
	tString
	tNumber
	tLParen
	tRParen
	tEq
	tIn
	tAnd
	tOr
	tNot
	tComma
)

type token struct {
	k tokenKind
	s string
}
type kind = tokenKind

func tokenize(s string) ([]token, error) {
	var out []token
	i := 0
	for i < len(s) {
		c := s[i]
		switch c {
		case ' ', '\t', '\n':
			i++
			continue
		case '(':
			out = append(out, token{k: tLParen})
			i++
		case ')':
			out = append(out, token{k: tRParen})
			i++
		case ',':
			out = append(out, token{k: tComma})
			i++
		case '=':
			if i+1 < len(s) && s[i+1] == '=' {
				out = append(out, token{k: tEq})
				i += 2
			} else {
				return nil, fmt.Errorf("policy: 期望 ==")
			}
		case '"':
			j := i + 1
			var sb strings.Builder
			for j < len(s) && s[j] != '"' {
				sb.WriteByte(s[j])
				j++
			}
			if j >= len(s) {
				return nil, fmt.Errorf("policy: 未闭合字符串")
			}
			out = append(out, token{k: tString, s: sb.String()})
			i = j + 1
		case '[':
			out = append(out, token{k: tLParen})
			i++
		case ']':
			out = append(out, token{k: tRParen})
			i++
		default:
			if isLetter(c) {
				j := i
				for j < len(s) && (isLetter(s[j]) || isDigit(s[j])) {
					j++
				}
				w := s[i:j]
				switch w {
				case "AND":
					out = append(out, token{k: tAnd})
				case "OR":
					out = append(out, token{k: tOr})
				case "NOT":
					out = append(out, token{k: tNot})
				case "in":
					out = append(out, token{k: tIn})
				default:
					out = append(out, token{k: tIdent, s: w})
				}
				i = j
			} else if isDigit(c) || c == '-' {
				j := i
				for j < len(s) && (isDigit(s[j]) || s[j] == '.') {
					j++
				}
				out = append(out, token{k: tNumber, s: s[i:j]})
				i = j
			} else {
				return nil, fmt.Errorf("policy: 非法字符 %c", c)
			}
		}
	}
	return out, nil
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peek() (token, bool) {
	if p.pos >= len(p.toks) {
		return token{}, false
	}
	return p.toks[p.pos], true
}

func (p *parser) consume() (token, bool) {
	t, ok := p.peek()
	if ok {
		p.pos++
	}
	return t, ok
}

func (p *parser) parseExpr() (*Node, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (*Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for {
		t, ok := p.peek()
		if !ok || t.k != tOr {
			break
		}
		p.consume()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &Node{Kind: KindOr, Children: []*Node{left, right}}
	}
	return left, nil
}

func (p *parser) parseAnd() (*Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t, ok := p.peek()
		if !ok || t.k != tAnd {
			break
		}
		p.consume()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &Node{Kind: KindAnd, Children: []*Node{left, right}}
	}
	return left, nil
}

func (p *parser) parseUnary() (*Node, error) {
	t, ok := p.peek()
	if ok && t.k == tNot {
		p.consume()
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Node{Kind: KindNot, Children: []*Node{inner}}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (*Node, error) {
	t, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("policy: 意外结束")
	}
	if t.k == tLParen {
		p.consume()
		n, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if c, ok := p.peek(); !ok || c.k != tRParen {
			return nil, fmt.Errorf("policy: 期望 )")
		}
		p.consume()
		return n, nil
	}
	if t.k != tIdent {
		return nil, fmt.Errorf("policy: 期望字段名")
	}
	key := t.s
	p.consume()
	next, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("policy: 字段后缺少运算符")
	}
	switch next.k {
	case tEq:
		p.consume()
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return &Node{Kind: KindEq, Key: key, Value: v}, nil
	case tIn:
		p.consume()
		if c, ok := p.peek(); !ok || c.k != tLParen {
			return nil, fmt.Errorf("policy: in 后缺少 [")
		}
		p.consume()
		var vs []*Node
		for {
			v, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			vs = append(vs, &Node{Value: v})
			if c, ok := p.peek(); !ok || c.k != tComma {
				break
			}
			p.consume()
		}
		if c, ok := p.peek(); !ok || c.k != tRParen {
			return nil, fmt.Errorf("policy: 期望 ]")
		}
		p.consume()
		return &Node{Kind: KindIn, Key: key, Children: vs}, nil
	}
	return nil, fmt.Errorf("policy: 未知运算符")
}

func (p *parser) parseValue() (any, error) {
	t, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("policy: 期望值")
	}
	switch t.k {
	case tString:
		p.consume()
		return t.s, nil
	case tNumber:
		p.consume()
		if strings.Contains(t.s, ".") {
			return strconv.ParseFloat(t.s, 64)
		}
		return strconv.ParseInt(t.s, 10, 64)
	default:
		return nil, fmt.Errorf("policy: 值类型不支持")
	}
}
