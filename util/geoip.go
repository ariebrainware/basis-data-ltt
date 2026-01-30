package util

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/oschwald/geoip2-golang"
	cache "github.com/patrickmn/go-cache"
)

var (
	geoipDB        *geoip2.Reader
	geoipCache     *cache.Cache
	geoipCacheHits int64
	geoipCacheMiss int64
)

// IPLocation groups city and country strings to avoid primitive-pair usage.
type IPLocation struct {
	City    string
	Country string
}

// InitGeoIP initializes the local GeoIP2 database reader and an in-memory cache.
// Provide the path to a GeoIP2/GeoLite2 .mmdb file via `dbPath`.
// If dbPath is empty or the file cannot be opened, initialization is a no-op.
func InitGeoIP(dbPath string) error {
	// Allow callers to pass dbPath or fall back to env var
	if dbPath == "" {
		dbPath = os.Getenv("GEOIP_DB_PATH")
	}
	if dbPath == "" {
		return nil
	}

	r, err := geoip2.Open(dbPath)
	if err != nil {
		return err
	}
	geoipDB = r
	// Cache entries for 24h, purge every hour
	geoipCache = cache.New(24*time.Hour, 1*time.Hour)
	return nil
}

// CloseGeoIP closes the GeoIP DB if opened.
func CloseGeoIP() {
	if geoipDB != nil {
		_ = geoipDB.Close()
		geoipDB = nil
	}
}

// DownloadRequest groups download parameters to reduce string-heavy arguments.
type DownloadRequest struct {
	URL      string
	DestPath string
	TempDir  string
}

func normalizeDownloadRequest(req DownloadRequest) DownloadRequest {
	if req.TempDir == "" && req.DestPath != "" {
		req.TempDir = filepath.Dir(req.DestPath)
	}
	return req
}

// DownloadGeoIPWithRequest performs the download using a grouped request.
func DownloadGeoIPWithRequest(ctx context.Context, req DownloadRequest) (string, error) {
	req = normalizeDownloadRequest(req)
	if err := ensureDirExists(req.TempDir); err != nil {
		return "", err
	}

	tmpPath, err := downloadToTemp(ctx, req)
	if err != nil {
		return "", err
	}

	if err := os.Rename(tmpPath, req.DestPath); err != nil {
		return "", err
	}
	return req.DestPath, nil
}

// ensureDirExists creates the directory if it doesn't exist.
func ensureDirExists(dir string) error {
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// downloadToTemp performs an HTTP GET for url and writes the body to a
// temporary file in tmpDir. It returns the temporary file path which the
// caller should rename into place when ready.
func downloadToTemp(ctx context.Context, dl DownloadRequest) (string, error) {
	resp, err := getHTTPResponse(ctx, dl)
	if err != nil {
		return "", err
	}
	defer closeResponseBody(resp)
	if err := ensureOKStatus(resp); err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp(dl.TempDir, "geoip-*.tmp")
	if err != nil {
		return "", err
	}
	// ensure file is closed; we'll close explicitly before returning
	defer func() { _ = tmpFile.Close() }()

	reader, gzCloser, err := chooseResponseReader(dl, resp)
	if err != nil {
		return "", err
	}
	if gzCloser != nil {
		defer closeGzipReader(gzCloser)
	}

	if err := copyResponseToTemp(reader, tmpFile); err != nil {
		return "", err
	}

	if err := tmpFile.Close(); err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}

func getHTTPResponse(ctx context.Context, dl DownloadRequest) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", dl.URL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	return client.Do(req)
}

func ensureOKStatus(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download, status: %d", resp.StatusCode)
	}
	return nil
}

func closeResponseBody(resp *http.Response) {
	if resp == nil {
		return
	}
	if cerr := resp.Body.Close(); cerr != nil {
		if securityLogger != nil {
			securityLogger.Printf("failed to close response body: %v", cerr)
		}
	}
}

func closeGzipReader(r io.Closer) {
	if r == nil {
		return
	}
	if cerr := r.Close(); cerr != nil {
		if securityLogger != nil {
			securityLogger.Printf("failed to close gzip reader: %v", cerr)
		}
	}
}

// chooseResponseReader returns an io.Reader for the response body, and an optional closer
// to be closed by the caller (e.g., gzip.Reader). It decides gzip usage based on URL
// extension or Content-Encoding header.
func chooseResponseReader(dl DownloadRequest, resp *http.Response) (io.Reader, io.Closer, error) {
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") || filepath.Ext(dl.URL) == ".gz" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, nil, err
		}
		return gzReader, gzReader, nil
	}
	return resp.Body, nil, nil
}

