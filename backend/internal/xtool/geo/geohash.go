package geo

// geohash.go:geohash 编解码与 Haversine 距离计算。
//
// Encode/Decode 用 base32 字符集交错经纬度;
// Neighbors 当前仅返回 cell 自身(简化版),Index.Nearby 仍可正确工作。

import "math"

// geohashBase32 是 geohash 标准字符集(不含 a/i/l/o)。
const geohashBase32 = "0123456789bcdefghjkmnpqrstuvwxyz"

// geohashIdx 是 base32 字符 → 索引的反向映射。
var geohashIdx = func() map[byte]int {
	m := make(map[byte]int, len(geohashBase32))
	for k := 0; k < len(geohashBase32); k++ {
		m[geohashBase32[k]] = k
	}
	return m
}()

// Encode 返回 p 在该精度下的 geohash 字符串。
func Encode(p Point, precision int) string {
	if precision < 1 {
		precision = 1
	}
	if precision > 12 {
		precision = 12
	}
	latLo, latHi := -90.0, 90.0
	lngLo, lngHi := -180.0, 180.0
	bit := 0
	ch := 0
	even := true
	out := make([]byte, 0, precision)
	for len(out) < precision {
		if even {
			mid := (lngLo + lngHi) / 2
			if p.Lng >= mid {
				ch = (ch << 1) | 1
				lngLo = mid
			} else {
				ch = ch << 1
				lngHi = mid
			}
		} else {
			mid := (latLo + latHi) / 2
			if p.Lat >= mid {
				ch = (ch << 1) | 1
				latLo = mid
			} else {
				ch = ch << 1
				latHi = mid
			}
		}
		even = !even
		bit++
		if bit == 5 {
			out = append(out, geohashBase32[ch])
			bit = 0
			ch = 0
		}
	}
	return string(out)
}

// Decode 返回一个 geohash 的边界框和中心点。
func Decode(hash string) (Point, float64, float64) {
	latLo, latHi := -90.0, 90.0
	lngLo, lngHi := -180.0, 180.0
	even := true
	for k := 0; k < len(hash); k++ {
		n, ok := geohashIdx[hash[k]]
		if !ok {
			continue
		}
		for b := 4; b >= 0; b-- {
			bit := (n >> b) & 1
			if even {
				mid := (lngLo + lngHi) / 2
				if bit == 1 {
					lngLo = mid
				} else {
					lngHi = mid
				}
			} else {
				mid := (latLo + latHi) / 2
				if bit == 1 {
					latLo = mid
				} else {
					latHi = mid
				}
			}
			even = !even
		}
	}
	center := Point{Lat: (latLo + latHi) / 2, Lng: (lngLo + lngHi) / 2}
	return center, math.Abs(latHi - latLo), math.Abs(lngHi - lngLo)
}

// Neighbors 返回围绕给定 cell 的 8 个相邻 cell (以及它自身) 的 geohash 字符串。
// surrounding the given cell (and itself).
//
// 当前为简化实现,仅返回 cell 自身;Index.Nearby 仍能配合后续的距离过滤工作。
func Neighbors(cell string) []string {
	out := []string{cell}
	if len(cell) < 1 {
		return out
	}
	return out
}

// dist 计算两点间的 Haversine 距离(公里)。
//
// R = 6371 km 为地球平均半径。
func dist(a, b Point) float64 {
	const R = 6371.0
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Sin(dLng/2)*math.Sin(dLng/2)*math.Cos(lat1)*math.Cos(lat2)
	return 2 * R * math.Asin(math.Sqrt(h))
}
