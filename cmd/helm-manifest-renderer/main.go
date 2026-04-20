package main

import (
	"flag"
	"fmt"
	"os"

	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/render"
)

func main() {
	configFile := flag.String("config", render.DefaultConfigFile, "Path to the chart source config file")
	valuesFile := flag.String("values-file", "", "Optional path to the values file")
	outputDir := flag.String("output-dir", render.DefaultOutputDir, "Directory for generated manifests")
	tempDir := flag.String("temp-dir", render.DefaultTempDir, "Temporary render output directory")
	stageLog := flag.Bool("stage-log", false, "Print stage-by-stage progress information")
	flag.Parse()

	targetDir := "."
	if flag.NArg() > 0 {
		targetDir = flag.Arg(0)
	}

	err := render.RunRenderWithOptions(render.Options{
		TargetDir:  targetDir,
		ConfigFile: *configFile,
		ValuesFile: *valuesFile,
		TempDir:    *tempDir,
		OutputDir:  *outputDir,
		StageLog:   *stageLog,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", render.ErrorPrefix(), err)
		os.Exit(1)
	}
}
