package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Client resolves, downloads, verifies, and caches provider binaries.
type Client struct {
	http     *http.Client
	cacheDir string
}

// New returns a Client. cacheDir defaults to <user cache>/nixform/providers when
// empty.
func New(cacheDir string) *Client {
	if cacheDir == "" {
		if base, err := os.UserCacheDir(); err == nil {
			cacheDir = filepath.Join(base, "nixform", "providers")
		} else {
			cacheDir = filepath.Join(os.TempDir(), "nixform-providers")
		}
	}
	return &Client{
		http:     &http.Client{Timeout: 120 * time.Second},
		cacheDir: cacheDir,
	}
}

// Fetch resolves a provider address to a local, verified, cached executable
// path. On a cache hit it performs no network I/O.
func (c *Client) Fetch(ctx context.Context, address string) (string, error) {
	addr, err := ParseAddress(address)
	if err != nil {
		return "", err
	}

	// Resolve the version + download URLs (network).
	dl, err := c.Resolve(ctx, addr)
	if err != nil {
		return "", err
	}

	// Cache hit?
	if bin := c.cachedBinary(addr, dl.Version); bin != "" {
		return bin, nil
	}

	// Download the archive and the checksums.
	zipData, err := c.get(ctx, dl.DownloadURL)
	if err != nil {
		return "", fmt.Errorf("registry: download %s: %w", dl.Filename, err)
	}
	sumsBody, err := c.get(ctx, dl.ShasumsURL)
	if err != nil {
		return "", fmt.Errorf("registry: download SHA256SUMS: %w", err)
	}

	// Verify BEFORE unpacking/executing anything.
	if err := verify(zipData, dl.Filename, parseShasums(string(sumsBody))); err != nil {
		return "", err
	}

	bin, err := storeBinary(c.cacheKeyDir(addr, dl.Version), zipData)
	if err != nil {
		return "", err
	}
	return bin, nil
}

// get fetches a URL body (following redirects, which GitHub release assets use).
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s -> HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
