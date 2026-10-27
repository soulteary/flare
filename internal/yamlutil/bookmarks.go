package yamlutil

import (
	"bytes"
	"fmt"
	"github.com/soulteary/flare/config/model"
	"go.yaml.in/yaml/v3"
	"io"
	"log"
)

// DecodeBookmarks validates permission-bearing syntax while allowing extensions.
func DecodeBookmarks(raw []byte, out *model.Bookmarks, apps bool) error {
	var doc yaml.Node
	d := yaml.NewDecoder(bytes.NewReader(raw))
	if err := d.Decode(&doc); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	var extra yaml.Node
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("multiple YAML documents are not supported")
	}
	if err := validateAliasGraph(&doc); err != nil {
		return err
	}
	if err := validateBookmarkNode(&doc); err != nil {
		return err
	}
	if err := doc.Decode(out); err != nil {
		return err
	}
	if err := validateEffectivePermissions(&doc, out); err != nil {
		return err
	}
	if len(doc.Content) > 0 {
		root := doc.Content[0]
		if seq := mappingValue(root, "links"); seq != nil {
			for i, n := range seq.Content {
				if i < len(out.Items) {
					extensions, err := extensionYAML(n, "links")
					if err != nil {
						return err
					}
					out.Items[i].Extensions = extensions
				}
			}
		}
		if seq := mappingValue(root, "categories"); seq != nil {
			for i, n := range seq.Content {
				if i < len(out.Categories) {
					extensions, err := extensionYAML(n, "categories")
					if err != nil {
						return err
					}
					out.Categories[i].Extensions = extensions
				}
			}
		}
	}
	ids := map[string]bool{}
	for _, c := range out.Categories {
		if c.ID == "" || ids[c.ID] {
			return fmt.Errorf("missing or duplicate category ID")
		}
		ids[c.ID] = true
	}
	for _, b := range out.Items {
		if b.Category != "" && (apps || !ids[b.Category]) {
			return fmt.Errorf("invalid category reference")
		}
	}
	return nil
}
func validateBookmarkNode(n *yaml.Node) error {

	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if seen[k.Value] {
				return fmt.Errorf("duplicate YAML key %q", k.Value)
			}
			seen[k.Value] = true
			if k.Value == "privte" || k.Value == "pirvate" || k.Value == "Private" {
				log.Printf("navigation warning: unknown field %q does not enable private protection", k.Value)
			}
		}
	}
	for _, c := range n.Content {
		if err := validateBookmarkNode(c); err != nil {
			return err
		}
	}
	return nil
}

// MergeBookmarks updates known fields on YAML nodes, retaining extension fields.
func MergeBookmarks(raw []byte, value model.Bookmarks) ([]byte, error) {
	if err := RejectBookmarkAliases(raw); err != nil {
		return nil, err
	}
	var old, next yaml.Node
	if err := yaml.Unmarshal(raw, &old); err != nil {
		return nil, err
	}
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(encoded, &next); err != nil {
		return nil, err
	}
	if len(old.Content) == 0 {
		old.Kind = yaml.DocumentNode
		old.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	if old.Content[0].Kind != yaml.MappingNode {
		old.Content[0] = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	root := old.Content[0]
	nr := next.Content[0]
	for _, group := range []struct {
		key    string
		extras []string
	}{{"links", bookmarkExtensions(value.Items)}, {"categories", categoryExtensions(value.Categories)}} {
		seq := mappingValue(nr, group.key)
		if seq != nil {
			for i, item := range seq.Content {
				if i < len(group.extras) && group.extras[i] != "" {
					var n yaml.Node
					if err := yaml.Unmarshal([]byte(group.extras[i]), &n); err != nil {
						return nil, err
					}
					if len(n.Content) > 0 {
						retainExtensions(item, n.Content[0], group.key)
					}
				}
			}
		}
	}
	for _, key := range []string{"categories", "links"} {
		prev := mappingValue(root, key)
		replacement := mappingValue(nr, key)
		if replacement == nil {
			replacement = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		}
		if prev != nil {
			field := "link"
			if key == "categories" {
				field = "id"
			}
			indexed := map[string][]*yaml.Node{}
			for _, prior := range prev.Content {
				if id := mappingValue(prior, field); id != nil {
					indexed[id.Value] = append(indexed[id.Value], prior)
				}
			}
			for _, item := range replacement.Content {
				if id := mappingValue(item, field); id != nil {
					queue := indexed[id.Value]
					if len(queue) > 0 {
						retainExtensions(item, queue[0], key)
						indexed[id.Value] = queue[1:]
					}
				}
			}
		}
		setMapping(root, key, replacement)
	}
	return yaml.Marshal(&old)
}
func mappingValue(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
func setMapping(n *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			n.Content[i+1] = v
			return
		}
	}
	n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
}
func retainExtensions(dst, src *yaml.Node, kind string) {
	known := map[string]bool{"id": true, "title": true, "private": true}
	if kind == "links" {
		known = map[string]bool{"name": true, "link": true, "icon": true, "desc": true, "category": true, "private": true}
	}
	for i := 0; i+1 < len(src.Content); i += 2 {
		if !known[src.Content[i].Value] && mappingValue(dst, src.Content[i].Value) == nil {
			dst.Content = append(dst.Content, src.Content[i], src.Content[i+1])
		}
	}
}

func extensionYAML(n *yaml.Node, kind string) (string, error) {
	extra := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	retainExtensions(extra, n, kind)
	if len(extra.Content) == 0 {
		return "", nil
	}
	raw, err := yaml.Marshal(extra)
	return string(raw), err
}
func bookmarkExtensions(items []model.Bookmark) []string {
	out := make([]string, len(items))
	for i, b := range items {
		out[i] = b.Extensions
	}
	return out
}
func categoryExtensions(items []model.Category) []string {
	out := make([]string, len(items))
	for i, b := range items {
		out[i] = b.Extensions
	}
	return out
}
