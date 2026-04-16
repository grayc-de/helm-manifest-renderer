package assembly

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func SplitYamlDocuments(renderRoot string, paths []string) error {
	for _, configuredPath := range paths {
		targetPath := filepath.Join(renderRoot, configuredPath)
		info, err := os.Stat(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		if info.IsDir() {
			err = filepath.WalkDir(targetPath, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() || !isYAMLPath(path) {
					return nil
				}
				return splitYamlFile(path)
			})
			if err != nil {
				return err
			}
			continue
		}

		if isYAMLPath(targetPath) {
			if err := splitYamlFile(targetPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func splitYamlFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	documents, err := decodeDocuments(content)
	if err != nil {
		return err
	}
	if len(documents) <= 1 {
		return nil
	}

	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ext := filepath.Ext(path)
	dir := filepath.Dir(path)
	usedNames := map[string]int{}

	for index, document := range documents {
		fileName := buildSplitFileName(base, ext, document, index, usedNames)
		targetPath := filepath.Join(dir, fileName)
		if err := os.WriteFile(targetPath, encodeDocument(document), 0644); err != nil {
			return err
		}
	}

	return os.Remove(path)
}

func decodeDocuments(content []byte) ([]*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var documents []*yaml.Node

	for {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(node.Content) == 0 {
			continue
		}
		documents = append(documents, &node)
	}

	return documents, nil
}

func encodeDocument(document *yaml.Node) []byte {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	_ = encoder.Encode(document)
	_ = encoder.Close()
	return buf.Bytes()
}

func buildSplitFileName(base, ext string, document *yaml.Node, index int, usedNames map[string]int) string {
	kind := sanitizeFileToken(findMappingValue(document.Content[0], "kind"))
	name := sanitizeFileToken(findMetadataName(document.Content[0]))

	var candidate string
	if kind == "customresourcedefinition" && name != "" {
		candidate = fmt.Sprintf("%s%s", name, ext)
	} else if kind != "" && name != "" {
		candidate = fmt.Sprintf("%s--%s--%s%s", base, kind, name, ext)
	} else {
		candidate = fmt.Sprintf("%s--%d%s", base, index+1, ext)
	}

	if usedNames[candidate] == 0 {
		usedNames[candidate] = 1
		return candidate
	}

	count := usedNames[candidate] + 1
	usedNames[candidate] = count
	return fmt.Sprintf("%s--%d%s", strings.TrimSuffix(candidate, ext), count, ext)
}

func findMetadataName(node *yaml.Node) string {
	metadata := findMappingNode(node, "metadata")
	if metadata == nil {
		return ""
	}
	return findMappingValue(metadata, "name")
}

func findMappingNode(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

func findMappingValue(node *yaml.Node, key string) string {
	valueNode := findMappingNode(node, key)
	if valueNode == nil {
		return ""
	}
	return valueNode.Value
}

func sanitizeFileToken(value string) string {
	if value == "" {
		return ""
	}

	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastWasDash := false

	for _, char := range value {
		isAllowed := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
		if isAllowed {
			builder.WriteRune(char)
			lastWasDash = false
			continue
		}
		if !lastWasDash {
			builder.WriteByte('-')
			lastWasDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}

func isYAMLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}
