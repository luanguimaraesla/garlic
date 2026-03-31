// Package crypto provides AES-GCM encryption/decryption utilities and a SHA-256
// hashing function.
//
// # Manager Interface
//
// The [Manager] interface defines Encrypt and Decrypt methods, allowing
// production and mock implementations to be swapped in tests.
//
// # Encryption
//
// [Crypto] implements [Manager] using AES-256-GCM with authenticated encryption.
// The encryption key is derived from the configured master key via HKDF-SHA256:
//
//	c, err := crypto.New(&crypto.Config{MasterKey: "my-secret-key"})
//	ciphertext, err := c.Encrypt([]byte("plaintext"))
//	plaintext, err := c.Decrypt(ciphertext)
//
// Encrypted output is base64-encoded with the nonce prepended.
//
// # Hashing
//
// [HashSHA256] returns the hex-encoded SHA-256 digest of a string.
//
// # Mocking
//
// In unit tests (build tag: unit), use [CryptoManagerMock] as a drop-in
// replacement for [Manager]. [NewCryptoManagerMock] returns a mock that
// round-trips values unchanged; [NewInvalidCryptoManagerMock] returns one that
// always fails with the given error.
package crypto
