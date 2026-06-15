package registry

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestParseAddress(t *testing.T) {
	cases := []struct {
		in              string
		host, ns, name  string
		wantErr         bool
	}{
		{"hashicorp/aws", DefaultHost, "hashicorp", "aws", false},
		{"hetznercloud/hcloud", DefaultHost, "hetznercloud", "hcloud", false},
		{"registry.example.com/ns/name", "registry.example.com", "ns", "name", false},
		{"too/many/segments/here", "", "", "", true},
		{"single", "", "", "", true},
	}
	for _, c := range cases {
		a, err := ParseAddress(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseAddress(%q) expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseAddress(%q): %v", c.in, err)
			continue
		}
		if a.Host != c.host || a.Namespace != c.ns || a.Name != c.name {
			t.Errorf("ParseAddress(%q) = %+v", c.in, a)
		}
	}
}

func TestLooksLikeAddress(t *testing.T) {
	yes := []string{"hashicorp/aws", "host/ns/name"}
	no := []string{"./bin/provider-alpha", "/abs/path", "~/x", "single", "a/b/c/d"}
	for _, s := range yes {
		if !LooksLikeAddress(s) {
			t.Errorf("LooksLikeAddress(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if LooksLikeAddress(s) {
			t.Errorf("LooksLikeAddress(%q) = true, want false", s)
		}
	}
}

func TestParseShasumsAndVerify(t *testing.T) {
	data := []byte("provider archive bytes")
	sum := sha256.Sum256(data)
	hexsum := hex.EncodeToString(sum[:])
	body := hexsum + "  terraform-provider-x_1.0.0_linux_amd64.zip\n" +
		"deadbeef  terraform-provider-x_1.0.0_darwin_amd64.zip\n"

	sums := parseShasums(body)
	if sums["terraform-provider-x_1.0.0_linux_amd64.zip"] != hexsum {
		t.Fatalf("parseShasums missed the linux entry: %v", sums)
	}

	if err := verify(data, "terraform-provider-x_1.0.0_linux_amd64.zip", sums); err != nil {
		t.Errorf("verify of matching data failed: %v", err)
	}
	// tampered: wrong bytes for the same filename
	if err := verify([]byte("tampered"), "terraform-provider-x_1.0.0_linux_amd64.zip", sums); err == nil {
		t.Error("verify must reject tampered data")
	}
	// missing entry
	if err := verify(data, "terraform-provider-x_1.0.0_windows_amd64.zip", sums); err == nil {
		t.Error("verify must reject a filename with no checksum")
	}
}

// makeProviderZip builds an in-memory zip containing a terraform-provider-* exe.
func makeProviderZip(t *testing.T, exeName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(exeName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStoreBinaryUnzips(t *testing.T) {
	dir := t.TempDir()
	zipData := makeProviderZip(t, "terraform-provider-hcloud_v1.19.1", "#!fake-binary")
	bin, err := storeBinary(dir, zipData)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(bin) != "terraform-provider-hcloud_v1.19.1" {
		t.Errorf("unexpected binary name: %s", bin)
	}
	got, _ := os.ReadFile(bin)
	if string(got) != "#!fake-binary" {
		t.Errorf("extracted content = %q", got)
	}
	fi, _ := os.Stat(bin)
	if fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("extracted binary not executable: %v", fi.Mode())
	}
}

func TestStoreBinaryRejectsZipWithoutProvider(t *testing.T) {
	zipData := makeProviderZip(t, "README.txt", "not a provider")
	if _, err := storeBinary(t.TempDir(), zipData); err == nil {
		t.Error("storeBinary must fail when the zip has no terraform-provider-* executable")
	}
}

func TestCacheHitAndResolverPassthrough(t *testing.T) {
	c := New(t.TempDir())
	addr := Address{Host: DefaultHost, Namespace: "hetznercloud", Name: "hcloud"}

	// Pre-seed the cache as if a download already happened.
	dir := c.cacheKeyDir(addr, "1.19.1")
	if _, err := storeBinary(dir, makeProviderZip(t, "terraform-provider-hcloud_v1.19.1", "x")); err != nil {
		t.Fatal(err)
	}
	if got := c.cachedBinary(addr, "1.19.1"); got == "" {
		t.Fatal("expected a cache hit after seeding")
	}

	// Resolver passthrough: an existing filesystem path is returned as-is (no net).
	f := filepath.Join(t.TempDir(), "provider-local")
	if err := os.WriteFile(f, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := c.ResolveProvider(context.Background(), f)
	if err != nil || got != f {
		t.Errorf("ResolveProvider(existing path) = %q, %v; want passthrough", got, err)
	}
}
