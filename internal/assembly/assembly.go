package assembly

import (
	"bytes"
	"fmt"
	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/config"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type StageLogger func(string)

func splitPathSegments(path string) []string {
	cleaned := filepath.Clean(path)
	if cleaned == "." {
		return nil
	}
	return strings.Split(cleaned, string(os.PathSeparator))
}

func hasPathSegment(path string, segment string) bool {
	for _, part := range splitPathSegments(path) {
		if part == segment {
			return true
		}
	}
	return false
}

func subchartTemplateTarget(chartsDir, path string) (string, string, bool) {
	relToCharts, err := filepath.Rel(chartsDir, path)
	if err != nil {
		return "", "", false
	}
	parts := splitPathSegments(relToCharts)
	if len(parts) < 3 {
		return "", "", false
	}
	if parts[1] != "templates" {
		return "", "", false
	}
	if parts[0] == "crds" && parts[1] == "templates" {
		return "", "", false
	}
	return parts[0], filepath.Join(parts[2:]...), true
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	err = os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}
		return copyFile(path, dstPath)
	})
}

func resolveManifestRelativePath(manifestsDir, relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("path must be relative: %s", relativePath)
	}
	cleaned := filepath.Clean(relativePath)
	if cleaned == "." {
		return manifestsDir, nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path must stay within manifests dir: %s", relativePath)
	}
	return filepath.Join(manifestsDir, cleaned), nil
}

func ApplyMovePaths(manifestsDir string, rules []config.MovePathRule, log StageLogger) error {
	for _, rule := range rules {
		if log != nil {
			log(fmt.Sprintf("postRender.movePaths: %s -> %s", rule.From, rule.To))
		}
		pattern, err := resolveManifestRelativePath(manifestsDir, filepath.FromSlash(rule.From))
		if err != nil {
			return err
		}
		targetDir, err := resolveManifestRelativePath(manifestsDir, filepath.FromSlash(rule.To))
		if err != nil {
			return err
		}

		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				return err
			}
			if info.IsDir() {
				continue
			}

			destination := filepath.Join(targetDir, filepath.Base(match))
			if filepath.Clean(match) == filepath.Clean(destination) {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				return err
			}
			if err := os.Rename(match, destination); err != nil {
				return err
			}
		}
	}
	return nil
}

func AssembleManifests(renderRoot, manifestsDir string, config config.ChartSourceConfig, log StageLogger) error {
	if log != nil {
		log(fmt.Sprintf("Assemble manifests: %s -> %s", renderRoot, manifestsDir))
	}
	os.RemoveAll(manifestsDir)
	os.MkdirAll(manifestsDir, 0755)

	templatesDir := filepath.Join(renderRoot, "templates")
	if info, err := os.Stat(templatesDir); err == nil && info.IsDir() {
		filepath.WalkDir(templatesDir, func(path string, d fs.DirEntry, err error) error {
			if !d.IsDir() {
				rel, _ := filepath.Rel(templatesDir, path)
				copyFile(path, filepath.Join(manifestsDir, rel))
			}
			return nil
		})
	}

	chartsDir := filepath.Join(renderRoot, "charts")
	if info, err := os.Stat(chartsDir); err == nil && info.IsDir() {
		filepath.WalkDir(chartsDir, func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}
			subchartDir, relToTemplates, ok := subchartTemplateTarget(chartsDir, path)
			if ok {
				dst := filepath.Join(manifestsDir, subchartDir, relToTemplates)
				copyFile(path, dst)
			}
			return nil
		})

		crdsChartDir := filepath.Join(chartsDir, "crds")
		if cInfo, err := os.Stat(crdsChartDir); err == nil && cInfo.IsDir() {
			entries, readErr := os.ReadDir(crdsChartDir)
			if readErr != nil {
				return readErr
			}
			for _, entry := range entries {
				src := filepath.Join(crdsChartDir, entry.Name())
				dst := filepath.Join(manifestsDir, entry.Name())
				if entry.IsDir() {
					if err := copyDir(src, dst); err != nil {
						return err
					}
				} else {
					if err := copyFile(src, dst); err != nil {
						return err
					}
				}
			}
		}

		crdsSrcDir := filepath.Join(chartsDir, "crds", "templates")
		if cInfo, err := os.Stat(crdsSrcDir); err == nil && cInfo.IsDir() {
			filepath.WalkDir(crdsSrcDir, func(path string, d fs.DirEntry, err error) error {
				if !d.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
					rel, _ := filepath.Rel(crdsSrcDir, path)
					copyFile(path, filepath.Join(manifestsDir, "crds", rel))
				}
				return nil
			})
		}

		// Delete CRDs from standard templates that shouldn't be there
		filepath.WalkDir(manifestsDir, func(path string, d fs.DirEntry, err error) error {
			if !d.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
				// skip if in crds directory
				if hasPathSegment(path, "crds") {
					return nil
				}
				content, err := os.ReadFile(path)
				if err == nil && bytes.Contains(content, []byte("apiVersion: apiextensions.k8s.io/")) {
					os.Remove(path)
				}
			}
			return nil
		})
	}

	// Copy top-level files
	entries, err := os.ReadDir(renderRoot)
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if name != "templates" && name != "charts" {
				src := filepath.Join(renderRoot, name)
				dst := filepath.Join(manifestsDir, name)
				if entry.IsDir() {
					copyDir(src, dst)
				} else {
					copyFile(src, dst)
				}
			}
		}
	}

	err = SplitYamlDocuments(manifestsDir, config.PostRender.SplitYamlDocumentsInPaths)
	if err != nil {
		return err
	}
	if log != nil && len(config.PostRender.SplitYamlDocumentsInPaths) > 0 {
		log(fmt.Sprintf("postRender.splitYamlDocumentsInPaths: %d path(s)", len(config.PostRender.SplitYamlDocumentsInPaths)))
	}

	if err := ApplyMovePaths(manifestsDir, config.PostRender.MovePaths, log); err != nil {
		return err
	}

	// Apply ExcludePaths after move rules so the final manifest layout can be targeted directly.
	for _, pattern := range config.PostRender.ExcludePaths {
		if log != nil {
			log(fmt.Sprintf("postRender.excludePaths: %s", pattern))
		}
		fullPath := pattern
		if !strings.HasPrefix(pattern, manifestsDir+"/") {
			fullPath = filepath.Join(manifestsDir, pattern)
		}
		os.RemoveAll(fullPath)
	}

	// Generate Kustomization
	if log != nil {
		log("Generate kustomization.yaml")
	}
	err = GenerateKustomization(manifestsDir)
	return err
}

