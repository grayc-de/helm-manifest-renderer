package helm

import (
	"fmt"
	"strings"

	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/config"
)

func GenerateHelmCommands(cfg config.ChartSourceConfig, outDir string, valuesFile string) ([][]string, error) {
	var cmds [][]string
	var templateArgs []string

	switch cfg.SourceType {
	case "local":
		if cfg.Source.Local == nil || cfg.Source.Local.ChartPath == "" {
			return nil, fmt.Errorf("missing source.local.chartPath for sourceType=local")
		}
		templateArgs = []string{
			"helm", "template", cfg.ReleaseName, cfg.Source.Local.ChartPath,
			"--namespace", cfg.Namespace,
			"--include-crds",
			"--skip-tests",
			"--dependency-update",
			"--output-dir", outDir,
		}

	case "helm":
		if cfg.Source.Helm == nil {
			return nil, fmt.Errorf("missing source.helm for sourceType=helm")
		}
		cmds = append(cmds, []string{"helm", "repo", "add", "--force-update", cfg.Source.Helm.RepoName, cfg.Source.Helm.RepoURL})
		cmds = append(cmds, []string{"helm", "repo", "update"})

		chartRef := fmt.Sprintf("%s/%s", cfg.Source.Helm.RepoName, cfg.Source.Helm.Name)
		templateArgs = []string{
			"helm", "template", cfg.ReleaseName, chartRef,
			"--namespace", cfg.Namespace,
			"--version", cfg.Source.Helm.Version,
			"--include-crds",
			"--skip-tests",
			"--output-dir", outDir,
		}

	case "oci":
		if cfg.Source.OCI == nil {
			return nil, fmt.Errorf("missing source.oci for sourceType=oci")
		}
		chartRef := fmt.Sprintf(
			"oci://%s/%s/%s",
			strings.Trim(cfg.Source.OCI.Registry, "/"),
			strings.Trim(cfg.Source.OCI.Path, "/"),
			cfg.Source.OCI.Name,
		)
		templateArgs = []string{
			"helm", "template", cfg.ReleaseName, chartRef,
			"--namespace", cfg.Namespace,
			"--version", cfg.Source.OCI.Version,
			"--include-crds",
			"--skip-tests",
			"--output-dir", outDir,
		}
	default:
		return nil, fmt.Errorf("unknown sourceType '%s'", cfg.SourceType)
	}

	if valuesFile != "" {
		templateArgs = append(templateArgs, "--values", valuesFile)
	}
	templateArgs = append(templateArgs, cfg.HelmArgs...)
	cmds = append(cmds, templateArgs)

	return cmds, nil
}
