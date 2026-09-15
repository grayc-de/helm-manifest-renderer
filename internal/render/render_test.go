package render

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/config"
)

func TestShouldSkipStructuredCleanup(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "skip raw chart crds",
			path: "/tmp/render/release/charts/crds/crds/crd-alertmanagerconfigs.yaml",
			want: true,
		},
		{
			name: "do not skip normal template file",
			path: "/tmp/render/release/templates/deployment.yaml",
			want: false,
		},
		{
			name: "do not skip assembled crd output",
			path: "/tmp/generated-manifests/crds/crd-alertmanagerconfigs.yaml",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipStructuredCleanup(tt.path)
			if got != tt.want {
				t.Fatalf("shouldSkipStructuredCleanup(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestStage(t *testing.T) {
	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	stage(false, "disabled")
	stage(true, "enabled")

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}

	var output bytes.Buffer
	if _, err := io.Copy(&output, reader); err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}

	logged := output.String()
	if strings.Contains(logged, "disabled") {
		t.Fatalf("did not expect disabled stage to be logged, got: %q", logged)
	}
	if !strings.Contains(logged, "[stage] enabled") {
		t.Fatalf("expected enabled stage log, got: %q", logged)
	}
}

func TestMaterializeSourceFetchesURLSources(t *testing.T) {
	original := fetchAsset
	defer func() { fetchAsset = original }()

	var gotSrc config.URLSource
	var gotDest string
	fetchAsset = func(src config.URLSource, destDir string) error {
		gotSrc = src
		gotDest = destDir
		return nil
	}

	cfg := config.ChartSourceConfig{
		SourceType: "url",
		Source: config.SourceConfig{
			URL: &config.URLSource{
				Repo:    "kubernetes-sigs/cluster-api",
				Version: "v1.12.7",
				Asset:   "cluster-api-components.yaml",
			},
		},
	}

	tempDir := t.TempDir()
	if err := materializeSource(cfg, tempDir, "", nil); err != nil {
		t.Fatalf("materializeSource() error = %v", err)
	}

	if gotSrc.Repo != "kubernetes-sigs/cluster-api" {
		t.Errorf("fetched the wrong source: %+v", gotSrc)
	}
	want := filepath.Join(tempDir, "cluster-api")
	if gotDest != want {
		t.Errorf("dest = %s, want %s", gotDest, want)
	}
}

func TestMaterializeSourceReportsFetchFailure(t *testing.T) {
	original := fetchAsset
	defer func() { fetchAsset = original }()

	fetchAsset = func(src config.URLSource, destDir string) error {
		return fmt.Errorf("status 404 (Not Found)")
	}

	cfg := config.ChartSourceConfig{
		SourceType: "url",
		Source: config.SourceConfig{
			URL: &config.URLSource{Repo: "o/r", Version: "v1", Asset: "a.yaml"},
		},
	}

	err := materializeSource(cfg, t.TempDir(), "", nil)
	if err == nil {
		t.Fatal("materializeSource() error = nil, want the fetch failure")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error lost the cause: %v", err)
	}
}
