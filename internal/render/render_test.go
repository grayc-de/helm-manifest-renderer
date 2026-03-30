package render

import "testing"

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
