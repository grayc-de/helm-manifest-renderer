package yamlcleaner

import "testing"

func TestCleanYaml(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		options  Options
		expected string
	}{
		{
			name: "remove helm and app labels when normalization is enabled",
			input: `apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/name: my-app
    app.kubernetes.io/version: 1.0.0
    app.kubernetes.io/instance: my-release
    app.kubernetes.io/managed-by: Helm
    helm.sh/chart: my-chart-1.0.0
    keep: me
spec:
  ports:
    - port: 80
`,
			options: Options{
				DeletePaths:       []string{},
				NormalizeMetadata: true,
			},
			expected: `apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/name: my-app
    keep: me
spec:
  ports:
    - port: 80
`,
		},
		{
			name: "keep metadata when normalization is disabled",
			input: `apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/version: 1.0.0
    keep: me
`,
			options: Options{
				DeletePaths:       []string{},
				NormalizeMetadata: false,
			},
			expected: `apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/version: 1.0.0
    keep: me
`,
		},
		{
			name: "preserve empty string entries in sequences",
			input: `rules:
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
`,
			options: Options{
				DeletePaths:       []string{},
				NormalizeMetadata: true,
			},
			expected: `rules:
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
`,
		},
		{
			name: "preserve multiple documents",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: first
---
apiVersion: grafana.integreatly.org/v1beta1
kind: GrafanaDashboard
metadata:
  name: second
`,
			options: Options{
				DeletePaths:       []string{},
				NormalizeMetadata: true,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: first
---
apiVersion: grafana.integreatly.org/v1beta1
kind: GrafanaDashboard
metadata:
  name: second
`,
		},
		{
			name: "preserve subresources status in crd schemas",
			input: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
spec:
  versions:
    - name: v1alpha1
      subresources:
        status: {}
`,
			options: Options{
				DeletePaths:       []string{},
				NormalizeMetadata: true,
			},
			expected: `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
spec:
  versions:
    - name: v1alpha1
      subresources:
        status: {}
`,
		},
		{
			name: "remove empty metadata blocks",
			input: `apiVersion: v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/version: 1.0.0
  annotations:
    helm.sh/resource-policy: keep
status:
  conditions: []
  storedVersions: []
`,
			options: Options{
				DeletePaths:       []string{},
				NormalizeMetadata: true,
			},
			expected: `apiVersion: v1
kind: Deployment
`,
		},
		{
			name: "apply custom deleteYamlPaths",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/name: my-app
    custom/label: remove-me
data:
  config: "value"
`,
			options: Options{
				DeletePaths:       []string{`metadata.labels."app.kubernetes.io/name"`, `metadata.labels."custom/label"`},
				NormalizeMetadata: true,
			},
			expected: `apiVersion: v1
kind: ConfigMap
data:
  config: "value"
`,
		},
		{
			name: "remove httpHeaders from deployment only when normalization is enabled",
			input: `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: app
          livenessProbe:
            httpGet:
              path: /
              httpHeaders:
                - name: X-Custom-Header
                  value: Awesome
`,
			options: Options{
				DeletePaths:       []string{},
				NormalizeMetadata: true,
			},
			expected: `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: app
          livenessProbe:
            httpGet:
              path: /
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, err := CleanYaml([]byte(tt.input), tt.options)
			if err != nil {
				t.Fatalf("CleanYaml failed: %v", err)
			}
			if string(cleaned) != tt.expected {
				t.Errorf("CleanYaml()\nGot:\n%s\nWant:\n%s", string(cleaned), tt.expected)
			}
		})
	}
}
