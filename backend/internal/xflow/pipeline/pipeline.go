// Package pipeline 提供一组按顺序串联的处理函数。
package pipeline

import "context"

// Step 是一个可处理输入并返回新值或错误的步骤。
type Step[T any] func(ctx context.Context, v T) (T, error)

// Pipe 是按顺序串联的步骤集合。
type Pipe[T any] struct {
	steps []Step[T]
}

// New 创建一个空管道。
func New[T any]() *Pipe[T] {
	return &Pipe[T]{}
}

// Append 添加一个步骤。
func (p *Pipe[T]) Append(s Step[T]) *Pipe[T] {
	p.steps = append(p.steps, s)
	return p
}

// Run 按顺序执行所有步骤，任何一步返回错误立刻中止。
func (p *Pipe[T]) Run(ctx context.Context, v T) (T, error) {
	var err error
	for _, s := range p.steps {
		v, err = s(ctx, v)
		if err != nil {
			return v, err
		}
	}
	return v, nil
}

// Len 返回步骤数。
func (p *Pipe[T]) Len() int { return len(p.steps) }
