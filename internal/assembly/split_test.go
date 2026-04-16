package assembly

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestSplitYamlDocumentsSplitsConfiguredFile(t *testing.T) {
	manifestsDir := t.TempDir()
	crdsDir := filepath.Join(manifestsDir, "crds")
	if err := os.MkdirAll(crdsDir, 0755); err != nil {
		t.Fatalf("failed to create crds directory: %v", err)
	}

	content := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: widgets-reader
`
	sourcePath := filepath.Join(crdsDir, "bundle.yaml")
	if err := os.WriteFile(sourcePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write source yaml: %v", err)
	}

	if err := SplitYamlDocuments(manifestsDir, []string{"crds/bundle.yaml"}); err != nil {
		t.Fatalf("SplitYamlDocuments failed: %v", err)
	}

	entries, err := os.ReadDir(crdsDir)
	if err != nil {
		t.Fatalf("failed to read crds directory: %v", err)
	}

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	expected := []string{
		"bundle--clusterrole--widgets-reader.yaml",
		"widgets-example-com.yaml",
	}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("unexpected split files: got %v want %v", names, expected)
	}
}

func TestSplitYamlDocumentsSplitsYamlFilesInConfiguredDirectory(t *testing.T) {
	manifestsDir := t.TempDir()
	crdsDir := filepath.Join(manifestsDir, "crds")
	if err := os.MkdirAll(crdsDir, 0755); err != nil {
		t.Fatalf("failed to create crds directory: %v", err)
	}

	content := `apiVersion: v1
kind: ConfigMap
metadata:
  name: first
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: second
`
	sourcePath := filepath.Join(crdsDir, "all.yaml")
	if err := os.WriteFile(sourcePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write source yaml: %v", err)
	}

	if err := SplitYamlDocuments(manifestsDir, []string{"crds"}); err != nil {
		t.Fatalf("SplitYamlDocuments failed: %v", err)
	}

	entries, err := os.ReadDir(crdsDir)
	if err != nil {
		t.Fatalf("failed to read crds directory: %v", err)
	}

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	expected := []string{
		"all--configmap--first.yaml",
		"all--configmap--second.yaml",
	}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("unexpected split files: got %v want %v", names, expected)
	}
}

func TestSplitYamlDocumentsKeepsSingleDocumentFile(t *testing.T) {
	manifestsDir := t.TempDir()
	templatesDir := filepath.Join(manifestsDir, "crds")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create crds directory: %v", err)
	}

	sourcePath := filepath.Join(templatesDir, "deployment.yaml")
	if err := os.WriteFile(sourcePath, []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\n"), 0644); err != nil {
		t.Fatalf("failed to write source yaml: %v", err)
	}

	if err := SplitYamlDocuments(manifestsDir, []string{"crds/deployment.yaml"}); err != nil {
		t.Fatalf("SplitYamlDocuments failed: %v", err)
	}

	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("expected original single-document file to remain, got: %v", err)
	}
}
