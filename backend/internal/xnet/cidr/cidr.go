// Package cidr 提供 CIDR/IP 网段解析与匹配辅助。
package cidr

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"
)

// Contains 判断 ip 是否在 CIDR 网段中。
func Contains(cidr string, ip net.IP) (bool, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, err
	}
	return ipnet.Contains(ip), nil
}

// MustContains 同 Contains，出错时返回 false。
func MustContains(cidr string, ip net.IP) bool {
	ok, _ := Contains(cidr, ip)
	return ok
}

// Parse 解析 CIDR 字符串并返回起始 / 结束 IP（uint32 / [16]byte）。
func Parse(cidr string) (net.IP, net.IP, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, nil, err
	}
	return ip, ipnet.IP, nil
}

// Equal 比较两个 IP。
func Equal(a, b net.IP) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Equal(b)
}

// ToUint32 把 IPv4 转成 uint32。
func ToUint32(ip net.IP) (uint32, bool) {
	v4 := ip.To4()
	if v4 == nil {
		return 0, false
	}
	return binary.BigEndian.Uint32(v4), true
}

// FromUint32 把 uint32 转成 IPv4。
func FromUint32(v uint32) net.IP {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return net.IPv4(b[0], b[1], b[2], b[3])
}

// Validate 检查字符串是否是合法 IP 或 CIDR。
func Validate(s string) bool {
	if strings.Contains(s, "/") {
		_, _, err := net.ParseCIDR(s)
		return err == nil
	}
	return net.ParseIP(s) != nil
}

// ErrInvalid 是非法输入的统一错误。
var ErrInvalid = errors.New("cidr: 非法输入")
