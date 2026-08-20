package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRun_Empty(t *testing.T) {
	p := New[int]("empty")
	if _, err := p.Run(context.Background(), 1); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_SingleStage(t *testing.T) {
	p := New[int]("p", func(ctx context.Context, in int) (int, error) {
		return in + 1, nil
	})
	v, err := p.Run(context.Background(), 1)
	if err != nil || v != 2 {
		t.Fatal(err, v)
	}
}

func TestRun_MultiStage(t *testing.T) {
	p := New[int]("p",
		func(ctx context.Context, in int) (int, error) { return in + 1, nil },
		func(ctx context.Context, in int) (int, error) { return in * 2, nil },
		func(ctx context.Context, in int) (int, error) { return in + 100, nil },
	)
	v, err := p.Run(context.Background(), 1)
	if err != nil || v != 104 {
		t.Fatalf("v=%d err=%v", v, err)
	}
}

func TestRun_StageError(t *testing.T) {
	p := New[int]("p",
		func(ctx context.Context, in int) (int, error) { return in, nil },
		func(ctx context.Context, in int) (int, error) { return 0, errors.New("bad") },
		func(ctx context.Context, in int) (int, error) { return in, nil },
	)
	_, err := p.Run(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_ContextCancel(t *testing.T) {
	p := New[int]("p", func(ctx context.Context, in int) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Run(ctx, 1); err == nil {
		t.Fatal("expected error")
	}
}

func TestFork(t *testing.T) {
	branches := map[string]Stage[int]{
		"a": func(ctx context.Context, in int) (int, error) { return in + 1, nil },
		"b": func(ctx context.Context, in int) (int, error) { return in * 2, nil },
		"c": func(ctx context.Context, in int) (int, error) { return 0, errors.New("x") },
	}
	results := Fork(context.Background(), branches, 5)
	if len(results) != 3 {
		t.Fatal("count")
	}
	got := map[string]ForkResult[int]{}
	for _, r := range results {
		got[r.Name] = r
	}
	if got["a"].Value != 6 {
		t.Fatal("a")
	}
	if got["b"].Value != 10 {
		t.Fatal("b")
	}
	if got["c"].Err == nil {
		t.Fatal("c err")
	}
}

func TestFork_FirstError(t *testing.T) {
	results := []ForkResult[int]{
		{Name: "a", Err: nil},
		{Name: "b", Err: errors.New("boom")},
		{Name: "c", Err: nil},
	}
	if err := FirstError(results); err == nil || err.Error() != "boom" {
		t.Fatal(err)
	}
	if err := FirstError(results[:1]); err != nil {
		t.Fatal("no error expected")
	}
}

func TestFork_ContextCancel(t *testing.T) {
	branches := map[string]Stage[int]{
		"a": func(ctx context.Context, in int) (int, error) {
			<-ctx.Done()
			return 0, ctx.Err()
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results := Fork(ctx, branches, 1)
	if len(results) != 1 || results[0].Err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_Deadline(t *testing.T) {
	p := New[int]("p", func(ctx context.Context, in int) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return in, nil
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := p.Run(ctx, 1); err == nil {
		t.Fatal("expected deadline error")
	}
}
