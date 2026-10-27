package yamlutil

import (
	"errors"
	"fmt"
	"github.com/soulteary/flare/config/model"
	"go.yaml.in/yaml/v3"
)

// ErrBookmarkAliasesReadOnly keeps established YAML anchor/merge files readable
// without letting an editor rewrite their shared-node semantics accidentally.
var ErrBookmarkAliasesReadOnly = errors.New("YAML anchors and aliases are read-only; edit the YAML file and restart")

func RejectBookmarkAliases(raw []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return err
	}
	return rejectBookmarkAliasesNode(&root)
}
func rejectBookmarkAliasesNode(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return ErrBookmarkAliasesReadOnly
	}
	for _, child := range n.Content {
		if err := rejectBookmarkAliasesNode(child); err != nil {
			return err
		}
	}
	return nil
}

// yaml.v3 already bounds alias expansion during Decode. This separate resolver
// checks effective permission-node types with bounded work and cycle detection;
// it follows the same explicit-key and merge-sequence precedence as the decoder.
type permissionResolver struct {
	remaining int
	stack     map[*yaml.Node]bool
}

func (r *permissionResolver) step(depth int) error {
	r.remaining--
	if depth > 64 || r.remaining < 0 {
		return fmt.Errorf("YAML permission alias expansion exceeds limits")
	}
	return nil
}
func (r *permissionResolver) deref(n *yaml.Node, depth int) (*yaml.Node, error) {
	seen := map[*yaml.Node]bool{}
	for n != nil && n.Kind == yaml.AliasNode {
		if stepErr := r.step(depth); stepErr != nil {
			return nil, stepErr
		}
		depth++
		if seen[n] {
			return nil, fmt.Errorf("cyclic YAML alias")
		}
		seen[n] = true
		n = n.Alias
	}
	return n, nil
}
func (r *permissionResolver) value(n *yaml.Node, key string, depth int) (*yaml.Node, error) {
	if err := r.step(depth); err != nil {
		return nil, err
	}
	n, err := r.deref(n, depth)
	if err != nil || n == nil {
		return nil, err
	}
	if n.Kind != yaml.MappingNode {
		return nil, nil
	}
	if r.stack[n] {
		return nil, fmt.Errorf("cyclic YAML merge")
	}
	r.stack[n] = true
	defer delete(r.stack, n)
	var merge *yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		if stepErr := r.step(depth); stepErr != nil {
			return nil, stepErr
		}
		k, v := n.Content[i], n.Content[i+1]
		if k.Tag == "!!merge" {
			merge = v
			continue
		}
		if k.Value == key {
			return r.deref(v, depth+1)
		}
	}
	if merge == nil {
		return nil, nil
	}
	merge, err = r.deref(merge, depth+1)
	if err != nil || merge == nil {
		return nil, err
	}
	if merge.Kind == yaml.SequenceNode {
		for _, candidate := range merge.Content {
			v, e := r.value(candidate, key, depth+1)
			if e != nil || v != nil {
				return v, e
			}
		}
		return nil, nil
	}
	return r.value(merge, key, depth+1)
}
func validateEffectivePermissions(doc *yaml.Node, out *model.Bookmarks) error {
	if len(doc.Content) == 0 {
		return nil
	}
	r := &permissionResolver{remaining: 1000000, stack: map[*yaml.Node]bool{}}
	for _, key := range []string{"links", "categories"} {
		seq, err := r.value(doc.Content[0], key, 0)
		if err != nil {
			return err
		}
		if seq == nil {
			continue
		}
		if seq.Kind != yaml.SequenceNode {
			return fmt.Errorf("%s must be a YAML sequence", key)
		}
		for _, item := range seq.Content {
			v, err := r.value(item, "private", 0)
			if err != nil {
				return err
			}
			if v != nil && (v.Kind != yaml.ScalarNode || v.Tag != "!!bool" || (v.Value != "true" && v.Value != "false")) {
				return fmt.Errorf("private must be true or false")
			}
		}
	}
	return nil
}

func validateAliasGraph(root *yaml.Node) error {
	state := map[*yaml.Node]byte{}
	remaining := 1000000
	var visit func(*yaml.Node, int) error
	visit = func(n *yaml.Node, depth int) error {
		if n == nil {
			return fmt.Errorf("invalid YAML alias")
		}
		if state[n] == 1 {
			return fmt.Errorf("cyclic YAML alias")
		}
		if state[n] == 2 {
			return nil
		}
		remaining--
		if depth > 128 || remaining < 0 {
			return fmt.Errorf("YAML alias graph exceeds limits")
		}
		state[n] = 1
		if n.Kind == yaml.AliasNode {
			if err := visit(n.Alias, depth+1); err != nil {
				return err
			}
		}
		for _, child := range n.Content {
			if err := visit(child, depth+1); err != nil {
				return err
			}
		}
		state[n] = 2
		return nil
	}
	return visit(root, 0)
}
