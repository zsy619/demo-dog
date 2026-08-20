// Package tarx tar 扩展:流式打包/解包,避免全部加载。
//
// 本包按类型拆分到多个文件:
//   - header.go  Header + Parse/Build + 私有辅助(checksum / cString / padOctal)
//   - reader.go  Entry + Reader
//   - writer.go  Writer
package tarx

import (
	"bytes"
	"errors"
	"strconv"
)

// headerSize 是 ustar header 的固定字节数(512)。
const headerSize = 512

// ErrTruncated 输入不是 512 字节整数倍时返回。
var ErrTruncated = errors.New("tar truncated")

// ErrBadHeader magic 不匹配时返回。
var ErrBadHeader = errors.New("bad header")

// ErrBadChecksum 校验和不匹配时返回。
var ErrBadChecksum = errors.New("bad checksum")

// Header 是一个 ustar header (512 字节)。
type Header struct {
	Name     string // 文件名
	Mode     int64  // 权限位
	UID      int64  // 用户 ID
	GID      int64  // 组 ID
	Size     int64  // 文件大小
	ModTime  int64  // 修改时间(unix seconds)
	Chksum   int64  // 头部校验和
	TypeFlag byte   // 类型标记
	LinkName string // 链接目标
	Magic    string // ustar magic
	Version  string // ustar 版本
	UName    string // 用户名
	GName    string // 组名
	DevMajor int64  // 主设备号
	DevMinor int64  // 次设备号
	Prefix   string // 长文件名前缀
}

// ParseHeader 解析一个 512 字节的 ustar header。
//
// 允许 magic 不匹配的空 name header;否则会校验和。
func ParseHeader(b []byte) (*Header, error) {
	if len(b) < headerSize {
		return nil, ErrTruncated
	}
	name := cString(b[0:100])
	mode, _ := strconv.ParseInt(cString(b[100:108]), 8, 64)
	uid, _ := strconv.ParseInt(cString(b[108:116]), 8, 64)
	gid, _ := strconv.ParseInt(cString(b[116:124]), 8, 64)
	size, _ := strconv.ParseInt(cString(b[124:136]), 8, 64)
	mtime, _ := strconv.ParseInt(cString(b[136:148]), 8, 64)
	chksum, _ := strconv.ParseInt(cString(b[148:156]), 8, 64)
	typeFlag := b[156]
	link := cString(b[157:257])
	magic := cString(b[257:263])
	version := cString(b[263:265])
	uname := cString(b[265:297])
	gname := cString(b[297:329])
	devMajor, _ := strconv.ParseInt(cString(b[329:337]), 8, 64)
	devMinor, _ := strconv.ParseInt(cString(b[337:345]), 8, 64)
	prefix := cString(b[345:512])
	h := &Header{
		Name: name, Mode: mode, UID: uid, GID: gid, Size: size,
		ModTime: mtime, Chksum: chksum, TypeFlag: typeFlag,
		LinkName: link, Magic: magic, Version: version,
		UName: uname, GName: gname, DevMajor: devMajor, DevMinor: devMinor,
		Prefix: prefix,
	}
	if h.TypeFlag == '0' {
		h.TypeFlag = 'N' // normalize to 'regular file'
	}
	if magic != "ustar" && magic != "ustar  " {
		if name != "" {
			return h, nil
		}
		return h, ErrBadHeader
	}
	sum := checksum(b)
	if sum != chksum {
		return h, ErrBadChecksum
	}
	return h, nil
}

// BuildHeader 渲染一个 512 字节的 header。
//
// 内部重新计算 checksum 并写入 148:156;同时把 h.Chksum 同步为新值。
func BuildHeader(h *Header) []byte {
	out := make([]byte, headerSize)
	copy(out[0:100], h.Name)
	copy(out[100:108], padOctal(h.Mode, 7))
	copy(out[108:116], padOctal(h.UID, 7))
	copy(out[116:124], padOctal(h.GID, 7))
	copy(out[124:136], padOctal(h.Size, 11))
	copy(out[136:148], padOctal(h.ModTime, 11))
	copy(out[156:157], []byte{h.TypeFlag})
	if h.TypeFlag == 0 {
		out[156] = 'N'
	}
	copy(out[157:257], h.LinkName)
	copy(out[257:263], []byte("ustar"))
	out[263] = 0
	copy(out[264:265], []byte("0"))
	if h.Magic != "" {
		copy(out[257:265], h.Magic)
	}
	copy(out[265:297], h.UName)
	copy(out[297:329], h.GName)
	copy(out[329:337], padOctal(h.DevMajor, 7))
	copy(out[337:345], padOctal(h.DevMinor, 7))
	copy(out[345:512], h.Prefix)
	sum := checksum(out)
	copy(out[148:156], padOctal(sum, 7))
	h.Chksum = sum
	return out
}

// checksum 计算 header 的校验和(chksum 字段本身视为空格)。
func checksum(b []byte) int64 {
	var sum int64
	for i := 0; i < headerSize; i++ {
		if i >= 148 && i < 156 {
			sum += int64(' ')
		} else {
			sum += int64(b[i])
		}
	}
	return sum
}

// cString 从固定字节切片中提取以 NUL 结尾的字符串。
func cString(b []byte) string {
	n := bytes.IndexByte(b, 0)
	if n < 0 {
		n = len(b)
	}
	return string(b[:n])
}

// padOctal 把整数渲染为左零填充 + NUL 终止符的 8 进制字符串字节。
func padOctal(v int64, width int) []byte {
	if v < 0 {
		v = 0
	}
	s := strconv.FormatInt(v, 8)
	for len(s) < width {
		s = "0" + s
	}
	return []byte(s + "\x00")
}
