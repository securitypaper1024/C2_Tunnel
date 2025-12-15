package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"net"
)

// AESCipher 封装 AES-256-CFB 加解密
type AESCipher struct {
	key   []byte
	block cipher.Block
}

// NewAESCipher 创建新的 AES 加密器
// password 会通过 SHA256 转换为 32 字节密钥
func NewAESCipher(password string) (*AESCipher, error) {
	hash := sha256.Sum256([]byte(password))
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	return &AESCipher{
		key:   key,
		block: block,
	}, nil
}

// Encrypt 加密数据
func (c *AESCipher) Encrypt(plaintext []byte) ([]byte, error) {
	// IV + 密文
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	// 生成随机 IV
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(c.block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

// Decrypt 解密数据
func (c *AESCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCFBDecrypter(c.block, iv)
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// CryptoConn 加密连接包装器
type CryptoConn struct {
	net.Conn
	cipher *AESCipher
}

// NewCryptoConn 创建加密连接
func NewCryptoConn(conn net.Conn, cipher *AESCipher) *CryptoConn {
	return &CryptoConn{
		Conn:   conn,
		cipher: cipher,
	}
}

// ReadEncrypted 读取加密数据并解密
func (c *CryptoConn) ReadEncrypted() ([]byte, error) {
	// 读取长度头 (4字节)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(c.Conn, lenBuf); err != nil {
		return nil, err
	}

	length := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])

	if length <= 0 || length > 1024*1024*10 { // 最大 10MB
		return nil, errors.New("invalid data length")
	}

	// 读取加密数据
	encrypted := make([]byte, length)
	if _, err := io.ReadFull(c.Conn, encrypted); err != nil {
		return nil, err
	}

	// 解密
	return c.cipher.Decrypt(encrypted)
}

// WriteEncrypted 加密数据并写入
func (c *CryptoConn) WriteEncrypted(data []byte) error {
	// 加密
	encrypted, err := c.cipher.Encrypt(data)
	if err != nil {
		return err
	}

	// 写入长度头
	length := len(encrypted)
	lenBuf := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	if _, err := c.Conn.Write(lenBuf); err != nil {
		return err
	}

	// 写入加密数据
	_, err = c.Conn.Write(encrypted)
	return err
}

