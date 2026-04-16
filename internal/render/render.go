package render

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/assembly"
	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/config"
	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/helm"
	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/yamlcleaner"
)

const (
	DefaultConfigFile = "chart-source.yaml"
	DefaultValuesFile = "values.yaml"
	DefaultTempDir    = "tmp"
	DefaultOutputDir  = "generated-manifests"
	colorReset        = "\033[0m"
	colorRed          = "\033[31m"
	colorYellow       = "\033[33m"
	colorCyan         = "\033[36m"
)

type Options struct {
	TargetDir  string
	ConfigFile string
	ValuesFile string
	TempDir    string
	OutputDir  string
}

func info(message string) {
	fmt.Printf("%s[INFO]%s  %s\n", colorCyan, colorReset, message)
}

func warn(message string) {
	fmt.Fprintf(os.Stderr, "%s[WARN]%s  %s\n", colorYellow, colorReset, message)
}

func ErrorPrefix() string {
	return colorRed + "[ERROR]" + colorReset
}

func RunRender(targetDir string) error {
	return RunRenderWithOptions(Options{TargetDir: targetDir})
}

func RunRenderWithOptions(opts Options) error {
	cwd, _ := os.Getwd()
	targetDir := opts.TargetDir
	if targetDir == "" {
		targetDir = "."
	}
	if targetDir != "." {
		err := os.Chdir(targetDir)
		if err != nil {
			return err
		}
		defer os.Chdir(cwd)
	}

	configFile := opts.ConfigFile
	if configFile == "" {
		configFile = DefaultConfigFile
	}
	tempDir := opts.TempDir
	if tempDir == "" {
		tempDir = DefaultTempDir
	}
	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = DefaultOutputDir
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("%s not found", configFile)
	}

	cfg, err := config.ParseChartConfig(content)
	if err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	valuesFile, err := resolveValuesFile(opts.ValuesFile)
	if err != nil {
		return err
	}
	if valuesFile == "" {
		info("No values file found. Rendering chart with the default values from the Helm chart.")
	}

	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	cmds, err := helm.GenerateHelmCommands(cfg, tempDir, valuesFile)
	if err != nil {
		return fmt.Errorf("failed to generate helm command: %v", err)
	}

	for _, c := range cmds {
		info(fmt.Sprintf("Running: %v", c))
		cmd := exec.Command(c[0], c[1:]...)
		if len(c) >= 3 && c[0] == "helm" && c[1] == "repo" && (c[2] == "add" || c[2] == "update") {
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
		} else if len(c) >= 2 && c[0] == "helm" && c[1] == "template" {
			cmd.Stdout = io.Discard
			cmd.Stderr = os.Stderr
		} else {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		err := cmd.Run()
		if err != nil && len(c) >= 3 && c[0] == "helm" && c[1] == "repo" && (c[2] == "add" || c[2] == "update") {
			warn(fmt.Sprintf("helm repo command failed (ignoring): %v", err))
			continue
		} else if err != nil {
			return fmt.Errorf("helm command failed: %v", err)
		}
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("no rendered output found in %s", tempDir)
	}

	var renderRoot string
	for _, e := range entries {
		if e.IsDir() {
			renderRoot = filepath.Join(tempDir, e.Name())
			break
		}
	}

	if renderRoot == "" {
		return fmt.Errorf("no rendered output found in %s", tempDir)
	}

	err = filepath.Walk(renderRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || (filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml") {
			return nil
		}
		if shouldSkipStructuredCleanup(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		cleaned, err := yamlcleaner.CleanYaml(content, yamlcleaner.Options{
			DeletePaths:       cfg.PostRender.DeleteYamlPaths,
			NormalizeMetadata: *cfg.PostRender.NormalizeMetadata,
			RemoveObsoleteQuotes: *cfg.PostRender.RemoveObsoleteQuotes,
		})
		if err != nil {
			return err
		}
		return os.WriteFile(path, cleaned, 0644)
	})
	if err != nil {
		return fmt.Errorf("failed to clean yamls: %v", err)
	}

	err = assembly.AssembleManifests(renderRoot, outputDir, cfg)
	if err != nil {
		return fmt.Errorf("failed to assemble manifests: %v", err)
	}

	err = assembly.TidyFiles(outputDir)
	if err != nil {
		return fmt.Errorf("failed to tidy files: %v", err)
	}

	info(fmt.Sprintf("Done. Generated manifests are stored in folder: %s", outputDir))
	return nil
}

func shouldSkipStructuredCleanup(path string) bool {
	crdSegment := string(os.PathSeparator) + "charts" + string(os.PathSeparator) + "crds" + string(os.PathSeparator)
	return strings.Contains(path, crdSegment)
}

func resolveValuesFile(configured string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(configured); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("%s not found", configured)
			}
			return "", err
		}
		return configured, nil
	}

	for _, candidate := range []string{DefaultValuesFile, "values.yml"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", nil
}
