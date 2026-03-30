package helm

import (
	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/config"
	"reflect"
	"testing"
)

func TestGenerateHelmCommands(t *testing.T) {
	tests := []struct {
		name       string
		config     config.ChartSourceConfig
		outDir     string
		valuesFile string
		expected   [][]string
		wantErr    bool
	}{
		{
			name: "local source",
			config: config.ChartSourceConfig{
				SourceType:  "local",
				ReleaseName: "my-release",
				Namespace:   "my-ns",
				Source: config.SourceConfig{
					Local: &config.LocalSource{ChartPath: "./my-chart"},
				},
			},
			outDir:     "tmp",
			valuesFile: "values.yaml",
			expected: [][]string{
				{"helm", "template", "my-release", "./my-chart", "--namespace", "my-ns", "--include-crds", "--skip-tests", "--dependency-update", "--output-dir", "tmp", "--values", "values.yaml"},
			},
		},
		{
			name: "helm source",
			config: config.ChartSourceConfig{
				SourceType:  "helm",
				ReleaseName: "kyverno",
				Namespace:   "kyverno",
				Source: config.SourceConfig{
					Helm: &config.HelmSource{
						RepoURL:  "https://kyverno.github.io/kyverno",
						Name:     "kyverno",
						Version:  "3.7.1",
						RepoName: "kyverno",
					},
				},
				HelmArgs: []string{"--no-hooks", "--kube-version", "1.33"},
			},
			outDir:     "tmp",
			valuesFile: "values.yaml",
			expected: [][]string{
				{"helm", "repo", "add", "--force-update", "kyverno", "https://kyverno.github.io/kyverno"},
				{"helm", "repo", "update"},
				{"helm", "template", "kyverno", "kyverno/kyverno", "--namespace", "kyverno", "--version", "3.7.1", "--include-crds", "--skip-tests", "--output-dir", "tmp", "--values", "values.yaml", "--no-hooks", "--kube-version", "1.33"},
			},
		},
		{
			name: "oci source",
			config: config.ChartSourceConfig{
				SourceType:  "oci",
				ReleaseName: "envoy-gateway",
				Namespace:   "ipen-infrastructure",
				Source: config.SourceConfig{
					OCI: &config.OCISource{
						Registry: "docker.io",
						Path:     "envoyproxy",
						Name:     "gateway-helm",
						Version:  "1.6.5",
					},
				},
			},
			outDir:     "tmp",
			valuesFile: "values.yaml",
			expected: [][]string{
				{"helm", "template", "envoy-gateway", "oci://docker.io/envoyproxy/gateway-helm", "--namespace", "ipen-infrastructure", "--version", "1.6.5", "--include-crds", "--skip-tests", "--output-dir", "tmp", "--values", "values.yaml"},
			},
		},
		{
			name: "helm source without values file uses chart defaults",
			config: config.ChartSourceConfig{
				SourceType:  "helm",
				ReleaseName: "metrics-server",
				Namespace:   "kube-system",
				Source: config.SourceConfig{
					Helm: &config.HelmSource{
						RepoURL:  "https://kubernetes-sigs.github.io/metrics-server",
						Name:     "metrics-server",
						Version:  "3.13.0",
						RepoName: "metrics-server",
					},
				},
			},
			outDir:     "tmp",
			valuesFile: "",
			expected: [][]string{
				{"helm", "repo", "add", "--force-update", "metrics-server", "https://kubernetes-sigs.github.io/metrics-server"},
				{"helm", "repo", "update"},
				{"helm", "template", "metrics-server", "metrics-server/metrics-server", "--namespace", "kube-system", "--version", "3.13.0", "--include-crds", "--skip-tests", "--output-dir", "tmp"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateHelmCommands(tt.config, tt.outDir, tt.valuesFile)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateHelmCommands() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GenerateHelmCommands()\n got: %v\nwant: %v", got, tt.expected)
			}
		})
	}
}
