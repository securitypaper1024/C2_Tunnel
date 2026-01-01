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

type AESCipher struct {
	key   []byte
	block cipher.Block
}

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

func (c *AESCipher) Encrypt(plaintext []byte) ([]byte, error) {
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(c.block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

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

type CryptoConn struct {
	net.Conn
	cipher *AESCipher
}

func NewCryptoConn(conn net.Conn, cipher *AESCipher) *CryptoConn {
	return &CryptoConn{
		Conn:   conn,
		cipher: cipher,
	}
}

func (c *CryptoConn) ReadEncrypted() ([]byte, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(c.Conn, lenBuf); err != nil {
		return nil, err
	}

	length := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])

	if length <= 0 || length > 1024*1024*10 {
		return nil, errors.New("invalid data length")
	}

	encrypted := make([]byte, length)
	if _, err := io.ReadFull(c.Conn, encrypted); err != nil {
		return nil, err
	}

	return c.cipher.Decrypt(encrypted)
}

func (c *CryptoConn) WriteEncrypted(data []byte) error {
	encrypted, err := c.cipher.Encrypt(data)
	if err != nil {
		return err
	}

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

	_, err = c.Conn.Write(encrypted)
	return err
}