// copyResponseToTemp copies from reader to tmpFile and ensures the file is synced.
func copyResponseToTemp(reader io.Reader, tmpFile *os.File) error {
	if _, err := io.Copy(tmpFile, reader); err != nil {
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		return err
	}
	return nil
}

// ValidateGeoIP attempts to open the MMDB file to ensure it's a valid DB.
func ValidateGeoIP(path string) error {
	r, err := geoip2.Open(path)
	if err != nil {
		return err
	}
	_ = r.Close()
	return nil
}

// GetIPLocation returns city and country name for the provided IP using the
// local GeoIP database with an in-memory cache. Returns empty strings when
// a lookup is not available.
func GetIPLocation(ip string) (string, string) {
	if ip == "" || isLikelyLocalOrPrivate(ip) {
		return "", ""
	}
	if loc, ok := getCachedOrResolve(ip); ok {
		return loc.City, loc.Country
	}
	return "", ""
}

// getCachedOrResolve checks cache for IP and falls back to resolving via the
// GeoIP DB. It updates cache metrics and stores successful lookups. Returns
// found=true when a city/country value is available.
func getCachedOrResolve(ip string) (IPLocation, bool) {
	if loc, ok := cacheGetIP(ip); ok {
		return loc, true
	}

	atomic.AddInt64(&geoipCacheMiss, 1)
	if securityLogger != nil {
		securityLogger.Printf("GeoIP cache miss for %s", ip)
	}

	loc := resolveCityCountryFromIP(ip)
	if loc.City == "" && loc.Country == "" {
		return IPLocation{}, false
	}
	cacheSetIP(ip, loc)
	return loc, true
}

// resolveCityCountryFromIP performs the GeoIP lookup for an IP and returns
// the extracted city and country. It returns empty values on any error or
// if the lookup is unavailable.
func resolveCityCountryFromIP(ip string) IPLocation {
	if geoipDB == nil {
		return IPLocation{}
	}
	netip := net.ParseIP(ip)
	if netip == nil {
		return IPLocation{}
	}
	rec, err := geoipDB.City(netip)
	if err != nil || rec == nil {
		return IPLocation{}
	}
	return extractCityCountry(rec)
}

// extractCityCountry returns the English city and country names from a GeoIP record,
// falling back to ISO country code when a localized name is unavailable.
func extractCityCountry(rec *geoip2.City) IPLocation {
	if rec == nil {
		return IPLocation{}
	}
	loc := IPLocation{
		City:    localizedName(rec.City.Names, "en"),
		Country: localizedName(rec.Country.Names, "en"),
	}
	loc.Country = fallbackCountry(loc.Country, rec.Country.IsoCode)
	return loc
}

func localizedName(names map[string]string, lang string) string {
	if names == nil {
		return ""
	}
	if v, ok := names[lang]; ok {
		return v
	}
	return ""
}

func fallbackCountry(name, iso string) string {
	if name != "" {
		return name
	}
	return iso
}

// isLikelyLocalOrPrivate checks a few common local/private IP forms quickly.
func isLikelyLocalOrPrivate(ip string) bool {
	if ip == "" {
		return true
	}
	// canonical simple checks
	if ip == "127.0.0.1" || ip == "::1" {
		return true
	}
	prefixes := []string{"10.", "192.168", "::"}
	for _, p := range prefixes {
		if strings.HasPrefix(ip, p) {
			return true
		}
	}
	return false
}

// cacheGetIP returns a cached IPLocation for ip when present.
func cacheGetIP(ip string) (IPLocation, bool) {
	if geoipCache == nil {
		return IPLocation{}, false
	}
	v, ok := geoipCache.Get(ip)
	if !ok {
		return IPLocation{}, false
	}
	atomic.AddInt64(&geoipCacheHits, 1)
	if securityLogger != nil {
		securityLogger.Printf("GeoIP cache hit for %s", ip)
	}
	loc, ok := v.(IPLocation)
	if !ok {
		return IPLocation{}, false
	}
	return loc, true
}

// cacheSetIP stores an IPLocation in cache if available.
func cacheSetIP(ip string, loc IPLocation) {
	if geoipCache == nil {
		return
	}
	geoipCache.Set(ip, loc, cache.DefaultExpiration)
	if securityLogger != nil {
		securityLogger.Printf("GeoIP cached for %s -> %s/%s", ip, loc.City, loc.Country)
	}
}

// GetGeoIPCacheMetrics returns the cache hits and misses and current cache size.
func GetGeoIPCacheMetrics() (hits int64, misses int64, size int) {
	hits = atomic.LoadInt64(&geoipCacheHits)
	misses = atomic.LoadInt64(&geoipCacheMiss)
	return hits, misses, cacheItemCount()
}

func cacheItemCount() int {
	if geoipCache == nil {
		return 0
	}
	return geoipCache.ItemCount()
}
