package render

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
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
