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
// If the downloaded content is gzip-compressed (URL ends with .gz), it will be
// decompressed automatically. Returns the final path written.
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
	defer resp.Body.Close()
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
	defer func() { _ = tmpFile.Close() }()

	// If URL looks like a gzipped file, decompress on the fly
	if filepath.Ext(url) == ".gz" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", err
		}
		defer gzReader.Close()
		if _, err := io.Copy(tmpFile, gzReader); err != nil {
			return "", err
		}
	} else {
		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			return "", err
		}
	}

	if err := tmpFile.Sync(); err != nil {
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	if err := os.Rename(tmpFile.Name(), destPath); err != nil {
		return "", err
	}
	return destPath, nil
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
	if ip == "" {
		return "", ""
	}

	// Skip common private/local ranges quickly
	if ip == "127.0.0.1" || ip == "::1" ||
		(len(ip) >= 4 && ip[:4] == "10.") ||
		(len(ip) >= 8 && ip[:8] == "192.168") ||
		(len(ip) >= 2 && ip[:2] == "::") {
		return "", ""
	}

	if geoipCache != nil {
		if v, ok := geoipCache.Get(ip); ok {
			atomic.AddInt64(&geoipCacheHits, 1)
			// Log cache hit for observability
			if securityLogger != nil {
				securityLogger.Printf("GeoIP cache hit for %s", ip)
			}
			if arr, ok := v.([]string); ok && len(arr) == 2 {
				return arr[0], arr[1]
			}
		}
	}
	atomic.AddInt64(&geoipCacheMiss, 1)
	if securityLogger != nil {
		securityLogger.Printf("GeoIP cache miss for %s", ip)
	}

	if geoipDB == nil {
		return "", ""
	}

	netip := net.ParseIP(ip)
	if netip == nil {
		return "", ""
	}

	rec, err := geoipDB.City(netip)
	if err != nil {
		return "", ""
	}

	city := ""
	country := ""
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

	if geoipCache != nil {
		geoipCache.Set(ip, []string{city, country}, cache.DefaultExpiration)
		if securityLogger != nil {
			securityLogger.Printf("GeoIP cached for %s -> %s/%s", ip, city, country)
		}
	}

	return city, country
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
