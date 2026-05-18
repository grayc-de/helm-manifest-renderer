package yamlcleaner

import (
	"bytes"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type Options struct {
	DeletePaths       []string
	NormalizeMetadata bool
}

// CleanYaml processes one or more YAML documents and applies configured cleanup rules.
func CleanYaml(content []byte, opts Options) ([]byte, error) {
	dec := yaml.NewDecoder(bytes.NewReader(content))
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	defer enc.Close()

	for {
		var node yaml.Node
		if err := dec.Decode(&node); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if len(node.Content) == 0 {
			continue
		}

		doc := node.Content[0]

		for _, path := range opts.DeletePaths {
			parsedPath := parseDeletePath(path)
			deleteFromNode(doc, parsedPath)
		}

		if opts.NormalizeMetadata {
			applyStructuralCleanup(doc, isKind(doc, "Deployment") || isKind(doc, "DaemonSet"))
		}

		if tidyEmptyNodes(doc, 0) {
			continue
		}

		if err := enc.Encode(&node); err != nil {
			return nil, err
		}
	}

	if buf.Len() == 0 {
		return []byte{}, nil
	}
	return buf.Bytes(), nil
}

func isKind(node *yaml.Node, expectedKind string) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == "kind" && node.Content[i+1].Value == expectedKind {
			return true
		}
	}
	return false
}

func applyStructuralCleanup(node *yaml.Node, isDeploymentOrDaemonSet bool) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		var newContent []*yaml.Node
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			val := node.Content[i+1]

			if key == "app.kubernetes.io/version" ||
				key == "app.kubernetes.io/managed-by" ||
				key == "helm.sh/chart" ||
				key == "heritage" {
				continue
			}

			if key == "helm.sh/resource-policy" {
				continue
			}

			if isDeploymentOrDaemonSet && key == "httpHeaders" {
				continue
			}

			applyStructuralCleanup(val, isDeploymentOrDaemonSet)
			newContent = append(newContent, node.Content[i], val)
		}
		node.Content = newContent
	case yaml.SequenceNode:
		for _, child := range node.Content {
			applyStructuralCleanup(child, isDeploymentOrDaemonSet)
		}
	}
}

func tidyEmptyNodes(node *yaml.Node, depth int) bool {
	if node == nil {
		return false
	}
	switch node.Kind {
	case yaml.MappingNode:
		var newContent []*yaml.Node
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			val := node.Content[i+1]

			isEmpty := tidyEmptyNodes(val, depth+1)

			if isEmpty && (key == "metadata" || key == "creationTimestamp" || key == "annotations" || key == "labels" || key == "conditions" || key == "storedVersions") {
				continue
			}
			if isEmpty && key == "status" && depth == 0 {
				continue
			}
			newContent = append(newContent, node.Content[i], val)
		}
		node.Content = newContent
		return len(node.Content) == 0
	case yaml.SequenceNode:
		var newContent []*yaml.Node
		for _, child := range node.Content {
			isEmpty := tidyEmptyNodes(child, depth+1)
			if !isEmpty || child.Kind != yaml.ScalarNode {
				newContent = append(newContent, child)
			}
		}
		node.Content = newContent
		return len(node.Content) == 0
	case yaml.ScalarNode:
		return node.Tag == "!!null" || node.Value == "null"
	}
	return false
}

func deleteFromNode(node *yaml.Node, path []string) {
	if len(path) == 0 || node.Kind != yaml.MappingNode {
		return
	}
	keyToFind := path[0]
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == keyToFind {
			if len(path) == 1 {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				return
			}
			deleteFromNode(node.Content[i+1], path[1:])
			return
		}
	}
}

func parseDeletePath(path string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '"' {
			inQuotes = !inQuotes
		} else if c == '.' && !inQuotes {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}
