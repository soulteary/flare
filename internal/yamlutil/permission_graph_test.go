package yamlutil

import (
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestPermissionResolverRejectsCyclesAndExcessiveDepth(t *testing.T) {
	alias := &yaml.Node{Kind: yaml.AliasNode}
	alias.Alias = alias
	resolver := func() *permissionResolver { return &permissionResolver{remaining: 1000, stack: map[*yaml.Node]bool{}} }
	_, err := resolver().deref(alias, 0)
	require.ErrorContains(t, err, "cyclic")
	_, err = resolver().deref(alias, 65)
	require.ErrorContains(t, err, "limits")
	merge := &yaml.Node{Kind: yaml.MappingNode}
	merge.Content = []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!merge", Value: "<<"}, merge}
	_, err = resolver().value(merge, "private", 0)
	require.ErrorContains(t, err, "cyclic")
	exhausted := resolver()
	exhausted.remaining = 1
	_, err = exhausted.value(merge, "private", 0)
	require.ErrorContains(t, err, "limits")
	seq := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "irrelevant"}}}
	merge.Content[1] = seq
	value, err := resolver().value(merge, "private", 0)
	require.NoError(t, err)
	require.Nil(t, value)
	require.ErrorContains(t, validateAliasGraph(alias), "cyclic")
	require.ErrorContains(t, validateAliasGraph(&yaml.Node{Kind: yaml.AliasNode}), "invalid")
	root := &yaml.Node{Kind: yaml.SequenceNode}
	node := root
	for i := 0; i < 130; i++ {
		child := &yaml.Node{Kind: yaml.SequenceNode}
		node.Content = []*yaml.Node{child}
		node = child
	}
	require.ErrorContains(t, validateAliasGraph(root), "limits")
}

func TestEffectivePermissionsValidateMergedNodeTypes(t *testing.T) {
	for _, raw := range []string{
		"links: scalar\n",
		"links:\n- &entry {name: Test, link: https://example.com, private: 'true'}\n",
		"defaults: &defaults {private: [true]}\nlinks:\n- <<: *defaults\n  name: Test\n  link: https://example.com\n",
	} {
		var doc yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(raw), &doc))
		require.Error(t, validateEffectivePermissions(&doc, &model.Bookmarks{}))
	}
	require.NoError(t, validateEffectivePermissions(&yaml.Node{}, &model.Bookmarks{}))
	raw := []byte("first: &first {private: true}\nsecond: &second {private: false}\nlinks:\n- <<: [*first, *second]\n  name: Test\n  link: https://example.com\n")
	var parsed model.Bookmarks
	require.NoError(t, DecodeBookmarks(raw, &parsed, false))
	require.True(t, parsed.Items[0].Private)
	require.Error(t, RejectBookmarkAliases([]byte("[invalid")))
}
