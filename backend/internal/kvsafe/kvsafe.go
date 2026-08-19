// Package kvsafe 提供一个简单的加密 KV 持久化保险箱。
// 数据以 AES-GCM 加密后落盘为单个文件。
package kvsafe

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sync"
)

// ErrBadMagic 在文件头不是预期 magic 时返回。
var ErrBadMagic = errors.New("kvsafe: 文件头不匹配")

// ErrShortBuffer 当文件短于最小长度时返回。
var ErrShortBuffer = errors.New("kvsafe: 文件过短")

var magic = []byte("KVSAFE01")

const headerLen = 8 + 12 + 16

type Safe struct {
	mu    sync.RWMutex
	path  string
	key   []byte
	data  map[string][]byte
	dirty bool
}

func Open(path string, key []byte) (*Safe, error) {
	if _, err := aes.NewCipher(key); err != nil {
		return nil, err
	}
	s := &Safe{path: path, key: append([]byte{}, key...), data: make(map[string][]byte)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Safe) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	if !ok {
		return nil, false
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, true
}

func (s *Safe) Put(key string, value []byte) {
	s.mu.Lock()
	s.data[key] = append([]byte{}, value...)
	s.dirty = true
	s.mu.Unlock()
}

func (s *Safe) Delete(key string) {
	s.mu.Lock()
	if _, ok := s.data[key]; ok {
		delete(s.data, key)
		s.dirty = true
	}
	s.mu.Unlock()
}

func (s *Safe) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

func (s *Safe) Snapshot() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.data))
	for k := range s.data {
		out = append(out, k)
	}
	return out
}

func (s *Safe) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return nil
	}
	return s.flushLocked()
}

func (s *Safe) load() error {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	hdr := make([]byte, headerLen)
	if _, err := io.ReadFull(f, hdr); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	if !bytesEqual(hdr[:8], magic) {
		return ErrBadMagic
	}
	nonce := hdr[8:20]
	tag := hdr[20:36]
	body, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	gcm, err := newGCM(s.key)
	if err != nil {
		return err
	}
	plain, err := gcm.Open(nil, nonce, append(body, tag...), nil)
	if err != nil {
		return err
	}
	if len(plain) == 0 {
		return nil
	}
	return decodeKV(plain, s.data)
}

func (s *Safe) flushLocked() error {
	plain := encodeKV(s.data)
	gcm, err := newGCM(s.key)
	if err != nil {
		return err
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := gcm.Seal(nil, nonce, plain, nil)
	ct := sealed[:len(sealed)-16]
	tag := sealed[len(sealed)-16:]
	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(magic); err != nil {
		return err
	}
	if _, err := f.Write(nonce); err != nil {
		return err
	}
	if _, err := f.Write(tag); err != nil {
		return err
	}
	if _, err := f.Write(ct); err != nil {
		return err
	}
	s.dirty = false
	return nil
}

func encodeKV(m map[string][]byte) []byte {
	var out []byte
	for k, v := range m {
		var lk [4]byte
		var lv [4]byte
		binary.BigEndian.PutUint32(lk[:], uint32(len(k)))
		binary.BigEndian.PutUint32(lv[:], uint32(len(v)))
		out = append(out, lk[:]...)
		out = append(out, []byte(k)...)
		out = append(out, lv[:]...)
		out = append(out, v...)
	}
	return out
}

func decodeKV(b []byte, dst map[string][]byte) error {
	for len(b) > 0 {
		if len(b) < 4 {
			return ErrShortBuffer
		}
		lk := binary.BigEndian.Uint32(b[:4])
		if int(lk)+4 > len(b) {
			return ErrShortBuffer
		}
		k := string(b[4 : 4+lk])
		rest := b[4+lk:]
		if len(rest) < 4 {
			return ErrShortBuffer
		}
		lv := binary.BigEndian.Uint32(rest[:4])
		if int(lv)+4 > len(rest) {
			return ErrShortBuffer
		}
		v := append([]byte{}, rest[4:4+lv]...)
		dst[k] = v
		b = rest[4+lv:]
	}
	return nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
