package data

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/soulteary/flare/config/model"
	yaml "go.yaml.in/yaml/v3"
)

func readConfigDocument(raw []byte) (*yaml.Node, model.Application, error) {
	var doc yaml.Node
	var app model.Application
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&doc); err != nil {
		return nil, app, err
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, app, fmt.Errorf("configuration must be a mapping")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, app, fmt.Errorf("multiple YAML documents are not supported")
	}
	if err := doc.Decode(&app); err != nil {
		return nil, app, err
	}
	return &doc, app, nil
}

func requireRegularConfig(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("configuration must be a regular file; symbolic links are read-only")
	}
	return nil
}

// Aliases and merge keys can make a search edit mutate another section through
// a shared anchor. Refuse a changed save rather than guessing at that intent.
func rejectSharedConfigNodes(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode || node.Tag == "!!merge" {
		return fmt.Errorf("configuration aliases/merge keys cannot be safely edited; original file retained")
	}
	for _, child := range node.Content {
		if err := rejectSharedConfigNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func commitConfigDocument(path string, raw, out []byte, upgrading bool) error {
	return commitConfigDocumentWithBackup(path, raw, out, upgrading, retainSearchBackup)
}

func commitConfigDocumentWithBackup(path string, raw, out []byte, upgrading bool, backup func([]byte, bool) error) error {
	if err := requireRegularConfig(path); err != nil {
		return err
	}
	current, err := readFile(path)
	if err != nil || !bytes.Equal(raw, current) {
		return fmt.Errorf("configuration changed during save; reload and retry")
	}
	if err = backup(raw, upgrading); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	if err = requireRegularConfig(path); err != nil {
		return err
	}
	current, err = readFile(path)
	if err != nil || !bytes.Equal(raw, current) {
		return fmt.Errorf("configuration changed during save; reload and retry")
	}
	if !saveConfigAtomically(path, out) {
		return fmt.Errorf("atomic replacement failed; original backup retained")
	}
	invalidateFileCache("config")
	return nil
}

// If both old and new IDs exist only on one side, an extension-bearing engine
// may have been renamed. Do not silently discard its unknown fields by guessing.
func rejectAmbiguousEngineReplacement(root, patch *yaml.Node) error {
	oldSearch, newSearch := mappingValue(root, "Search"), mappingValue(patch, "Search")
	if oldSearch == nil || newSearch == nil {
		return nil
	}
	oldRows, newRows := mappingValue(oldSearch, "Engines"), mappingValue(newSearch, "Engines")
	if oldRows == nil || newRows == nil {
		return nil
	}
	oldIDs, newIDs := map[string]bool{}, map[string]bool{}
	for _, row := range oldRows.Content {
		if id := mappingValue(row, "ID"); id != nil {
			oldIDs[id.Value] = true
		}
	}
	for _, row := range newRows.Content {
		if id := mappingValue(row, "ID"); id != nil {
			newIDs[id.Value] = true
		}
	}
	added := false
	for id := range newIDs {
		if !oldIDs[id] {
			added = true
		}
	}
	if !added {
		return nil
	}
	known := map[string]bool{"ID": true, "Name": true, "URLTemplate": true, "Keyword": true, "Enabled": true}
	for _, row := range oldRows.Content {
		id := mappingValue(row, "ID")
		if id == nil || newIDs[id.Value] {
			continue
		}
		for i := 0; i < len(row.Content); i += 2 {
			if !known[row.Content[i].Value] {
				return fmt.Errorf("engine ID change could discard extension fields; retain the original ID or edit this engine in YAML")
			}
		}
	}
	return nil
}
