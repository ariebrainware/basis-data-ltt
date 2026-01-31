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
	"strconv"
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
		// Best-effort cleanup of temporary file on failure to avoid orphaned files.
		_ = os.Remove(tmpPath)
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
	tmpPath := tmpFile.Name()

	// Track success to determine if cleanup is needed
	success := false
	defer func() {
		_ = tmpFile.Close()
		// If function didn't succeed, remove the temporary file
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

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

	// Mark as successful before returning
	success = true
	return tmpPath, nil
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
// local GeoIP database with an in-memory cache. Returns an empty IPLocation
// when a lookup is not available.
func GetIPLocation(ip string) IPLocation {
	if ip == "" || isLikelyLocalOrPrivate(ip) {
		return IPLocation{}
	}
	if loc, ok := getCachedOrResolve(ip); ok {
		return loc
	}
	return IPLocation{}
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

	// Parse IP once, then perform GeoIP lookup using the parsed net.IP to avoid repeated string handling.
	netip := net.ParseIP(ip)
	if netip == nil {
		// If parsing fails, we can't resolve via geoip DB; treat as miss but avoid further processing.
		return IPLocation{}, false
	}

	loc := resolveCityCountryFromNetIP(netip)
	if loc.City == "" && loc.Country == "" {
		return IPLocation{}, false
	}
	cacheSetIP(ip, loc)
	return loc, true
}

// resolveCityCountryFromNetIP performs the GeoIP lookup for a parsed net.IP and returns
// the extracted city and country. It returns empty values on any error or
// if the lookup is unavailable.
func resolveCityCountryFromNetIP(netip net.IP) IPLocation {
	if geoipDB == nil || netip == nil {
		return IPLocation{}
	}
	rec, err := geoipDB.City(netip)
	if err != nil || rec == nil {
		return IPLocation{}
	}
	return extractCityCountry(rec)
}

// LocalizationData groups name mappings and ISO code to extract localized names with fallback.
type LocalizationData struct {
	Names   map[string]string
	IsoCode string
}

// extractLocalizedName returns the English name from the localization data,
// falling back to the ISO code when the localized name is unavailable.
func extractLocalizedName(data LocalizationData, lang string) string {
	if data.Names != nil {
		if v, ok := data.Names[lang]; ok {
			return v
		}
	}
	return data.IsoCode
}

// extractCityCountry returns the English city and country names from a GeoIP record,
// falling back to ISO country code when a localized name is unavailable.
func extractCityCountry(rec *geoip2.City) IPLocation {
	if rec == nil {
		return IPLocation{}
	}
	cityData := LocalizationData{
		Names:   rec.City.Names,
		IsoCode: "",
	}
	countryData := LocalizationData{
		Names:   rec.Country.Names,
		IsoCode: rec.Country.IsoCode,
	}
	return IPLocation{
		City:    extractLocalizedName(cityData, "en"),
		Country: extractLocalizedName(countryData, "en"),
	}
}

// isLikelyLocalOrPrivate checks whether an IP is local, private or link-local.
// It is used to short-circuit GeoIP lookups for addresses that will not have
// meaningful GeoIP information.
func isLikelyLocalOrPrivate(ip string) bool {
	if ip == "" {
		// Treat empty IP as non-routable.
		return true
	}

	parsed := net.ParseIP(ip)
	if parsed != nil {
		return isNonRoutableIP(parsed)
	}

	// Fallback for malformed or non-standard IP strings: check common private
	// IPv4 prefixes (10.*, 192.168.*) and the 172.16-31.* range.
	return hasPrivateIPv4Prefix(ip)
}

// isNonRoutableIP reports whether the parsed IP is loopback, private,
// link-local, unspecified, or otherwise non-routable.
func isNonRoutableIP(p net.IP) bool {
	return p.IsLoopback() || p.IsPrivate() || p.IsLinkLocalUnicast() || p.IsLinkLocalMulticast() || p.IsUnspecified()
}

// hasPrivateIPv4Prefix does a lightweight check for common private IPv4
// prefixes when the input couldn't be parsed into a net.IP. It accepts
// 10.*, 192.168.*, and 172.16.* - 172.31.* ranges.
func hasPrivateIPv4Prefix(ip string) bool {
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") {
		return true
	}
	if strings.HasPrefix(ip, "172.") {
		return is172PrivateRange(ip)
	}
	return false
}

// is172PrivateRange checks if an input starting with "172." falls into the
// private range 172.16.0.0 - 172.31.255.255. It performs a lightweight
// parse of the second octet and avoids deep nesting by returning early on
// errors.
func is172PrivateRange(ip string) bool {
	parts := strings.SplitN(ip, ".", 3)
	if len(parts) < 2 {
		return false
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	return n >= 16 && n <= 31
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