func GenerateKustomization(manifestsDir string) error {
	kpath := filepath.Join(manifestsDir, "kustomization.yaml")

	var subFolderFiles []string
	var topLevelFiles []string

	filepath.WalkDir(manifestsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		if path == kpath {
			return nil
		}

		rel, err := filepath.Rel(manifestsDir, path)
		if err != nil {
			return nil
		}

		if strings.Contains(rel, string(os.PathSeparator)) {
			subFolderFiles = append(subFolderFiles, rel)
		} else {
			topLevelFiles = append(topLevelFiles, rel)
		}
		return nil
	})

	sort.Strings(subFolderFiles)
	sort.Strings(topLevelFiles)

	f, err := os.Create(kpath)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString("resources:\n")
	for _, rel := range subFolderFiles {
		f.WriteString("  - " + rel + "\n")
	}
	for _, rel := range topLevelFiles {
		f.WriteString("  - " + rel + "\n")
	}

	return nil
}

func TidyFiles(manifestsDir string) error {
	return filepath.WalkDir(manifestsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var newContent string
		if hasPathSegment(path, "crds") {
			newContent = cleanupCRDYAML(string(content))
		} else {
			newContent = cleanupYAMLText(string(content))
		}

		return os.WriteFile(path, []byte(newContent), 0644)
	})
}

func cleanupCRDYAML(content string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^[ \t]*controller-gen\.kubebuilder\.io/version:[ \t]*.*\n`),
		regexp.MustCompile(`(?m)^[ \t]*app\.kubernetes\.io/instance:[ \t]*.*\n`),
		regexp.MustCompile(`(?m)^[ \t]*app\.kubernetes\.io/managed-by:[ \t]*.*\n`),
		regexp.MustCompile(`(?m)^[ \t]*app\.kubernetes\.io/version:[ \t]*.*\n`),
		regexp.MustCompile(`(?m)^[ \t]*helm\.sh/chart:[ \t]*.*\n`),
	}

	for _, pattern := range patterns {
		content = pattern.ReplaceAllString(content, "")
	}

	content = regexp.MustCompile(`(?s)\A---\n(?:#.*\n)+---\n`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`(?m)^([ \t]*)annotations:[ \t]*null\n`).ReplaceAllString(content, "")
	content = removeEmptyAnnotationsBlocks(content)
	content = regexp.MustCompile(`(?m)^[ \t]*\n`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`\n{2,}`).ReplaceAllString(content, "\n")
	content = regexp.MustCompile(`\\n\\n+`).ReplaceAllString(content, `\n`)

	return strings.TrimRight(content, "\n") + "\n"
}

func removeEmptyAnnotationsBlocks(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed != "annotations:" {
			result = append(result, line)
			continue
		}

		currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		nextIndex := i + 1
		if nextIndex >= len(lines) {
			continue
		}

		nextLine := lines[nextIndex]
		if strings.TrimSpace(nextLine) == "" {
			continue
		}

		nextIndent := len(nextLine) - len(strings.TrimLeft(nextLine, " \t"))
		if nextIndent <= currentIndent {
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func cleanupYAMLText(content string) string {
	content = regexp.MustCompile(`(?m)^[ \t]*annotations:[ \t]*null\n`).ReplaceAllString(content, "")

	lines := strings.Split(content, "\n")
	cleanedLines := make([]string, 0, len(lines))
	inExprBlock := false
	exprContentIndent := -1

	for _, line := range lines {
		stripped := strings.TrimLeft(line, " ")
		indent := len(line) - len(stripped)

		if inExprBlock {
			if stripped == "" {
				continue
			}
			if exprContentIndent == -1 {
				exprContentIndent = indent
			}
			if indent < exprContentIndent {
				inExprBlock = false
				exprContentIndent = -1
			} else {
				cleanedLines = append(cleanedLines, line)
				continue
			}
		}

		if isExprBlockStart(line) {
			inExprBlock = true
			exprContentIndent = -1
		}

		cleanedLines = append(cleanedLines, line)
	}

	content = strings.Join(cleanedLines, "\n")
	return strings.TrimRight(content, "\n") + "\n"
}

func isExprBlockStart(line string) bool {
	return regexp.MustCompile(`^[ ]*(?:-\s+)?expr:[ ]*[|>][-+]?[ ]*$`).MatchString(line)
}
