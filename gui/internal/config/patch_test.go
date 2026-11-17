package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMergePatchRFC7396(t *testing.T) {
	// RFC 7396 appendix A, a selection.
	tests := []struct{ target, patch, want string }{
		{`{"a":"b"}`, `{"a":"c"}`, `{"a":"c"}`},
		{`{"a":"b"}`, `{"b":"c"}`, `{"a":"b","b":"c"}`},
		{`{"a":"b"}`, `{"a":null}`, `{}`},
		{`{"a":{"b":"c"}}`, `{"a":{"b":"d","c":null}}`, `{"a":{"b":"d"}}`},
		{`{"a":[{"b":"c"}]}`, `{"a":[1]}`, `{"a":[1]}`},
		{`{"a":"foo"}`, `{"a":{"b":"c"}}`, `{"a":{"b":"c"}}`},
	}

	for _, tt := range tests {
		var target, patch, want any
		for _, p := range []struct {
			s string
			v *any
		}{{tt.target, &target}, {tt.patch, &patch}, {tt.want, &want}} {
			if err := json.Unmarshal([]byte(p.s), p.v); err != nil {
				t.Fatalf("Unmarshal %s: %v", p.s, err)
			}
		}

		if got := mergePatch(target, patch); !reflect.DeepEqual(got, want) {
			t.Errorf("mergePatch(%s, %s) = %v, want %s", tt.target, tt.patch, got, tt.want)
		}
	}
}

func TestMergePatchDocument(t *testing.T) {
	doc := loadFixture(t)

	var patch any
	if err := json.Unmarshal([]byte(`{"field":{"goalWidth":1234},"defaults":{"color":{"reference_force":0.5}}}`), &patch); err != nil {
		t.Fatal(err)
	}

	got, err := MergePatch(doc, patch)
	if err != nil {
		t.Fatalf("MergePatch: %v", err)
	}

	if got.Field.GoalWidth != 1234 {
		t.Errorf("goalWidth = %d, want 1234", got.Field.GoalWidth)
	}

	if got.Field.FieldLength != doc.Field.FieldLength {
		t.Errorf("fieldLength changed: %d -> %d", doc.Field.FieldLength, got.Field.FieldLength)
	}

	if len(got.Cameras) != len(doc.Cameras) {
		t.Errorf("cameras: %d, want %d untouched", len(got.Cameras), len(doc.Cameras))
	}
}

func TestMergePatchRejectsUnknownKey(t *testing.T) {
	var patch any
	if err := json.Unmarshal([]byte(`{"feild":{"goalWidth":1}}`), &patch); err != nil {
		t.Fatal(err)
	}

	if _, err := MergePatch(loadFixture(t), patch); !IsValidation(err) {
		t.Errorf("MergePatch with unknown key: err = %v, want a validation error", err)
	}
}

func TestMergePatchRejectsNonObject(t *testing.T) {
	if _, err := MergePatch(loadFixture(t), []any{}); !IsValidation(err) {
		t.Errorf("MergePatch with array: err = %v, want a validation error", err)
	}
}
