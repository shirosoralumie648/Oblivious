package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// CodecPrefix identifies values encrypted by this package.
	CodecPrefix              = "obv:gcm:v1:"
	DomainRelayChannelAPIKey = "relay-channel-api-key"
)

// Protect encrypts plaintext with AES-GCM and binds it to the supplied domain.
func Protect(domain, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if IsProtected(plaintext) {
		return plaintext, nil
	}

	gcm, err := newGCM(domain)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), []byte(domain))
	payload := append(nonce, ciphertext...)
	return CodecPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

// Open decrypts values protected by Protect. Unprefixed values are returned as
// plaintext so existing database rows remain readable until rotated.
func Open(domain, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !IsProtected(stored) {
		return stored, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(stored, CodecPrefix))
	if err != nil {
		return "", err
	}
	gcm, err := newGCM(domain)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("invalid protected secret payload")
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(domain))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func IsProtected(stored string) bool {
	return strings.HasPrefix(stored, CodecPrefix)
}

func newGCM(domain string) (cipher.AEAD, error) {
	key := encryptionKey(domain)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptionKey(domain string) [32]byte {
	secret := strings.TrimSpace(os.Getenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	}
	return sha256.Sum256([]byte("oblivious:secretbox:" + domain + ":" + secret))
}
