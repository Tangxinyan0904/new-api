package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/QuantumNous/new-api/setting/geoip_setting"
	"github.com/stretchr/testify/require"
)

func TestGeoIPDecisionBlocksConfiguredCountry(t *testing.T) {
	decision := DecideGeoIPAccessForCountry(GeoIPDecisionInput{
		Mode:             geoip_setting.ModeHomepageBlockAPIReject,
		CountryCode:      "CN",
		BlockedCountries: []string{"CN"},
		DatabaseReady:    true,
		Message:          "blocked",
	})

	require.True(t, decision.Enabled)
	require.True(t, decision.Blocked)
	require.True(t, decision.RejectsAPI())
	require.False(t, decision.RejectsWeb())
}

func TestGeoIPDecisionFailsOpenWhenDatabaseMissing(t *testing.T) {
	decision := DecideGeoIPAccessForCountry(GeoIPDecisionInput{
		Mode:             geoip_setting.ModeFullReject,
		CountryCode:      "",
		BlockedCountries: []string{"CN"},
		DatabaseReady:    false,
		Message:          "blocked",
	})

	require.True(t, decision.Enabled)
	require.False(t, decision.Blocked)
	require.False(t, decision.DatabaseReady)
	require.False(t, decision.RejectsAPI())
	require.False(t, decision.RejectsWeb())
}

func TestGeoIPDecisionIsDisabledWhenModeOff(t *testing.T) {
	decision := DecideGeoIPAccessForCountry(GeoIPDecisionInput{
		Mode:             geoip_setting.ModeOff,
		CountryCode:      "CN",
		BlockedCountries: []string{"CN"},
		DatabaseReady:    true,
		Message:          "blocked",
	})

	require.False(t, decision.Enabled)
	require.False(t, decision.Blocked)
	require.False(t, decision.RejectsAPI())
	require.False(t, decision.RejectsWeb())
}

func TestExtractGeoIPDatabaseBytesSupportsDirectMMDB(t *testing.T) {
	want := []byte("mmdb bytes")

	got, err := extractGeoIPDatabaseBytes("Country.mmdb", want)

	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestExtractGeoIPDatabaseBytesSupportsGzipMMDB(t *testing.T) {
	want := []byte("mmdb gzip bytes")
	archive := gzipBytes(t, want)

	got, err := extractGeoIPDatabaseBytes("Country.mmdb.gz", archive)

	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestExtractGeoIPDatabaseBytesSupportsTarGz(t *testing.T) {
	want := []byte("mmdb tar bytes")
	archive := tarGzipBytes(t, "folder/Country.mmdb", want)

	got, err := extractGeoIPDatabaseBytes("GeoLite2-Country.tar.gz", archive)

	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestExtractGeoIPDatabaseBytesSupportsZip(t *testing.T) {
	want := []byte("mmdb zip bytes")
	archive := zipBytes(t, "folder/Country.mmdb", want)

	got, err := extractGeoIPDatabaseBytes("Country.zip", archive)

	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestExtractGeoIPDatabaseBytesRejectsArchiveWithoutMMDB(t *testing.T) {
	archive := zipBytes(t, "readme.txt", []byte("missing"))

	_, err := extractGeoIPDatabaseBytes("Country.zip", archive)

	require.Error(t, err)
}

func gzipBytes(t *testing.T, value []byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write(value)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func tarGzipBytes(t *testing.T, name string, value []byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	require.NoError(t, tarWriter.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o600,
		Size: int64(len(value)),
	}))
	_, err := tarWriter.Write(value)
	require.NoError(t, err)
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())
	return buffer.Bytes()
}

func zipBytes(t *testing.T, name string, value []byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	fileWriter, err := writer.Create(name)
	require.NoError(t, err)
	_, err = fileWriter.Write(value)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}
