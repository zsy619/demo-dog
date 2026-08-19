// Package metricsx 的浮点原子转换工具。
package metricsx

import "math"

func floatToBits(f float64) uint64 {
	return math.Float64bits(f)
}

func bitsToFloat(b uint64) float64 {
	return math.Float64frombits(b)
}
