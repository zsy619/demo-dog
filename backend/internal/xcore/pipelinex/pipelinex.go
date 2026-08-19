// Package pipelinex 提供通用上下文驱动的 Pipeline。
package pipelinex

import "context"

// Stage 表示一个处理阶段，接收输入返回输出。
type Stage[T any] func(ctx context.Context, in T) (T, error)

// Pipeline 串联多个 Stage。
type Pipeline[T any] struct {
	stages []Stage[T]
}

// New 创建一个空 Pipeline。
func New[T any]() *Pipeline[T] {
	return &Pipeline[T]{}
}

// Add 追加一个 Stage。
func (p *Pipeline[T]) Add(s Stage[T]) *Pipeline[T] {
	p.stages = append(p.stages, s)
	return p
}

// Run 串行执行所有 Stage。
func (p *Pipeline[T]) Run(ctx context.Context, in T) (T, error) {
	cur := in
	for _, s := range p.stages {
		if err := ctx.Err(); err != nil {
			return cur, err
		}
		next, err := s(ctx, cur)
		if err != nil {
			return cur, err
		}
		cur = next
	}
	return cur, nil
}

// Len 返回 Stage 数。
func (p *Pipeline[T]) Len() int { return len(p.stages) }
