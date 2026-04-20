package config

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type PostRenderConfig struct {
	DeleteYamlPaths           []string       `yaml:"deleteYamlPaths"`
	ExcludePaths              []string       `yaml:"excludePaths"`
	SplitYamlDocumentsInPaths []string       `yaml:"splitYamlDocumentsInPaths"`
	MovePaths                 []MovePathRule `yaml:"movePaths"`
	NormalizeMetadata         *bool          `yaml:"normalizeMetadata"`
}

type MovePathRule struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type LocalSource struct {
	ChartPath string `yaml:"chartPath"`
	Version   string `yaml:"version"`
}

type HelmSource struct {
	RepoURL  string `yaml:"repoUrl"`
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	RepoName string `yaml:"repoName"`
}

type OCISource struct {
	Registry string `yaml:"registry"`
	Path     string `yaml:"path"`
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
}

type SourceConfig struct {
	Local *LocalSource `yaml:"local"`
	Helm  *HelmSource  `yaml:"helm"`
	OCI   *OCISource   `yaml:"oci"`
}

type ChartSourceConfig struct {
	SourceType  string           `yaml:"sourceType"`
	ReleaseName string           `yaml:"releaseName"`
	Namespace   string           `yaml:"namespace"`
	Source      SourceConfig     `yaml:"source"`
	HelmArgs    []string         `yaml:"helmArgs"`
	PostRender  PostRenderConfig `yaml:"postRender"`
}

func ParseChartConfig(content []byte) (ChartSourceConfig, error) {
	var c ChartSourceConfig
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&c); err != nil {
		return ChartSourceConfig{}, err
	}

	c.SourceType = strings.ToLower(strings.TrimSpace(c.SourceType))
	if c.SourceType == "" {
		return ChartSourceConfig{}, fmt.Errorf("sourceType must be set")
	}

	switch c.SourceType {
	case "local":
		if c.Source.Local == nil {
			return ChartSourceConfig{}, fmt.Errorf("source.local must be set for sourceType=local")
		}
		if strings.TrimSpace(c.Source.Local.ChartPath) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.local.chartPath must be set")
		}
		if c.Source.Helm != nil || c.Source.OCI != nil {
			return ChartSourceConfig{}, fmt.Errorf("source contains inactive sections for sourceType=local")
		}
	case "helm":
		if c.Source.Helm == nil {
			return ChartSourceConfig{}, fmt.Errorf("source.helm must be set for sourceType=helm")
		}
		if strings.TrimSpace(c.Source.Helm.RepoURL) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.helm.repoUrl must be set")
		}
		if strings.TrimSpace(c.Source.Helm.Name) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.helm.name must be set")
		}
		if strings.TrimSpace(c.Source.Helm.Version) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.helm.version must be set")
		}
		if strings.TrimSpace(c.Source.Helm.RepoName) == "" {
			c.Source.Helm.RepoName = "helmrepo"
		}
		if c.Source.Local != nil || c.Source.OCI != nil {
			return ChartSourceConfig{}, fmt.Errorf("source contains inactive sections for sourceType=helm")
		}
	case "oci":
		if c.Source.OCI == nil {
			return ChartSourceConfig{}, fmt.Errorf("source.oci must be set for sourceType=oci")
		}
		if strings.TrimSpace(c.Source.OCI.Registry) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.oci.registry must be set")
		}
		if strings.TrimSpace(c.Source.OCI.Path) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.oci.path must be set")
		}
		if strings.TrimSpace(c.Source.OCI.Name) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.oci.name must be set")
		}
		if strings.TrimSpace(c.Source.OCI.Version) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.oci.version must be set")
		}
		if c.Source.Local != nil || c.Source.Helm != nil {
			return ChartSourceConfig{}, fmt.Errorf("source contains inactive sections for sourceType=oci")
		}
	default:
		return ChartSourceConfig{}, fmt.Errorf("sourceType must be one of: local, helm, oci")
	}

	if strings.TrimSpace(c.ReleaseName) == "" {
		return ChartSourceConfig{}, fmt.Errorf("releaseName must be set")
	}
	if strings.TrimSpace(c.Namespace) == "" {
		return ChartSourceConfig{}, fmt.Errorf("namespace must be set")
	}

	if c.HelmArgs == nil {
		c.HelmArgs = []string{}
	}
	if c.PostRender.DeleteYamlPaths == nil {
		c.PostRender.DeleteYamlPaths = []string{}
	}
	if c.PostRender.ExcludePaths == nil {
		c.PostRender.ExcludePaths = []string{}
	}
	if c.PostRender.SplitYamlDocumentsInPaths == nil {
		c.PostRender.SplitYamlDocumentsInPaths = []string{}
	}
	if c.PostRender.MovePaths == nil {
		c.PostRender.MovePaths = []MovePathRule{}
	}
	if c.PostRender.NormalizeMetadata == nil {
		defaultValue := true
		c.PostRender.NormalizeMetadata = &defaultValue
	}
	for i, rule := range c.PostRender.MovePaths {
		if strings.TrimSpace(rule.From) == "" {
			return ChartSourceConfig{}, fmt.Errorf("postRender.movePaths[%d].from must be set", i)
		}
		if strings.TrimSpace(rule.To) == "" {
			return ChartSourceConfig{}, fmt.Errorf("postRender.movePaths[%d].to must be set", i)
		}
	}

	return c, nil
}
