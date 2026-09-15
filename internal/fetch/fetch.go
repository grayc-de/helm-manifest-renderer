// Package fetch downloads a released manifest file for sourceType=url.
//
// The one rule that matters: a non-2xx response is a hard error. kustomize's
// own remote-file loader answers an HTTP error by reinterpreting the URL as a
// git repository, which is what made a GitHub blip read as a malformed URL
// (GDO-336). Nothing here falls back to anything.
package fetch

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/config"
)

// DefaultBaseURL is the only host releases are fetched from today.
const DefaultBaseURL = "https://github.com"

// DefaultTimeout bounds Client's requests. The default http.Client has no
// timeout at all, so a stalled connection would hang a CI render step
// forever with no output. The largest real asset (cluster-api components,
// 3.2 MiB) fetched in 11s during end-to-end testing, so two minutes is ample
// headroom while still guaranteeing a render eventually fails loudly instead
// of hanging.
const DefaultTimeout = 2 * time.Minute

// Client is the HTTP client used for all fetches. It is a package-level var,
// not a const, so tests can substitute a client with a much shorter timeout
// rather than waiting out DefaultTimeout.
var Client = &http.Client{Timeout: DefaultTimeout}

// AssetURL builds the release-asset URL for src.
func AssetURL(baseURL string, src config.URLSource) string {
	return fmt.Sprintf("%s/%s/releases/download/%s/%s",
		strings.TrimSuffix(baseURL, "/"), src.Repo, src.Version, src.Asset)
}

// Fetch downloads src into destDir from DefaultBaseURL.
func Fetch(src config.URLSource, destDir string) error {
	return FetchFrom(DefaultBaseURL, src, destDir)
}

// FetchFrom downloads src into destDir from baseURL, creating destDir if
// needed. The file is named after the asset's base name.
func FetchFrom(baseURL string, src config.URLSource, destDir string) error {
	url := AssetURL(baseURL, src)

	resp, err := Client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("fetch %s: status %d (%s)",
			url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", destDir, err)
	}

	target := filepath.Join(destDir, filepath.Base(src.Asset))
	file, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		os.Remove(target)
		return fmt.Errorf("write %s: %w", target, err)
	}

	return nil
}
