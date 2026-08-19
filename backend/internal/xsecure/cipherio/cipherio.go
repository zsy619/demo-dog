// Package cipherio 提供基于 AES-CTR 的流式加解密接口，
// 适合大文件或连续流加密场景。
package cipherio

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// NewReader 返回一个解密流：从 src 读取密文，明文写到 out。
func NewReader(stream cipher.Stream, src io.Reader) io.Reader {
	return &cipher.StreamReader{S: stream, R: src}
}

// NewWriter 返回一个加密流：写入的明文会先加密再写到 dst。
func NewWriter(stream cipher.Stream, dst io.Writer) io.Writer {
	return &cipher.StreamWriter{S: stream, W: dst}
}

// CTRFromKey 基于 16/24/32 字节 key 创建 AES-CTR 密码流；
// nonce 长度需等于 AES 块大小。
func CTRFromKey(key, nonce []byte) (cipher.Stream, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("cipherio: key 长度错误")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aes.BlockSize {
		return nil, errors.New("cipherio: nonce 长度错误")
	}
	return cipher.NewCTR(block, nonce), nil
}

// RandomNonce 生成长度为 n 的随机 nonce。
func RandomNonce(n int) ([]byte, error) {
	if n <= 0 {
		n = aes.BlockSize
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// EncryptCopy 把 r 加密后写入 w。
func EncryptCopy(w io.Writer, r io.Reader, key []byte) error {
	nonce, err := RandomNonce(aes.BlockSize)
	if err != nil {
		return err
	}
	if _, err := w.Write(nonce); err != nil {
		return err
	}
	stream, err := CTRFromKey(key, nonce)
	if err != nil {
		return err
	}
	_, err = io.Copy(NewWriter(stream, w), r)
	return err
}

// DecryptCopy 从 r 读取并解密，写入 w。
func DecryptCopy(w io.Writer, r io.Reader, key []byte) error {
	nonce := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(r, nonce); err != nil {
		return err
	}
	stream, err := CTRFromKey(key, nonce)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, NewReader(stream, r))
	return err
}
