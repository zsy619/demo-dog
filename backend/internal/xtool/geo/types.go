// Package geo 地理编码:地址/经纬度转换 + 距离计算。
//
// 本包实现 geohash 网格索引与 Haversine 距离计算:
//   - types.go   Point + Feature 类型定义
//   - index.go   Index(网格索引主体)
//   - geohash.go geohash 编解码与距离计算
package geo

// Point 是一个经纬度坐标。
type Point struct {
	Lat float64 // 纬度(-90..90)
	Lng float64 // 经度(-180..180)
}

// Feature 是一个带可选元数据的命名点。
type Feature struct {
	ID  string // 唯一 ID
	Loc Point  // 坐标
	Tag string // 可选标签/描述
}
