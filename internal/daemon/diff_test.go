package daemon

import (
	"strings"
	"testing"
)

func TestDiffSnapshots(t *testing.T) {
	base := []SnapNode{
		{Ref: "e1", Role: "heading", Name: "Title"},
		{Ref: "e2", Role: "textbox", Name: "Email", Value: ""},
		{Ref: "e3", Role: "button", Name: "Submit"},
	}

	// 1. Unchanged
	diff, changed := DiffSnapshots(base, base)
	if changed != 0 || diff != "unchanged\n" {
		t.Errorf("expected unchanged, got changed=%d diff=%s", changed, diff)
	}

	// 2. Value modified
	modified := []SnapNode{
		{Ref: "e1", Role: "heading", Name: "Title"},
		{Ref: "e2", Role: "textbox", Name: "Email", Value: "alice@example.com"},
		{Ref: "e3", Role: "button", Name: "Submit"},
	}
	diff, changed = DiffSnapshots(base, modified)
	if changed != 1 || !strings.Contains(diff, "~ e2 textbox") {
		t.Errorf("expected 1 modified node, got %d (%s)", changed, diff)
	}

	// 3. Node added & removed
	mutated := []SnapNode{
		{Ref: "e1", Role: "heading", Name: "Title"},
		{Ref: "e4", Role: "status", Name: "Loading"},
	}
	diff, changed = DiffSnapshots(base, mutated)
	if changed != 3 { // e2 removed, e3 removed, e4 added
		t.Errorf("expected 3 changes, got %d (%s)", changed, diff)
	}
}

func TestFilterSubtreeAndFind(t *testing.T) {
	nodes := []SnapNode{
		{Ref: "e1", Depth: 0, Role: "form", Name: "Login"},
		{Ref: "e2", Depth: 1, Role: "textbox", Name: "Username"},
		{Ref: "e3", Depth: 1, Role: "textbox", Name: "Password"},
		{Ref: "e4", Depth: 0, Role: "footer", Name: "Copyright"},
	}

	subtree := FilterSubtree(nodes, "e1")
	if len(subtree) != 3 {
		t.Errorf("expected 3 nodes in e1 subtree, got %d", len(subtree))
	}

	found := FilterFind(nodes, "password")
	if len(found) != 1 || found[0].Ref != "e3" {
		t.Errorf("expected finding e3, got %+v", found)
	}
}
