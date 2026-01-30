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

// DownloadGeoIP downloads a GeoIP MMDB file from `url` and writes it to `destPath`.
// If the downloaded content is gzip-compressed (URL ends with .gz or the server
// indicates gzip encoding), it will be decompressed automatically. Returns the final path written.
func DownloadGeoIP(ctx context.Context, url, destPath string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			if securityLogger != nil {
				securityLogger.Printf("failed to close response body: %v", cerr)
			}
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download, status: %d", resp.StatusCode)
	}

	tmpDir := filepath.Dir(destPath)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp(tmpDir, "geoip-*.tmp")
	if err != nil {
		return "", err
	}
	// ensure file is closed; we'll close explicitly before rename
	defer func() { _ = tmpFile.Close() }()

	// Choose reader: possibly a gzip reader wrapped around resp.Body
	reader, gzCloser, err := chooseResponseReader(url, resp)
	if err != nil {
		return "", err
	}
	if gzCloser != nil {
		defer func() {
			if cerr := gzCloser.Close(); cerr != nil {
				if securityLogger != nil {
					securityLogger.Printf("failed to close gzip reader: %v", cerr)
				}
			}
		}()
	}

	if err := copyResponseToTemp(reader, tmpFile); err != nil {
		return "", err
	}

	// Ensure data is flushed and file is closed before rename.
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	if err := os.Rename(tmpFile.Name(), destPath); err != nil {
		return "", err
	}
	return destPath, nil
}

// chooseResponseReader returns an io.Reader for the response body, and an optional closer
// to be closed by the caller (e.g., gzip.Reader). It decides gzip usage based on URL
// extension or Content-Encoding header.
func chooseResponseReader(url string, resp *http.Response) (io.Reader, io.Closer, error) {
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") || filepath.Ext(url) == ".gz" {
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

	if city, country, ok := getCachedOrResolve(ip); ok {
		return city, country
	}
	return "", ""
}

// getCachedOrResolve checks cache for IP and falls back to resolving via the
// GeoIP DB. It updates cache metrics and stores successful lookups. Returns
// found=true when a city/country value is available.
func getCachedOrResolve(ip string) (string, string, bool) {
	if ccity, ccountry, ok := cacheGetIP(ip); ok {
		return ccity, ccountry, true
	}

	atomic.AddInt64(&geoipCacheMiss, 1)
	if securityLogger != nil {
		securityLogger.Printf("GeoIP cache miss for %s", ip)
	}

	city, country := resolveCityCountryFromIP(ip)
	if city == "" && country == "" {
		return "", "", false
	}
	cacheSetIP(ip, city, country)
	return city, country, true
}

// resolveCityCountryFromIP performs the GeoIP lookup for an IP and returns
// the extracted city and country. It returns empty strings on any error or
// if the lookup is unavailable.
func resolveCityCountryFromIP(ip string) (string, string) {
	if geoipDB == nil {
		return "", ""
	}
	netip := net.ParseIP(ip)
	if netip == nil {
		return "", ""
	}
	rec, err := geoipDB.City(netip)
	if err != nil || rec == nil {
		return "", ""
	}
	return extractCityCountry(rec)
}

// extractCityCountry returns the English city and country names from a GeoIP record,
// falling back to ISO country code when a localized name is unavailable.
func extractCityCountry(rec *geoip2.City) (string, string) {
	if rec == nil {
		return "", ""
	}
	var city, country string
	if rec.City.Names != nil {
		if v, ok := rec.City.Names["en"]; ok {
			city = v
		}
	}
	if rec.Country.Names != nil {
		if v, ok := rec.Country.Names["en"]; ok {
			country = v
		}
	}
	if country == "" {
		country = rec.Country.IsoCode
	}
	return city, country
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

// cacheGetIP returns cached city/country for ip when present.
func cacheGetIP(ip string) (string, string, bool) {
	if geoipCache == nil {
		return "", "", false
	}
	v, ok := geoipCache.Get(ip)
	if !ok {
		return "", "", false
	}
	atomic.AddInt64(&geoipCacheHits, 1)
	if securityLogger != nil {
		securityLogger.Printf("GeoIP cache hit for %s", ip)
	}
	arr, ok := v.([]string)
	if !ok || len(arr) != 2 {
		return "", "", false
	}
	return arr[0], arr[1], true
}

// cacheSetIP stores city/country in cache if available.
func cacheSetIP(ip, city, country string) {
	if geoipCache == nil {
		return
	}
	geoipCache.Set(ip, []string{city, country}, cache.DefaultExpiration)
	if securityLogger != nil {
		securityLogger.Printf("GeoIP cached for %s -> %s/%s", ip, city, country)
	}
}

// GetGeoIPCacheMetrics returns the cache hits and misses and current cache size.
func GetGeoIPCacheMetrics() (hits int64, misses int64, size int) {
	hits = atomic.LoadInt64(&geoipCacheHits)
	misses = atomic.LoadInt64(&geoipCacheMiss)
	if geoipCache != nil {
		return hits, misses, geoipCache.ItemCount()
	}
	return hits, misses, 0
}
