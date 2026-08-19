// Package snapshot 提供对任意值的 JSON 快照编解码，
// 支持版本号、校验和与压缩。
package snapshot

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"
)

// ErrBadMagic 在文件头错误时返回。
var ErrBadMagic = errors.New("snapshot: 文件头不匹配")

// ErrBadChecksum 在校验和失败时返回。
var ErrBadChecksum = errors.New("snapshot: 校验和不匹配")

// FormatVersion 是当前格式版本号。
const FormatVersion = 1

var magic = []byte("SNAP01")

// Metadata 是快照的元数据。
type Metadata struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Comment   string    `json:"comment,omitempty"`
}

// Header 是文件头二进制格式：magic(6) + version(4) + checksum(32) + metaLen(4)。
// 之后是 metadata JSON（未压缩），再之后是 gzip(JSON(value))。
type Header struct {
	Version   uint32
	Checksum  []byte
	MetaJSON  []byte
	Payload   []byte
}

// Encode 生成 value 的快照字节。
func Encode(value any, comment string) ([]byte, error) {
	meta := Metadata{
		Version:   FormatVersion,
		CreatedAt: time.Now(),
		Comment:   comment,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	var payloadBuf bytes.Buffer
	gz := gzip.NewWriter(&payloadBuf)
	if err := json.NewEncoder(gz).Encode(value); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(payloadBuf.Bytes())
	hdr := make([]byte, 0, 6+4+32+4+len(metaJSON))
	hdr = append(hdr, magic...)
	var ver [4]byte
	binary.BigEndian.PutUint32(ver[:], uint32(FormatVersion))
	hdr = append(hdr, ver[:]...)
	hdr = append(hdr, sum[:]...)
	var metaLen [4]byte
	binary.BigEndian.PutUint32(metaLen[:], uint32(len(metaJSON)))
	hdr = append(hdr, metaLen[:]...)
	hdr = append(hdr, metaJSON...)
	hdr = append(hdr, payloadBuf.Bytes()...)
	return hdr, nil
}

// Decode 解析快照字节到 value，并返回元数据。
func Decode(data []byte, value any) (*Metadata, error) {
	if len(data) < 6+4+32+4 {
		return nil, ErrBadMagic
	}
	if !bytes.Equal(data[:6], magic) {
		return nil, ErrBadMagic
	}
	metaLen := binary.BigEndian.Uint32(data[42:46])
	if int(metaLen)+46 > len(data) {
		return nil, ErrBadChecksum
	}
	metaJSON := data[46 : 46+metaLen]
	payload := data[46+metaLen:]
	stored := data[10:42]
	sum := sha256.Sum256(payload)
	if !bytes.Equal(stored, sum[:]) {
		return nil, ErrBadChecksum
	}
	var meta Metadata
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	body, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, value); err != nil {
		return nil, err
	}
	return &meta, nil
}

// Checksum 返回字节流的 SHA-256（不依赖格式）。
func Checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
