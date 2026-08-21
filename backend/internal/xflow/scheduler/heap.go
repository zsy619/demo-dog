package scheduler

// heap.go:任务堆实现。
//
// taskHeap 实现 container/heap.Interface,按 NextRun 升序排列。

// taskItem 是堆中的一项。
type taskItem struct {
	task *Task // 指向 Task 引用
	idx  int   // 在堆中的索引,便于 O(1) 更新
}

// taskHeap 是一个最小堆(按 NextRun 排序)。
type taskHeap []*taskItem

// Len 返回堆大小。
func (h taskHeap) Len() int { return len(h) }

// Less 按 NextRun 升序排列。
func (h taskHeap) Less(i, j int) bool { return h[i].task.NextRun.Before(h[j].task.NextRun) }

// Swap 交换元素并更新 idx。
func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].idx = i
	h[j].idx = j
}

// Push 追加一个 taskItem(由 heap.Push 调用)。
func (h *taskHeap) Push(x any) {
	t := x.(*taskItem)
	t.idx = len(*h)
	*h = append(*h, t)
}

// Pop 弹出最后一个元素(由 heap.Pop 调用)。
func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return x
}
