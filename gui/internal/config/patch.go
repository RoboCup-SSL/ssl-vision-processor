package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// MergePatch applies a JSON Merge Patch (RFC 7396) to doc's JSON form: keys
// in patch replace doc's, nested objects merge, null removes a key. Arrays
// are replaced whole, so changing one camera means sending the full cameras
// list. A patch naming a key Document doesn't have is rejected. The result is
// not validated; Store.Update does that.
func MergePatch(doc Document, patch any) (Document, error) {
	if _, ok := patch.(map[string]any); !ok {
		return Document{}, &ValidationError{errors.New("patch: want a JSON object")}
	}

	data, err := json.Marshal(doc)
	if err != nil {
		return Document{}, err
	}

	var target any
	if err := json.Unmarshal(data, &target); err != nil {
		return Document{}, err
	}

	merged, err := json.Marshal(mergePatch(target, patch))
	if err != nil {
		return Document{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(merged))
	decoder.DisallowUnknownFields()

	var out Document
	if err := decoder.Decode(&out); err != nil {
		return Document{}, &ValidationError{fmt.Errorf("patch: %w", err)}
	}

	return out, nil
}

// mergePatch is RFC 7396's MergePatch(Target, Patch).
func mergePatch(target, patch any) any {
	patchMap, ok := patch.(map[string]any)
	if !ok {
		return patch
	}

	targetMap, _ := target.(map[string]any)
	out := make(map[string]any, len(targetMap)+len(patchMap))

	for k, v := range targetMap {
		out[k] = v
	}

	for k, v := range patchMap {
		if v == nil {
			delete(out, k)
		} else {
			out[k] = mergePatch(out[k], v)
		}
	}

	return out
}
