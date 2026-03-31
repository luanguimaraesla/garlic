package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"

	"github.com/luanguimaraesla/garlic/errors"
)

type Manager interface {
	Encrypt([]byte) (string, error)
	Decrypt(string) ([]byte, error)
}

type Crypto struct {
	config *Config
}

func New(cfg *Config) (*Crypto, error) {
	if cfg.MasterKey == "" {
		return nil, errors.New(errors.KindSystemError, "crypto master key must not be empty")
	}

	return &Crypto{cfg}, nil
}

// Encrypt uses AES-256-GCM to encrypt the content.
func (c *Crypto) Encrypt(content []byte) (string, error) {
	key, err := c.deriveKey()
	if err != nil {
		return "", errors.Propagate(err, "failed to derive encryption key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errors.Propagate(err, "failed to create AES cipher")
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", errors.Propagate(err, "failed to create GCM")
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", errors.Propagate(err, "failed to generate nonce")
	}

	ciphertext := aead.Seal(nonce, nonce, content, nil)

	return base64Encode(ciphertext), nil
}

// Decrypt uses AES-256-GCM to decrypt the encoded ciphertext.
func (c *Crypto) Decrypt(encodedText string) ([]byte, error) {
	data, err := base64Decode(encodedText)
	if err != nil {
		return nil, errors.Propagate(err, "failed to decode base64 string")
	}

	key, err := c.deriveKey()
	if err != nil {
		return nil, errors.Propagate(err, "failed to derive decryption key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Propagate(err, "failed to create AES cipher")
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Propagate(err, "failed to create GCM")
	}

	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New(errors.KindSystemError, "ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.Propagate(err, "failed to decrypt ciphertext")
	}

	return plaintext, nil
}

// deriveKey uses HKDF with SHA-256 to derive a 32-byte AES key from the master key.
func (c *Crypto) deriveKey() ([]byte, error) {
	masterKey := []byte(c.config.MasterKey)
	info := []byte("garlic-aes-256-gcm")

	hkdfReader := hkdf.New(sha256.New, masterKey, nil, info)

	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, errors.Propagate(err, "failed to derive key with HKDF")
	}

	return key, nil
}
