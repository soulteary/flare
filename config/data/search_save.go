package data

import (
	"fmt"
	"reflect"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/search"
	yaml "go.yaml.in/yaml/v3"
)

// Only explicit search saves persist the in-memory upgrade. Preserve comments,
// unknown fields and engine extensions, and retain an exact private backup first.
func saveSearchConfigPreservingData(options model.Application) error {
	path := getConfigPath("config")
	raw, err := readFile(path)
	if err != nil {
		return err
	}
	doc, original, err := readConfigDocument(raw)
	if err != nil {
		return err
	}
	if options.ConfigRevision != "" && options.ConfigRevision != backupDigest(raw) {
		return fmt.Errorf("configuration changed since this form was loaded; reload before saving")
	}
	if original.ShowSearchComponent == options.ShowSearchComponent && original.DisabledSearchAutoFocus == options.DisabledSearchAutoFocus && reflect.DeepEqual(search.Normalize(original.Search), search.Normalize(options.Search)) {
		return nil
	}
	if err = rejectSharedConfigNodes(doc); err != nil {
		return err
	}
	writer, err := versionForSave(original)
	if err != nil {
		return err
	}
	var patch yaml.Node
	if err = patch.Encode(struct {
		Version string               `yaml:"AppVersion"`
		Show    bool                 `yaml:"ShowSearchComponent"`
		Focus   bool                 `yaml:"DisabledSearchAutoFocus"`
		Search  *model.SearchOptions `yaml:"Search"`
	}{writer, options.ShowSearchComponent, options.DisabledSearchAutoFocus, options.Search}); err != nil {
		return err
	}
	if err = rejectAmbiguousEngineReplacement(doc.Content[0], &patch); err != nil {
		return err
	}
	mergeSearchNodes(doc.Content[0], &patch)
	out, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	// Ensure the new document is readable before touching the current file.
	var check model.Application
	if err = yaml.Unmarshal(out, &check); err != nil {
		return err
	}
	_, upgrading := search.Upgrade(original.Search)
	upgrading = upgrading || original.Search == nil || ConfigurationVersionState(original.AppVersion) == "upgrade" || original.AppVersion == ""
	return commitConfigDocument(path, raw, out, upgrading)
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func mergeSearchNodes(dst, src *yaml.Node) {
	if dst.Kind == yaml.MappingNode && src.Kind == yaml.MappingNode {
		for i := 0; i < len(src.Content); i += 2 {
			key, value := src.Content[i], src.Content[i+1]
			old := mappingValue(dst, key.Value)
			if old == nil {
				dst.Content = append(dst.Content, key, value)
			} else {
				mergeSearchNodes(old, value)
			}
		}
		return
	}
	if dst.Kind == yaml.SequenceNode && src.Kind == yaml.SequenceNode {
		// Engine order/removal follows the submitted form; unknown fields follow ID.
		for i, next := range src.Content {
			id := mappingValue(next, "ID")
			if id == nil {
				continue
			}
			for _, old := range dst.Content {
				oldID := mappingValue(old, "ID")
				if oldID != nil && oldID.Value == id.Value {
					mergeSearchNodes(old, next)
					src.Content[i] = old
					break
				}
			}
		}
	}
	head, line, foot := dst.HeadComment, dst.LineComment, dst.FootComment
	*dst = *src
	dst.HeadComment, dst.LineComment, dst.FootComment = head, line, foot
}
