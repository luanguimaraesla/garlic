package crypto_test

import (
	"fmt"

	"github.com/luanguimaraesla/garlic/crypto"
)

func ExampleCrypto() {
	c := crypto.New(&crypto.Config{MasterKey: "test-secret-key"})

	ciphertext, err := c.Encrypt([]byte("hello world"))
	if err != nil {
		panic(err)
	}

	plaintext, err := c.Decrypt(ciphertext)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(plaintext))
	// Output:
	// hello world
}

func ExampleHashSHA256() {
	hash := crypto.HashSHA256("hello")
	fmt.Println(hash)
	// Output:
	// 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
}
