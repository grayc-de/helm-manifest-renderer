package config

import (
	"reflect"
	"testing"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestParseChartConfig(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected ChartSourceConfig
		wantErr  bool
	}{
		{
			name: "helm source config",
			content: `sourceType: helm
releaseName: kyverno
namespace: kyverno

source:
  helm:
    repoUrl: https://kyverno.github.io/kyverno
    name: kyverno
    version: 3.7.1
    repoName: kyverno

helmArgs:
  - --no-hooks
  - --kube-version
  - 1.33

postRender:
  deleteYamlPaths:
    - metadata.labels."app.kubernetes.io/name"
  excludePaths:
    - certgen.yaml
  splitYamlDocumentsInPaths:
    - templates/crds.yaml
  movePaths:
    - from: "*.yaml"
      to: grouped
  normalizeMetadata: true`,
			expected: ChartSourceConfig{
				SourceType:  "helm",
				ReleaseName: "kyverno",
				Namespace:   "kyverno",
				Source: SourceConfig{
					Helm: &HelmSource{
						RepoURL:  "https://kyverno.github.io/kyverno",
						Name:     "kyverno",
						Version:  "3.7.1",
						RepoName: "kyverno",
					},
				},
				HelmArgs: []string{"--no-hooks", "--kube-version", "1.33"},
				PostRender: PostRenderConfig{
					DeleteYamlPaths:           []string{`metadata.labels."app.kubernetes.io/name"`},
					ExcludePaths:              []string{"certgen.yaml"},
					SplitYamlDocumentsInPaths: []string{"templates/crds.yaml"},
					MovePaths:                 []MovePathRule{{From: "*.yaml", To: "grouped"}},
					NormalizeMetadata:         boolPtr(true),
				},
			},
		},
		{
			name: "local source defaults",
			content: `sourceType: local
releaseName: metrics-server
namespace: kube-system

source:
  local:
    chartPath: ./my-chart
    version: 0.1.0`,
			expected: ChartSourceConfig{
				SourceType:  "local",
				ReleaseName: "metrics-server",
				Namespace:   "kube-system",
				Source: SourceConfig{
					Local: &LocalSource{
						ChartPath: "./my-chart",
						Version:   "0.1.0",
					},
				},
				HelmArgs: []string{},
				PostRender: PostRenderConfig{
					DeleteYamlPaths:           []string{},
					ExcludePaths:              []string{},
					SplitYamlDocumentsInPaths: []string{},
					MovePaths:                 []MovePathRule{},
					NormalizeMetadata:         boolPtr(true),
				},
			},
		},
		{
			name: "oci source config",
			content: `sourceType: oci
releaseName: envoy-gateway
namespace: ipen-infrastructure

source:
  oci:
    registry: docker.io
    path: envoyproxy
    name: gateway-helm
    version: 1.6.5`,
			expected: ChartSourceConfig{
				SourceType:  "oci",
				ReleaseName: "envoy-gateway",
				Namespace:   "ipen-infrastructure",
				Source: SourceConfig{
					OCI: &OCISource{
						Registry: "docker.io",
						Path:     "envoyproxy",
						Name:     "gateway-helm",
						Version:  "1.6.5",
					},
				},
				HelmArgs: []string{},
				PostRender: PostRenderConfig{
					DeleteYamlPaths:           []string{},
					ExcludePaths:              []string{},
					SplitYamlDocumentsInPaths: []string{},
					MovePaths:                 []MovePathRule{},
					NormalizeMetadata:         boolPtr(true),
				},
			},
		},
		{
			name: "reject unknown fields",
			content: `sourceType: helm
releaseName: kyverno
namespace: kyverno
unexpectedField: true

source:
  helm:
    repoUrl: https://kyverno.github.io/kyverno
    name: kyverno
    version: 3.7.1`,
			wantErr: true,
		},
		{
			name: "reject inactive source sections",
			content: `sourceType: helm
releaseName: test
namespace: default

source:
  helm:
    repoUrl: https://example.com/charts
    name: app
    version: 1.2.3
  local:
    chartPath: ./local`,
			wantErr: true,
		},
		{
			name: "url source config",
			content: `sourceType: url

source:
  url:
    repo: kubernetes-sigs/cluster-api
    version: v1.12.7
    asset: cluster-api-components.yaml

postRender:
  splitYamlDocumentsInPaths:
    - cluster-api-components.yaml`,
			expected: ChartSourceConfig{
				SourceType: "url",
				Source: SourceConfig{
					URL: &URLSource{
						Repo:    "kubernetes-sigs/cluster-api",
						Version: "v1.12.7",
						Asset:   "cluster-api-components.yaml",
					},
				},
				HelmArgs: []string{},
				PostRender: PostRenderConfig{
					DeleteYamlPaths:           []string{},
					ExcludePaths:              []string{},
					SplitYamlDocumentsInPaths: []string{"cluster-api-components.yaml"},
					MovePaths:                 []MovePathRule{},
					NormalizeMetadata:         boolPtr(true),
				},
			},
		},
		{
			name: "url source requires repo",
			content: `sourceType: url

source:
  url:
    version: v1.12.7
    asset: components.yaml`,
			wantErr: true,
		},
		{
			name: "url source requires version",
			content: `sourceType: url

source:
  url:
    repo: owner/repo
    asset: components.yaml`,
			wantErr: true,
		},
		{
			name: "url source requires asset",
			content: `sourceType: url

source:
  url:
    repo: owner/repo
    version: v1.0.0`,
			wantErr: true,
		},
		{
			name: "url source rejects inactive source sections",
			content: `sourceType: url

source:
  url:
    repo: owner/repo
    version: v1.0.0
    asset: components.yaml
  helm:
    repoUrl: https://example.com/charts
    name: app
    version: 1.2.3`,
			wantErr: true,
		},
		{
			name: "url source rejects releaseName and namespace",
			content: `sourceType: url
releaseName: cluster-api
namespace: capi-system

source:
  url:
    repo: owner/repo
    version: v1.0.0
    asset: components.yaml`,
			wantErr: true,
		},
		{
			name: "url source rejects helmArgs",
			content: `sourceType: url

source:
  url:
    repo: owner/repo
    version: v1.0.0
    asset: components.yaml

helmArgs:
  - --no-hooks`,
			wantErr: true,
		},
		{
			name: "helm source rejects an inactive url section",
			content: `sourceType: helm
releaseName: test
namespace: default

source:
  helm:
    repoUrl: https://example.com/charts
    name: app
    version: 1.2.3
  url:
    repo: owner/repo
    version: v1.0.0
    asset: components.yaml`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChartConfig([]byte(tt.content))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseChartConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseChartConfig()\n got: %+v\nwant: %+v", got, tt.expected)
			}
		})
	}
}
