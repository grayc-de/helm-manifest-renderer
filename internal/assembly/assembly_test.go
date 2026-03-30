package assembly

import (
	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/config"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestAssembleManifests(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "tmp")
	manifestsDir := filepath.Join(tmpDir, "manifests")
	renderRoot := filepath.Join(outDir, "my-release")

	// Create dummy structure
	dirs := []string{
		filepath.Join(renderRoot, "templates"),
		filepath.Join(renderRoot, "charts", "sub1", "templates"),
		filepath.Join(renderRoot, "charts", "crds", "templates"),
		filepath.Join(renderRoot, "charts", "crds", "crds"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0755)
	}

	// Create dummy files
	os.WriteFile(filepath.Join(renderRoot, "templates", "deployment.yaml"), []byte("apiVersion: apps/v1"), 0644)
	os.WriteFile(filepath.Join(renderRoot, "templates", "crd-bad.yaml"), []byte("apiVersion: apiextensions.k8s.io/v1"), 0644)
	os.WriteFile(filepath.Join(renderRoot, "charts", "sub1", "templates", "service.yaml"), []byte("apiVersion: v1"), 0644)
	os.WriteFile(filepath.Join(renderRoot, "charts", "crds", "templates", "crd-good.yaml"), []byte("apiVersion: apiextensions.k8s.io/v1"), 0644)
	os.WriteFile(filepath.Join(renderRoot, "charts", "crds", "crds", "crd-extra.yaml"), []byte("apiVersion: apiextensions.k8s.io/v1"), 0644)
	os.WriteFile(filepath.Join(renderRoot, "secret.yaml"), []byte("apiVersion: v1"), 0644) // top level file

	config := config.ChartSourceConfig{
		PostRender: config.PostRenderConfig{
			ExcludePaths: []string{"sub1/service.yaml"},
		},
	}

	err := AssembleManifests(renderRoot, manifestsDir, config)
	if err != nil {
		t.Fatalf("AssembleManifests failed: %v", err)
	}

	// Check files
	expectedFiles := []string{
		"deployment.yaml",
		"crds/crd-extra.yaml",
		"crds/crd-good.yaml",
		"secret.yaml",
		"kustomization.yaml",
	}

	var foundFiles []string
	filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			rel, _ := filepath.Rel(manifestsDir, path)
			foundFiles = append(foundFiles, rel)
		}
		return nil
	})

	sort.Strings(expectedFiles)
	sort.Strings(foundFiles)

	if !reflect.DeepEqual(foundFiles, expectedFiles) {
		t.Errorf("Expected files %v, got %v", expectedFiles, foundFiles)
	}

	// Check kustomization.yaml content
	kustContent, _ := os.ReadFile(filepath.Join(manifestsDir, "kustomization.yaml"))
	expectedKust := "resources:\n  - crds/crd-extra.yaml\n  - crds/crd-good.yaml\n  - deployment.yaml\n  - secret.yaml\n"
	if string(kustContent) != expectedKust {
		t.Errorf("Kustomization content expected:\n%s\ngot:\n%s", expectedKust, string(kustContent))
	}
}

func TestTidyFiles(t *testing.T) {
	tmpDir := t.TempDir()
	manifestsDir := filepath.Join(tmpDir, "generated-manifests")
	crdsDir := filepath.Join(manifestsDir, "crds")

	if err := os.MkdirAll(crdsDir, 0755); err != nil {
		t.Fatalf("failed to create test directories: %v", err)
	}

	normalYaml := `apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  annotations: null
spec:
  groups:
    - rules:
        - expr: |-
            up == 1

            OR

            up == 2
`
	crdYaml := `---
# Source: kube-prometheus-stack/charts/crds/crds/crd-alertmanagerconfigs.yaml
# https://example.invalid/crd-alertmanagerconfigs.yaml
---
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.17.2
    helm.sh/chart: test

spec:

  description: "foo\n\nbar"
`

	normalPath := filepath.Join(manifestsDir, "rule.yaml")
	crdPath := filepath.Join(crdsDir, "crd.yaml")
	if err := os.WriteFile(normalPath, []byte(normalYaml), 0644); err != nil {
		t.Fatalf("failed to write normal yaml: %v", err)
	}
	if err := os.WriteFile(crdPath, []byte(crdYaml), 0644); err != nil {
		t.Fatalf("failed to write crd yaml: %v", err)
	}

	if err := TidyFiles(manifestsDir); err != nil {
		t.Fatalf("TidyFiles failed: %v", err)
	}

	normalContent, err := os.ReadFile(normalPath)
	if err != nil {
		t.Fatalf("failed to read normal yaml: %v", err)
	}
	crdContent, err := os.ReadFile(crdPath)
	if err != nil {
		t.Fatalf("failed to read crd yaml: %v", err)
	}

	if strings.Contains(string(normalContent), "annotations: null") {
		t.Errorf("expected annotations: null to be removed from normal yaml")
	}
	if strings.Contains(string(normalContent), "\n\n            OR") {
		t.Errorf("expected blank lines inside expr block to be removed, got:\n%s", string(normalContent))
	}
	if strings.Contains(string(crdContent), "controller-gen.kubebuilder.io/version") {
		t.Errorf("expected noisy CRD annotations to be removed")
	}
	if strings.HasPrefix(string(crdContent), "---\n# Source:") {
		t.Errorf("expected leading CRD comment document wrapper to be removed, got:\n%s", string(crdContent))
	}
	if strings.Contains(string(crdContent), "annotations: null") {
		t.Errorf("expected annotations: null to be removed from CRD yaml, got:\n%s", string(crdContent))
	}
	if strings.Contains(string(crdContent), "annotations:\n") {
		t.Errorf("expected empty annotations block to be removed from CRD yaml, got:\n%s", string(crdContent))
	}
	if strings.Contains(string(crdContent), "\n\n") {
		t.Errorf("expected CRD output to collapse repeated newlines, got:\n%s", string(crdContent))
	}
	if strings.Contains(string(crdContent), `\n\n`) {
		t.Errorf("expected literal CRD newline escapes to collapse, got:\n%s", string(crdContent))
	}
}
