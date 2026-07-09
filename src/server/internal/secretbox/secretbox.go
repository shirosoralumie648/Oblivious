package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// CodecPrefix identifies values encrypted by this package.
	CodecPrefix                               = "obv:gcm:v1:"
	DomainRelayChannelAPIKey                  = "relay-channel-api-key"
	DomainObservabilityAlertProviderConfigKey = "observability-alert-provider-config-key"
	DomainPublishingChannelConfigKey          = "publishing-channel-config-key"
	DomainWorkflowDefinitionSecretValue       = "workflow-definition-secret-value"
)

var ErrPlaintextSecretRejected = errors.New("plaintext secret rejected")

type SecretStorageStatus string

const (
	SecretStorageStatusEmpty            SecretStorageStatus = "empty"
	SecretStorageStatusProtected        SecretStorageStatus = "protected"
	SecretStorageStatusPlaintext        SecretStorageStatus = "plaintext"
	SecretStorageStatusInvalidProtected SecretStorageStatus = "invalid_protected"
)

type SecretStorageInspection struct {
	Path          string              `json:"path,omitempty"`
	Domain        string              `json:"domain"`
	Status        SecretStorageStatus `json:"status"`
	NeedsRotation bool                `json:"needsRotation"`
	Message       string              `json:"message,omitempty"`
}

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

// Open decrypts values protected by Protect. Unprefixed values remain readable
// outside production for legacy-row migration, but production rejects them.
func Open(domain, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !IsProtected(stored) {
		if rejectPlaintextSecrets() {
			return "", fmt.Errorf("%w for domain %s", ErrPlaintextSecretRejected, domain)
		}
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

// InspectStored classifies a stored secret without decrypting or returning it.
// It is intended for migration and operator audits where legacy plaintext rows
// must be identified without leaking secret material into logs or responses.
func InspectStored(domain, stored string) SecretStorageInspection {
	inspection := SecretStorageInspection{Domain: domain}
	if stored == "" {
		inspection.Status = SecretStorageStatusEmpty
		return inspection
	}
	if !IsProtected(stored) {
		inspection.Status = SecretStorageStatusPlaintext
		inspection.NeedsRotation = true
		inspection.Message = "plaintext secret requires protection before production use"
		return inspection
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(stored, CodecPrefix))
	if err != nil {
		inspection.Status = SecretStorageStatusInvalidProtected
		inspection.NeedsRotation = true
		inspection.Message = "invalid protected secret payload requires rotation"
		return inspection
	}
	gcm, err := newGCM(domain)
	if err != nil || len(payload) < gcm.NonceSize() {
		inspection.Status = SecretStorageStatusInvalidProtected
		inspection.NeedsRotation = true
		inspection.Message = "invalid protected secret payload requires rotation"
		return inspection
	}
	inspection.Status = SecretStorageStatusProtected
	return inspection
}

// InspectStoredMap walks a nested configuration payload and classifies values
// whose keys are selected by isSecretKey. It never returns inspected values.
func InspectStoredMap(domain string, payload map[string]any, isSecretKey func(string) bool) []SecretStorageInspection {
	if payload == nil || isSecretKey == nil {
		return nil
	}
	results := []SecretStorageInspection{}
	inspectStoredValue(domain, "", payload, isSecretKey, &results)
	return results
}

func inspectStoredValue(domain, path string, value any, isSecretKey func(string) bool, results *[]SecretStorageInspection) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			nextPath := secretInspectionPath(path, key)
			if isSecretKey(key) {
				if stored, ok := nested.(string); ok {
					inspection := InspectStored(domain, stored)
					inspection.Path = nextPath
					*results = append(*results, inspection)
					continue
				}
			}
			inspectStoredValue(domain, nextPath, nested, isSecretKey, results)
		}
	case []any:
		for index, item := range typed {
			inspectStoredValue(domain, fmt.Sprintf("%s[%d]", path, index), item, isSecretKey, results)
		}
	case []map[string]any:
		for index, item := range typed {
			inspectStoredValue(domain, fmt.Sprintf("%s[%d]", path, index), item, isSecretKey, results)
		}
	}
}

func secretInspectionPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func rejectPlaintextSecrets() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
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
