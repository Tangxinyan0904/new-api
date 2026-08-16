package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorCredentialEncryptsAndDecrypts(t *testing.T) {
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "stable-channel-monitor-test-secret"
	t.Cleanup(func() {
		common.CryptoSecret = previousSecret
	})

	const plaintext = "sk-monitor-secret"
	ciphertext, suffix, err := EncryptChannelMonitorCredential(plaintext)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(ciphertext, "v1:"))
	assert.NotContains(t, ciphertext, plaintext)
	assert.Equal(t, "cret", suffix)
	assert.Equal(t, "****cret", MaskChannelMonitorCredentialSuffix(suffix))

	decrypted, err := DecryptChannelMonitorCredential(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestChannelMonitorCredentialRejectsChangedSecret(t *testing.T) {
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "first-channel-monitor-secret"
	t.Cleanup(func() {
		common.CryptoSecret = previousSecret
	})

	ciphertext, _, err := EncryptChannelMonitorCredential("monitor-key")
	require.NoError(t, err)

	common.CryptoSecret = "replacement-channel-monitor-secret"
	_, err = DecryptChannelMonitorCredential(ciphertext)
	require.Error(t, err)
}

func TestChannelMonitorCredentialRejectsEmptyAndUnknownFormats(t *testing.T) {
	_, _, err := EncryptChannelMonitorCredential("")
	require.Error(t, err)

	_, err = DecryptChannelMonitorCredential("")
	require.Error(t, err)

	_, err = DecryptChannelMonitorCredential("v2:not-supported")
	require.Error(t, err)

	assert.Equal(t, "", MaskChannelMonitorCredentialSuffix(""))
}
