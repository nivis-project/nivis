package registry

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// cacheKeyDir is the per-provider-version-platform cache directory under the
// client's cache root.
func (c *Client) cacheKeyDir(addr Address, version string) string {
	return filepath.Join(c.cacheDir, addr.Host, addr.Namespace, addr.Name, version,
		runtime.GOOS+"_"+runtime.GOARCH)
}

// cachedBinary returns the path to a cached provider executable for this
// address/version, or "" if not cached.
func (c *Client) cachedBinary(addr Address, version string) string {
	dir := c.cacheKeyDir(addr, version)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "terraform-provider-") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// storeBinary unzips the verified archive into the cache key dir, writes the
// provider executable 0755, and returns its path.
func storeBinary(dir string, zipData []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("registry: mkdir cache: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", fmt.Errorf("registry: open zip: %w", err)
	}
	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if !strings.HasPrefix(name, "terraform-provider-") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("registry: read %q from zip: %w", name, err)
		}
		out := filepath.Join(dir, name)
		dst, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("registry: create %q: %w", out, err)
		}
		if _, err := io.Copy(dst, rc); err != nil { //nolint:gosec // provider zip from a verified, checksum-matched archive
			rc.Close()
			dst.Close()
			return "", fmt.Errorf("registry: extract %q: %w", out, err)
		}
		rc.Close()
		dst.Close()
		return out, nil
	}
	return "", fmt.Errorf("registry: no terraform-provider-* executable in archive")
}
