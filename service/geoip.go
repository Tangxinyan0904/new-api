package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/geoip_setting"
	"github.com/oschwald/maxminddb-golang"
)

const defaultMaxMindCountryDownloadURL = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&suffix=tar.gz&license_key=%s"

type GeoIPDecisionInput struct {
	Mode             string
	CountryCode      string
	BlockedCountries []string
	DatabaseReady    bool
	Message          string
}

type GeoIPDecision struct {
	Enabled       bool   `json:"enabled"`
	Mode          string `json:"mode"`
	Blocked       bool   `json:"blocked"`
	CountryCode   string `json:"country_code"`
	Message       string `json:"message"`
	DatabaseReady bool   `json:"database_ready"`
}

type geoIPSettingsSnapshot struct {
	Mode                 string
	DatabasePath         string
	DownloadURL          string
	MaxMindLicenseKey    string
	PopupMessage         string
	AllowPrivateLoopback bool
	BlockedCountries     []string
}

type geoIPReaderCache struct {
	mutex   sync.Mutex
	path    string
	modTime time.Time
	size    int64
	reader  *maxminddb.Reader
}

var geoIPCache geoIPReaderCache

func (decision GeoIPDecision) RejectsAPI() bool {
	if !decision.Enabled || !decision.Blocked {
		return false
	}
	return decision.Mode == geoip_setting.ModeHomepageBlockAPIReject ||
		decision.Mode == geoip_setting.ModeFullReject
}

func (decision GeoIPDecision) RejectsWeb() bool {
	return decision.Enabled && decision.Blocked && decision.Mode == geoip_setting.ModeFullReject
}

func DecideGeoIPAccessForCountry(input GeoIPDecisionInput) GeoIPDecision {
	mode := strings.TrimSpace(input.Mode)
	if err := geoip_setting.ValidateMode(mode); err != nil {
		mode = geoip_setting.ModeOff
	}

	decision := GeoIPDecision{
		Enabled:       mode != geoip_setting.ModeOff,
		Mode:          mode,
		CountryCode:   strings.ToUpper(strings.TrimSpace(input.CountryCode)),
		Message:       input.Message,
		DatabaseReady: input.DatabaseReady,
	}
	if !decision.Enabled || !input.DatabaseReady {
		return decision
	}

	blockedCountries := make(map[string]struct{}, len(input.BlockedCountries))
	for _, country := range input.BlockedCountries {
		code := strings.ToUpper(strings.TrimSpace(country))
		if code != "" {
			blockedCountries[code] = struct{}{}
		}
	}
	_, decision.Blocked = blockedCountries[decision.CountryCode]
	return decision
}

func ResolveGeoIPDecision(ipText string) GeoIPDecision {
	settings := currentGeoIPSettings()
	if settings.Mode == geoip_setting.ModeOff {
		return DecideGeoIPAccessForCountry(GeoIPDecisionInput{
			Mode:             settings.Mode,
			BlockedCountries: settings.BlockedCountries,
			Message:          settings.PopupMessage,
		})
	}

	ip := net.ParseIP(strings.TrimSpace(ipText))
	if ip == nil {
		return DecideGeoIPAccessForCountry(GeoIPDecisionInput{
			Mode:             settings.Mode,
			BlockedCountries: settings.BlockedCountries,
			Message:          settings.PopupMessage,
			DatabaseReady:    false,
		})
	}
	if settings.AllowPrivateLoopback && shouldBypassGeoIPForIP(ip) {
		return GeoIPDecision{
			Enabled:       true,
			Mode:          settings.Mode,
			Blocked:       false,
			CountryCode:   "",
			Message:       settings.PopupMessage,
			DatabaseReady: false,
		}
	}

	countryCode, databaseReady, _ := lookupGeoIPCountryCode(ip, settings.DatabasePath)
	return DecideGeoIPAccessForCountry(GeoIPDecisionInput{
		Mode:             settings.Mode,
		CountryCode:      countryCode,
		BlockedCountries: settings.BlockedCountries,
		DatabaseReady:    databaseReady,
		Message:          settings.PopupMessage,
	})
}

func DownloadGeoIPDatabase(ctx context.Context) error {
	settings := currentGeoIPSettings()
	downloadURL := resolveGeoIPDownloadURL(settings)
	if downloadURL == "" {
		return errors.New("GeoIP download URL is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	if err := ValidateSSRFProtectedFetchURL(downloadURL); err != nil {
		return fmt.Errorf("request reject: %w", err)
	}

	client := GetSSRFProtectedHTTPClient()
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("GeoIP database download failed with status %d", resp.StatusCode)
	}

	maxBytes := int64(constant.MaxFileDownloadMB)
	if maxBytes <= 0 {
		maxBytes = 128
	}
	maxBytes <<= 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > maxBytes {
		return fmt.Errorf("GeoIP database download exceeds %dMB", maxBytes>>20)
	}

	filename := geoIPDownloadFilename(resp, downloadURL)
	databaseBytes, err := extractGeoIPDatabaseBytes(filename, body)
	if err != nil {
		return err
	}
	return writeGeoIPDatabase(settings.DatabasePath, databaseBytes)
}

func currentGeoIPSettings() geoIPSettingsSnapshot {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	blockedCountries := append([]string(nil), geoip_setting.BlockedCountries...)
	return geoIPSettingsSnapshot{
		Mode:                 geoip_setting.Mode,
		DatabasePath:         geoip_setting.DatabasePath,
		DownloadURL:          geoip_setting.DownloadURL,
		MaxMindLicenseKey:    geoip_setting.MaxMindLicenseKey,
		PopupMessage:         geoip_setting.PopupMessage,
		AllowPrivateLoopback: geoip_setting.AllowPrivateLoopback,
		BlockedCountries:     blockedCountries,
	}
}

func shouldBypassGeoIPForIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast()
}

func lookupGeoIPCountryCode(ip net.IP, databasePath string) (string, bool, error) {
	reader, err := getGeoIPReader(databasePath)
	if err != nil {
		return "", false, err
	}

	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
		RegisteredCountry struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"registered_country"`
	}
	if err := reader.Lookup(ip, &record); err != nil {
		return "", true, err
	}
	countryCode := strings.ToUpper(strings.TrimSpace(record.Country.ISOCode))
	if countryCode == "" {
		countryCode = strings.ToUpper(strings.TrimSpace(record.RegisteredCountry.ISOCode))
	}
	return countryCode, true, nil
}

func getGeoIPReader(databasePath string) (*maxminddb.Reader, error) {
	if strings.TrimSpace(databasePath) == "" {
		databasePath = geoip_setting.DatabasePath
	}
	stat, err := os.Stat(databasePath)
	if err != nil {
		return nil, err
	}

	geoIPCache.mutex.Lock()
	defer geoIPCache.mutex.Unlock()
	if geoIPCache.reader != nil &&
		geoIPCache.path == databasePath &&
		geoIPCache.modTime.Equal(stat.ModTime()) &&
		geoIPCache.size == stat.Size() {
		return geoIPCache.reader, nil
	}

	reader, err := maxminddb.Open(databasePath)
	if err != nil {
		return nil, err
	}
	if geoIPCache.reader != nil {
		geoIPCache.reader.Close()
	}
	geoIPCache.reader = reader
	geoIPCache.path = databasePath
	geoIPCache.modTime = stat.ModTime()
	geoIPCache.size = stat.Size()
	return reader, nil
}

func closeGeoIPReaderCache(databasePath string) {
	geoIPCache.mutex.Lock()
	defer geoIPCache.mutex.Unlock()
	if geoIPCache.reader == nil {
		return
	}
	if databasePath == "" || geoIPCache.path == databasePath {
		geoIPCache.reader.Close()
		geoIPCache.reader = nil
		geoIPCache.path = ""
		geoIPCache.modTime = time.Time{}
		geoIPCache.size = 0
	}
}

func resolveGeoIPDownloadURL(settings geoIPSettingsSnapshot) string {
	if downloadURL := strings.TrimSpace(settings.DownloadURL); downloadURL != "" {
		return downloadURL
	}
	if licenseKey := strings.TrimSpace(settings.MaxMindLicenseKey); licenseKey != "" {
		return fmt.Sprintf(defaultMaxMindCountryDownloadURL, url.QueryEscape(licenseKey))
	}
	return ""
}

func geoIPDownloadFilename(resp *http.Response, rawURL string) string {
	if resp != nil {
		if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
			if filename := strings.TrimSpace(params["filename"]); filename != "" {
				return filename
			}
		}
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "Country.mmdb"
	}
	filename := path.Base(parsedURL.Path)
	if filename == "." || filename == "/" || filename == "" {
		return "Country.mmdb"
	}
	return filename
}

func extractGeoIPDatabaseBytes(filename string, data []byte) ([]byte, error) {
	lowerName := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lowerName, ".tar.gz") || strings.HasSuffix(lowerName, ".tgz"):
		return extractMMDBFromTarGzip(data)
	case strings.HasSuffix(lowerName, ".zip"):
		return extractMMDBFromZip(data)
	case strings.HasSuffix(lowerName, ".mmdb.gz") || strings.HasSuffix(lowerName, ".gz"):
		return gunzipBytes(data)
	case strings.HasSuffix(lowerName, ".mmdb"):
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported GeoIP database archive type: %s", filename)
	}
}

func gunzipBytes(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func extractMMDBFromTarGzip(data []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if header == nil || header.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(header.Name), ".mmdb") {
			continue
		}
		return io.ReadAll(tarReader)
	}
	return nil, errors.New("GeoIP archive does not contain an .mmdb file")
}

func extractMMDBFromZip(data []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(file.Name), ".mmdb") {
			continue
		}
		readCloser, err := file.Open()
		if err != nil {
			return nil, err
		}
		value, readErr := io.ReadAll(readCloser)
		closeErr := readCloser.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return value, nil
	}
	return nil, errors.New("GeoIP archive does not contain an .mmdb file")
}

func writeGeoIPDatabase(databasePath string, databaseBytes []byte) error {
	if strings.TrimSpace(databasePath) == "" {
		databasePath = geoip_setting.DatabasePath
	}
	absolutePath, err := filepath.Abs(databasePath)
	if err != nil {
		return err
	}
	directory := filepath.Dir(absolutePath)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(directory, ".geoip-*.mmdb")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(databaseBytes); err != nil {
		tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := validateGeoIPDatabaseFile(tempPath); err != nil {
		return err
	}

	closeGeoIPReaderCache(databasePath)
	closeGeoIPReaderCache(absolutePath)
	if err := os.Rename(tempPath, absolutePath); err != nil {
		if removeErr := os.Remove(absolutePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		if renameErr := os.Rename(tempPath, absolutePath); renameErr != nil {
			return renameErr
		}
	}
	return nil
}

func validateGeoIPDatabaseFile(databasePath string) error {
	reader, err := maxminddb.Open(databasePath)
	if err != nil {
		return err
	}
	reader.Close()
	return nil
}
