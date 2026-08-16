package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const channelMonitorCredentialVersion = "v1"

var channelMonitorCredentialAAD = []byte("new-api/channel-monitor/v1")

func EncryptChannelMonitorCredential(plaintext string) (string, string, error) {
	if plaintext == "" {
		return "", "", errors.New("channel monitor credential is required")
	}

	gcm, err := newChannelMonitorCredentialGCM()
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", fmt.Errorf("generate channel monitor credential nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), channelMonitorCredentialAAD)
	payload := make([]byte, 0, len(nonce)+len(sealed))
	payload = append(payload, nonce...)
	payload = append(payload, sealed...)

	runes := []rune(plaintext)
	start := len(runes) - 4
	if start < 0 {
		start = 0
	}
	suffix := string(runes[start:])
	return channelMonitorCredentialVersion + ":" + base64.RawStdEncoding.EncodeToString(payload), suffix, nil
}

func DecryptChannelMonitorCredential(ciphertext string) (string, error) {
	prefix := channelMonitorCredentialVersion + ":"
	if !strings.HasPrefix(ciphertext, prefix) {
		return "", errors.New("unsupported channel monitor credential format")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(ciphertext, prefix))
	if err != nil {
		return "", errors.New("invalid channel monitor credential encoding")
	}

	gcm, err := newChannelMonitorCredentialGCM()
	if err != nil {
		return "", err
	}
	if len(payload) <= gcm.NonceSize() {
		return "", errors.New("invalid channel monitor credential payload")
	}
	plaintext, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], channelMonitorCredentialAAD)
	if err != nil {
		return "", errors.New("decrypt channel monitor credential failed")
	}
	return string(plaintext), nil
}

func MaskChannelMonitorCredentialSuffix(suffix string) string {
	if suffix == "" {
		return ""
	}
	return "****" + suffix
}

func newChannelMonitorCredentialGCM() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte("new-api/channel-monitor/v1\x00" + common.CryptoSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create channel monitor credential cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create channel monitor credential gcm: %w", err)
	}
	return gcm, nil
}
