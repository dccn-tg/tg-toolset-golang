package strings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"time"

	mrand "math/rand"

	log "github.com/sirupsen/logrus"
)

var logger *log.Entry

func init() {
	// set logging
	logger = log.WithFields(log.Fields{"source": "utility.strings"})
}

// StringWrap wraps text at the specified column lineWidth on word breaks
func StringWrap(text string, lineWidth int) string {
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		return text
	}
	wrapped := words[0]
	spaceLeft := lineWidth - len(wrapped)
	for _, word := range words[1:] {
		if len(word)+1 > spaceLeft {
			wrapped += "\n" + word
			spaceLeft = lineWidth - len(word)
		} else {
			wrapped += " " + word
			spaceLeft -= 1 + len(word)
		}
	}
	return wrapped
}

// StringXOR finds the XOR between two strings and return the XOR part in byte array.
func StringXOR(str1, str2 string) []byte {

	var out []byte

	set1 := make(map[byte]bool)
	set2 := make(map[byte]bool)

	for _, c := range []byte(str1) {
		set1[c] = true
	}

	for _, c := range []byte(str2) {
		set2[c] = true
	}

	for _, c := range []byte(str1) {
		if !set2[c] {
			out = append(out, c)
		}
	}

	for _, c := range []byte(str2) {
		if !set1[c] {
			out = append(out, c)
		}
	}

	return out
}

// StringAND finds the AND between two strings and return the AND part in byte array.
func StringAND(str1, str2 string) []byte {

	var out []byte

	set1 := make(map[byte]bool)

	for _, c := range []byte(str1) {
		set1[c] = true
	}

	for _, c := range []byte(str2) {
		if set1[c] {
			out = append(out, c)
		}
	}

	return out
}

// Encrypt encrypts string to base64 crypto using AES
func Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts from base64 to decrypted string
func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *mrand.Rand = mrand.New(mrand.NewSource(time.Now().UnixNano()))

// Random generates a string with a given length.
func Random(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
