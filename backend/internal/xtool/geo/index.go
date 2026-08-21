package geo

// index.go:Index(基于 geohash 的内存网格索引)。
//
// Index 用 cell → id → feature 嵌套 map 加速半径查询;
// Nearby 先取 9 个邻接 cell,再用 Haversine 做精确距离过滤。

import "sync"

// Index 是一个内存中的 geohash 网格。
type Index struct {
	mu   sync.RWMutex             // 保护 grid
	prec int                      // geohash 精度
	grid map[string]map[string]Feature // cell → id → feature
}

// New 返回具有给定精度(1-12)的 Index。
//
// 越界精度自动归一到 6。
func New(precision int) *Index {
	if precision < 1 || precision > 12 {
		precision = 6
	}
	return &Index{prec: precision, grid: make(map[string]map[string]Feature)}
}

// Encode 返回 p 在 index 精度下的 geohash 字符串。
func (i *Index) Encode(p Point) string {
	return Encode(p, i.prec)
}

// Add 插入一个 feature。
func (i *Index) Add(f Feature) {
	cell := i.Encode(f.Loc)
	i.mu.Lock()
	if _, ok := i.grid[cell]; !ok {
		i.grid[cell] = make(map[string]Feature)
	}
	i.grid[cell][f.ID] = f
	i.mu.Unlock()
}

// Remove 按 ID 移除一个 feature。
//
// 会扫描所有 cell,删除空 cell。
func (i *Index) Remove(id string) {
	i.mu.Lock()
	for cell, m := range i.grid {
		if _, ok := m[id]; ok {
			delete(m, id)
			if len(m) == 0 {
				delete(i.grid, cell)
			}
		}
	}
	i.mu.Unlock()
}

// Nearby 返回 encoded cell 与查询 cell 或其 8 个邻居相匹配的 features。
// query cell or its 8 neighbors. Fine-grain distance is
// 细粒度的距离通过 Haversine 计算。
func (i *Index) Nearby(q Point, radiusKm float64) []Feature {
	cell := i.Encode(q)
	cells := Neighbors(cell)
	i.mu.RLock()
	seen := make(map[string]Feature)
	for _, c := range cells {
		for _, f := range i.grid[c] {
			seen[f.ID] = f
		}
	}
	i.mu.RUnlock()
	out := make([]Feature, 0, len(seen))
	for _, f := range seen {
		if dist(q, f.Loc) <= radiusKm {
			out = append(out, f)
		}
	}
	return out
}

// Len 返回已索引 feature 的数量。
func (i *Index) Len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	n := 0
	for _, m := range i.grid {
		n += len(m)
	}
	return n
}
