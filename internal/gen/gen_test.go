package gen

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// mapType is exercised over synthetic tftypes covering the hard cases the simple
// fake-provider schemas don't have: collections, nested objects, etc.
func TestMapTypeScalars(t *testing.T) {
	cases := []struct {
		t    tftypes.Type
		want Kind
	}{
		{tftypes.String, KindString},
		{tftypes.Number, KindNumber},
		{tftypes.Bool, KindBool},
	}
	for _, c := range cases {
		if got := mapType(c.t).Kind; got != c.want {
			t.Errorf("mapType(%v).Kind = %v, want %v", c.t, got, c.want)
		}
	}
}

func TestMapTypeCollections(t *testing.T) {
	list := mapType(tftypes.List{ElementType: tftypes.String})
	if list.Kind != KindList || list.Elem == nil || list.Elem.Kind != KindString {
		t.Errorf("list(string) mapped to %+v", list)
	}
	set := mapType(tftypes.Set{ElementType: tftypes.Bool})
	if set.Kind != KindSet || set.Elem.Kind != KindBool {
		t.Errorf("set(bool) mapped to %+v", set)
	}
	m := mapType(tftypes.Map{ElementType: tftypes.Number})
	if m.Kind != KindMap || m.Elem.Kind != KindNumber {
		t.Errorf("map(number) mapped to %+v", m)
	}
}

func TestMapTypeNestedObject(t *testing.T) {
	obj := mapType(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"ip":    tftypes.String,
		"ports": tftypes.List{ElementType: tftypes.Number},
	}})
	if obj.Kind != KindObject {
		t.Fatalf("object kind = %v", obj.Kind)
	}
	if obj.Attrs["ip"].Kind != KindString {
		t.Errorf("object.ip = %v, want string", obj.Attrs["ip"].Kind)
	}
	if obj.Attrs["ports"].Kind != KindList || obj.Attrs["ports"].Elem.Kind != KindNumber {
		t.Errorf("object.ports = %+v, want list(number)", obj.Attrs["ports"])
	}
}

func TestAttrRoles(t *testing.T) {
	required := Attr{Name: "from", Required: true}
	optional := Attr{Name: "label", Optional: true}
	computed := Attr{Name: "id", Computed: true}
	optComputed := Attr{Name: "tags", Optional: true, Computed: true}

	if !required.IsInput() || !optional.IsInput() {
		t.Error("required/optional must be inputs")
	}
	if computed.IsInput() {
		t.Error("computed-only must NOT be an input (it is an output)")
	}
	if !optComputed.IsInput() {
		t.Error("optional+computed is still an input")
	}
}

// Emit output must be deterministic and structurally correct for a resource with
// required, optional, and computed-only attributes.
func TestEmitStructure(t *testing.T) {
	r := Resource{
		Type: "thing",
		Attrs: []Attr{
			{Name: "id", Type: NixType{Kind: KindString}, Computed: true},
			{Name: "label", Type: NixType{Kind: KindString}, Optional: true},
			{Name: "name_in", Type: NixType{Kind: KindString}, Required: true},
		},
	}
	sortAttrs(r.Attrs)
	out := Emit("p", r)

	// required attr -> a throw naming it
	if !strings.Contains(out, `required argument 'name_in' is missing`) {
		t.Error("emit missing required-throw for name_in")
	}
	// optional attr -> conditional merge
	if !strings.Contains(out, "if label == null then {} else { label = label; }") {
		t.Error("emit missing optional conditional for label")
	}
	// computed-only attr -> NOT an input arg
	if strings.Contains(out, ", id ?") || strings.Contains(out, ", id,") {
		t.Error("computed-only 'id' must not be a constructor input")
	}
	// override seam present
	if !strings.Contains(out, "// overrides") {
		t.Error("emit missing override seam")
	}
	// determinism
	if Emit("p", r) != out {
		t.Error("emit not deterministic")
	}
}
