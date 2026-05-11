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
	os.WriteFile(filepath.Join(renderRoot, "charts", "crds", "templates", "bundle.yaml"), []byte("apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: widgets.example.com\n---\napiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: gadgets.example.com\n"), 0644)
	os.WriteFile(filepath.Join(renderRoot, "secret.yaml"), []byte("apiVersion: v1"), 0644) // top level file

	config := config.ChartSourceConfig{
		PostRender: config.PostRenderConfig{
			ExcludePaths:              []string{"sub1/service.yaml"},
			SplitYamlDocumentsInPaths: []string{"crds/bundle.yaml"},
		},
	}

	err := AssembleManifests(renderRoot, manifestsDir, config, nil)
	if err != nil {
		t.Fatalf("AssembleManifests failed: %v", err)
	}

	// Check files
	expectedFiles := []string{
		"crds/gadgets-example-com.yaml",
		"crds/widgets-example-com.yaml",
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
	expectedKust := "resources:\n  - crds/crd-extra.yaml\n  - crds/crd-good.yaml\n  - crds/gadgets-example-com.yaml\n  - crds/widgets-example-com.yaml\n  - deployment.yaml\n  - secret.yaml\n"
	if string(kustContent) != expectedKust {
		t.Errorf("Kustomization content expected:\n%s\ngot:\n%s", expectedKust, string(kustContent))
	}
}

func TestAssembleManifestsAppliesExcludePathsAfterSplit(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "tmp")
	manifestsDir := filepath.Join(tmpDir, "manifests")
	renderRoot := filepath.Join(outDir, "my-release")

	if err := os.MkdirAll(filepath.Join(renderRoot, "charts", "crds", "templates"), 0755); err != nil {
		t.Fatalf("failed to create render tree: %v", err)
	}

	bundle := "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: widgets.example.com\n---\napiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: gadgets.example.com\n"
	if err := os.WriteFile(filepath.Join(renderRoot, "charts", "crds", "templates", "bundle.yaml"), []byte(bundle), 0644); err != nil {
		t.Fatalf("failed to write bundle yaml: %v", err)
	}

	config := config.ChartSourceConfig{
		PostRender: config.PostRenderConfig{
			SplitYamlDocumentsInPaths: []string{"crds/bundle.yaml"},
			ExcludePaths:              []string{"crds/widgets-example-com.yaml"},
		},
	}

	if err := AssembleManifests(renderRoot, manifestsDir, config, nil); err != nil {
		t.Fatalf("AssembleManifests failed: %v", err)
	}

	expectedFiles := []string{
		"crds/gadgets-example-com.yaml",
		"kustomization.yaml",
	}

	var foundFiles []string
	filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(manifestsDir, path)
			foundFiles = append(foundFiles, rel)
		}
		return nil
	})

	sort.Strings(foundFiles)
	if !reflect.DeepEqual(foundFiles, expectedFiles) {
		t.Fatalf("expected files %v, got %v", expectedFiles, foundFiles)
	}

	kustContent, err := os.ReadFile(filepath.Join(manifestsDir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("failed to read kustomization.yaml: %v", err)
	}

	expectedKust := "resources:\n  - crds/gadgets-example-com.yaml\n"
	if string(kustContent) != expectedKust {
		t.Fatalf("Kustomization content expected:\n%s\ngot:\n%s", expectedKust, string(kustContent))
	}
}

func TestAssembleManifestsAppliesExcludePathsAfterMovePaths(t *testing.T) {
	tmpDir := t.TempDir()
	manifestsDir := filepath.Join(tmpDir, "manifests")
	renderRoot := filepath.Join(tmpDir, "render", "release")

	if err := os.MkdirAll(filepath.Join(renderRoot, "templates"), 0755); err != nil {
		t.Fatalf("failed to create render tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(renderRoot, "templates", "alpha.yaml"), []byte("apiVersion: v1"), 0644); err != nil {
		t.Fatalf("failed to write alpha.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(renderRoot, "templates", "beta.yaml"), []byte("apiVersion: v1"), 0644); err != nil {
		t.Fatalf("failed to write beta.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(renderRoot, "templates", "keep.txt"), []byte("ignored"), 0644); err != nil {
		t.Fatalf("failed to write keep.txt: %v", err)
	}

	config := config.ChartSourceConfig{
		PostRender: config.PostRenderConfig{
			MovePaths: []config.MovePathRule{
				{From: "alpha.yaml", To: "group-a"},
				{From: "*.yaml", To: "group-b"},
			},
			ExcludePaths: []string{"group-b/beta.yaml"},
		},
	}

	if err := AssembleManifests(renderRoot, manifestsDir, config, nil); err != nil {
		t.Fatalf("AssembleManifests failed: %v", err)
	}

	expectedFiles := []string{
		"group-a/alpha.yaml",
		"keep.txt",
		"kustomization.yaml",
	}

	var foundFiles []string
	filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(manifestsDir, path)
			foundFiles = append(foundFiles, rel)
		}
		return nil
	})

	sort.Strings(foundFiles)
	if !reflect.DeepEqual(foundFiles, expectedFiles) {
		t.Fatalf("expected files %v, got %v", expectedFiles, foundFiles)
	}

	kustContent, err := os.ReadFile(filepath.Join(manifestsDir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("failed to read kustomization.yaml: %v", err)
	}

	expectedKust := "resources:\n  - group-a/alpha.yaml\n"
	if string(kustContent) != expectedKust {
		t.Fatalf("Kustomization content expected:\n%s\ngot:\n%s", expectedKust, string(kustContent))
	}
}

func TestAssembleManifestsRemovesEmptyDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	manifestsDir := filepath.Join(tmpDir, "manifests")
	renderRoot := filepath.Join(tmpDir, "render", "release")

	templatesDir := filepath.Join(renderRoot, "charts", "crds", "templates", "example.io")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create render tree: %v", err)
	}

	crd := "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: widgets.example.io\n"
	if err := os.WriteFile(filepath.Join(templatesDir, "widget.yaml"), []byte(crd), 0644); err != nil {
		t.Fatalf("failed to write crd yaml: %v", err)
	}

	config := config.ChartSourceConfig{}

	if err := AssembleManifests(renderRoot, manifestsDir, config, nil); err != nil {
		t.Fatalf("AssembleManifests failed: %v", err)
	}

	for _, dir := range []string{
		filepath.Join(manifestsDir, "templates"),
		filepath.Join(manifestsDir, "templates", "example.io"),
	} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("expected empty directory %s to be removed, stat error: %v", dir, err)
		}
	}

	expectedFiles := []string{
		"crds/example.io/widget.yaml",
		"kustomization.yaml",
	}

	var foundFiles []string
	filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(manifestsDir, path)
			foundFiles = append(foundFiles, rel)
		}
		return nil
	})

	sort.Strings(foundFiles)
	if !reflect.DeepEqual(foundFiles, expectedFiles) {
		t.Fatalf("expected files %v, got %v", expectedFiles, foundFiles)
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
