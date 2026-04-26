// Package crypto provides encryption and decryption utilities for secure communication.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// DecryptAES256CBC decrypts data that was encrypted using AES-256-CBC.
// The encryptKey is hashed using SHA256 to create a 32-byte key.
// The encrypted data should be base64-encoded and contain the IV in the first 16 bytes.
func DecryptAES256CBC(encryptedData, encryptKey string) ([]byte, error) {
	keyBytes := sha256.Sum256([]byte(encryptKey))
	key := keyBytes[:]

	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, err
	}

	if len(encryptedBytes) < aes.BlockSize+1 {
		return nil, errors.New("encrypted data too short")
	}

	iv := encryptedBytes[:aes.BlockSize]
	encryptedEvent := encryptedBytes[aes.BlockSize:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(encryptedEvent, encryptedEvent)

	plaintext, err := pkcs7Unpad(encryptedEvent, aes.BlockSize)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("data is empty")
	}

	if len(data)%blockSize != 0 {
		return nil, errors.New("data length is not a multiple of block size")
	}

	padLen := int(data[len(data)-1])
	if padLen > blockSize || padLen == 0 {
		return nil, errors.New("invalid padding length")
	}

	for i := 0; i < padLen; i++ {
		if data[len(data)-1-i] != byte(padLen) {
			return nil, errors.New("invalid padding content")
		}
	}

	return data[:len(data)-padLen], nil
}