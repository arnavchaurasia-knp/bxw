package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	jsonv2 "github.com/chromedp/cdproto/cdp/jsonv2"
)

func makeAXValue(val any) *accessibility.Value {
	b, _ := json.Marshal(val)
	return &accessibility.Value{
		Value: jsonv2.Value(b),
	}
}

func TestBuildSnapshotFiltersAndRefs(t *testing.T) {
	nodes := []*accessibility.Node{
		{
			NodeID:   "1",
			ParentID: "",
			Role:     makeAXValue("RootWebArea"),
			ChildIDs: []accessibility.NodeID{"2", "3", "4", "5"},
		},
		{
			NodeID:           "2",
			ParentID:         "1",
			Role:             makeAXValue("generic"),
			ChildIDs:         []accessibility.NodeID{"6"},
			BackendDOMNodeID: 102,
		},
		{
			NodeID:           "6",
			ParentID:         "2",
			Role:             makeAXValue("button"),
			Name:             makeAXValue("Submit Form"),
			BackendDOMNodeID: 106,
			Properties: []*accessibility.Property{
				{Name: "disabled", Value: makeAXValue("true")},
			},
		},
		{
			NodeID:           "3",
			ParentID:         "1",
			Role:             makeAXValue("textbox"),
			Name:             makeAXValue("Username"),
			Value:            makeAXValue("alice"),
			BackendDOMNodeID: 103,
			ChildIDs:         []accessibility.NodeID{"7"}, // Child of leaf node must be ignored
		},
		{
			NodeID:   "7",
			ParentID: "3",
			Role:     makeAXValue("StaticText"),
			Name:     makeAXValue("alice"),
		},
		{
			NodeID:   "4",
			ParentID: "1",
			Ignored:  true,
			Role:     makeAXValue("button"),
			Name:     makeAXValue("Ignored Button"),
		},
		{
			NodeID:   "5",
			ParentID: "1",
			Role:     makeAXValue("InlineTextBox"),
			Name:     makeAXValue("Inline Text"),
		},
	}

	snapNodes, refs, descs := buildSnapshot(nodes)

	if len(snapNodes) != 2 {
		t.Fatalf("expected 2 snap nodes, got %d", len(snapNodes))
	}

	// F1: buildSnapshot records a stable descriptor (role+name) per ref.
	if descs["e1"] != (RefDescriptor{Role: "button", Name: "Submit Form"}) {
		t.Errorf("expected descriptor e1 = {button, Submit Form}, got %+v", descs["e1"])
	}
	if descs["e2"] != (RefDescriptor{Role: "textbox", Name: "Username"}) {
		t.Errorf("expected descriptor e2 = {textbox, Username}, got %+v", descs["e2"])
	}

	// First node: button (from under generic)
	if snapNodes[0].Ref != "e1" {
		t.Errorf("expected e1, got %s", snapNodes[0].Ref)
	}
	if snapNodes[0].Role != "button" || snapNodes[0].Name != "Submit Form" {
		t.Errorf("unexpected node 0: %+v", snapNodes[0])
	}
	if len(snapNodes[0].Props) != 1 || snapNodes[0].Props[0] != "disabled" {
		t.Errorf("expected [disabled] prop, got %v", snapNodes[0].Props)
	}
	if refs["e1"] != 106 {
		t.Errorf("expected ref e1 -> 106, got %d", refs["e1"])
	}

	// Second node: textbox (leaf node, child 7 skipped)
	if snapNodes[1].Ref != "e2" {
		t.Errorf("expected e2, got %s", snapNodes[1].Ref)
	}
	if snapNodes[1].Role != "textbox" || snapNodes[1].Name != "Username" || snapNodes[1].Value != "alice" {
		t.Errorf("unexpected node 1: %+v", snapNodes[1])
	}
	if refs["e2"] != 103 {
		t.Errorf("expected ref e2 -> 103, got %d", refs["e2"])
	}

	// Test Text output
	snap := Snapshot{
		Title: "Test Page",
		URL:   "https://example.com/login",
		Nodes: snapNodes,
	}
	rendered := snap.Text()
	if !strings.Contains(rendered, `e1 button "Submit Form" [disabled]`) {
		t.Errorf("expected e1 formatted line in:\n%s", rendered)
	}
	if !strings.Contains(rendered, `e2 textbox "Username" value="alice"`) {
		t.Errorf("expected e2 formatted line in:\n%s", rendered)
	}
}

// TestF1StableRefMatching proves the core F1 guarantee: after a benign re-render
// reorders/renumbers elements, a ref re-resolves to the same logical element by its
// stable role+name descriptor — even though its eN index changed and the underlying
// DOM node was recreated. This is the pure, browser-free heart of selector()'s fallback.
func TestF1StableRefMatching(t *testing.T) {
	// Descriptor captured for the "Learn more" link when it was e2 in the original snapshot.
	want := RefDescriptor{Role: "link", Name: "Learn more"}

	// After a re-render: the link moved to the front (now e1), indices shifted, a new field appeared.
	after := []SnapNode{
		{Ref: "e1", Role: "link", Name: "Learn more"},
		{Ref: "e2", Role: "button", Name: "Submit"},
		{Ref: "e3", Role: "textbox", Name: "Email"},
	}

	ref, ok := findRefByDescriptor(after, want)
	if !ok {
		t.Fatal("expected to re-find the 'Learn more' link after re-render")
	}
	if ref != "e1" {
		t.Fatalf("expected the link to re-resolve to e1 after reorder, got %s", ref)
	}

	// Negative: a descriptor that no longer exists must not match.
	if r, ok := findRefByDescriptor(after, RefDescriptor{Role: "button", Name: "Checkout"}); ok {
		t.Fatalf("expected no match for absent descriptor, got %s", r)
	}

	// Ambiguity guard: an empty name is too weak to re-resolve by.
	if _, ok := findRefByDescriptor(after, RefDescriptor{Role: "link", Name: ""}); ok {
		t.Fatal("expected empty-name descriptor to be rejected as ambiguous")
	}
}

func TestClipText(t *testing.T) {
	short := clipText("Hello", 10)
	if short != "Hello" {
		t.Errorf("unexpected short: %s", short)
	}
	long := clipText("1234567890abcdef", 10)
	if long != "123456789…" {
		t.Errorf("unexpected long: %s", long)
	}
}
