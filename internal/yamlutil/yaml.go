// Package yamlutil provides shared YAML decoding policies.
package yamlutil

import (
	"bytes"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// UnmarshalStrict rejects unknown fields, duplicate keys and trailing documents.
// Empty input retains the no-op behavior of the former YAML v2 decoder.
func UnmarshalStrict(raw []byte, out any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("multiple YAML documents are not supported")
	}
	return nil
}
