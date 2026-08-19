// Package xhash 汇总常用哈希工具：md5/sha1/sha256/sha512/fnv。
package xhash

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash/fnv"
)

// MD5Hex 返回 md5 十六进制字符串。
func MD5Hex(b []byte) string {
	s := md5.Sum(b)
	return hex.EncodeToString(s[:])
}

// SHA1Hex 返回 sha1 十六进制字符串。
func SHA1Hex(b []byte) string {
	s := sha1.Sum(b)
	return hex.EncodeToString(s[:])
}

// SHA256Hex 返回 sha256 十六进制字符串。
func SHA256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// SHA512Hex 返回 sha512 十六进制字符串。
func SHA512Hex(b []byte) string {
	s := sha512.Sum512(b)
	return hex.EncodeToString(s[:])
}

// FNV32 返回 FNV-32 哈希值。
func FNV32(b []byte) uint32 {
	h := fnv.New32()
	h.Write(b)
	return h.Sum32()
}

// FNV64 返回 FNV-64 哈希值。
func FNV64(b []byte) uint64 {
	h := fnv.New64()
	h.Write(b)
	return h.Sum64()
}

// StringSHA256 字符串便捷版本。
func StringSHA256(s string) string { return SHA256Hex([]byte(s)) }

// StringMD5 字符串便捷版本。
func StringMD5(s string) string { return MD5Hex([]byte(s)) }
