package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func encryptForTest(key string, plaintext []byte) (string, error) {
	encrypted, err := encryptAESForTest(plaintext, key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func encryptAESForTest(plaintext []byte, encryptKey string) ([]byte, error) {
	keyBytes := sha256.Sum256([]byte(encryptKey))
	key := keyBytes[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	iv := make([]byte, aes.BlockSize)
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(padded, padded)

	return append(iv, padded...), nil
}

func TestDecryptAES256CBC_ValidData(t *testing.T) {
	key := "test_encrypt_key_1234567890"
	plaintext := []byte("Hello, World!")

	encrypted, err := encryptForTest(key, plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt test data: %v", err)
	}

	decrypted, err := DecryptAES256CBC(encrypted, key)
	if err != nil {
		t.Fatalf("DecryptAES256CBC() error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptAES256CBC_EmptyKey(t *testing.T) {
	_, err := DecryptAES256CBC("somedata", "")
	if err == nil {
		t.Error("DecryptAES256CBC() should return error for empty key")
	}
}

func TestDecryptAES256CBC_InvalidBase64(t *testing.T) {
	_, err := DecryptAES256CBC("not-valid-base64!!!", "somekey")
	if err == nil {
		t.Error("DecryptAES256CBC() should return error for invalid base64")
	}
}

func TestDecryptAES256CBC_DataTooShort(t *testing.T) {
	key := "test_key"
	_, err := DecryptAES256CBC("c2hvcnQ=", key)
	if err == nil {
		t.Error("DecryptAES256CBC() should return error for data shorter than AES block size + 1")
	}
}

func TestPKCS7Unpad_ValidPadding(t *testing.T) {
	data := []byte("Hello, World!\x03\x03\x03")
	unpadded, err := pkcs7Unpad(data, 16)
	if err != nil {
		t.Fatalf("pkcs7Unpad() error = %v", err)
	}
	if !bytes.Equal(unpadded, []byte("Hello, World!")) {
		t.Errorf("pkcs7Unpad() = %q, want %q", unpadded, "Hello, World!")
	}
}

func TestPKCS7Unpad_EmptyData(t *testing.T) {
	_, err := pkcs7Unpad([]byte{}, 16)
	if err == nil {
		t.Error("pkcs7Unpad() should return error for empty data")
	}
}

func TestPKCS7Unpad_NotMultipleOfBlockSize(t *testing.T) {
	data := []byte("not a multiple")
	_, err := pkcs7Unpad(data, 16)
	if err == nil {
		t.Error("pkcs7Unpad() should return error when data length is not multiple of block size")
	}
}

func TestPKCS7Unpad_InvalidPaddingLength(t *testing.T) {
	data := []byte("Hello\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10")
	_, err := pkcs7Unpad(data, 16)
	if err == nil {
		t.Error("pkcs7Unpad() should return error for invalid padding length (padLen > blockSize)")
	}
}

func TestPKCS7Unpad_ZeroPaddingLength(t *testing.T) {
	data := []byte("Hello\x00\x00\x00\x00\x00")
	_, err := pkcs7Unpad(data, 16)
	if err == nil {
		t.Error("pkcs7Unpad() should return error for zero padding length")
	}
}

func TestPKCS7Unpad_InvalidPaddingContent(t *testing.T) {
	data := []byte("Hello\x05\x05\x05\x05\x03")
	_, err := pkcs7Unpad(data, 16)
	if err == nil {
		t.Error("pkcs7Unpad() should return error for invalid padding content")
	}
}

func TestPKCS7Unpad_AllZerosPadding(t *testing.T) {
	data := []byte("TestData\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10\x10")
	_, err := pkcs7Unpad(data, 16)
	if err == nil {
		t.Error("pkcs7Unpad() should return error for padding with wrong byte values")
	}
}