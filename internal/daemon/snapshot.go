package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
)

// SnapNode represents one node in an accessibility snapshot, labeled with an e-ref.
type SnapNode struct {
	Ref   string   `json:"ref"`
	Depth int      `json:"depth"`
	Role  string   `json:"role"`
	Name  string   `json:"name,omitempty"`
	Value string   `json:"value,omitempty"`
	Props []string `json:"props,omitempty"`
}

// Snapshot is the formatted accessibility snapshot of a tab.
type Snapshot struct {
	URL       string     `json:"url"`
	Title     string     `json:"title"`
	Nodes     []SnapNode `json:"nodes"`
	Hash      string     `json:"hash,omitempty"`
	Diff      string     `json:"diff,omitempty"`
	Unchanged bool       `json:"unchanged,omitempty"`
}

var skipRoles = map[string]bool{
	"generic": true, "none": true, "presentation": true, "InlineTextBox": true,
	"LineBreak": true, "RootWebArea": true, "paragraph": true, "group": true,
	"Section": true, "LayoutTable": true, "LayoutTableRow": true, "LayoutTableCell": true,
	"LabelText": true, "MenuListPopup": true, "ListMarker": true, "separator": true,
}

var boolProps = map[string]bool{
	"focused": true, "disabled": true, "checked": true, "expanded": true,
	"selected": true, "required": true, "readonly": true, "pressed": true,
	"modal": true, "multiselectable": true,
}

var leafRoles = map[string]bool{
	"textbox": true, "searchbox": true, "spinbutton": true,
}

// buildSnapshot traverses an accessibility tree and assigns sequential e1, e2... refs.
func buildSnapshot(nodes []*accessibility.Node) ([]SnapNode, map[string]cdp.BackendNodeID) {
	byID := make(map[accessibility.NodeID]*accessibility.Node, len(nodes))
	var root *accessibility.Node
	for _, n := range nodes {
		byID[n.NodeID] = n
		if n.ParentID == "" && root == nil {
			root = n
		}
	}
	b := &snapBuilder{
		byID: byID,
		refs: make(map[string]cdp.BackendNodeID),
	}
	if root != nil {
		b.walk(root, 0, "")
	}
	return b.out, b.refs
}

type snapBuilder struct {
	byID map[accessibility.NodeID]*accessibility.Node
	out  []SnapNode
	refs map[string]cdp.BackendNodeID
}

func (b *snapBuilder) walk(n *accessibility.Node, depth int, parentName string) {
	role := axString(n.Role)
	name := axString(n.Name)

	if role == "generic" && editable(n) {
		role = "textbox"
	}

	keep := !n.Ignored && !skipRoles[role] && role != ""
	if role == "StaticText" && (!hasWord(name) || name == parentName) {
		keep = false
	}

	childDepth := depth
	childParent := parentName

	if keep && leafRoles[role] {
		b.add(n, depth, role, name)
		return
	}

	if keep {
		b.add(n, depth, role, name)
		childDepth = depth + 1
		childParent = name
	}

	for _, id := range n.ChildIDs {
		if c := b.byID[id]; c != nil {
			b.walk(c, childDepth, childParent)
		}
	}
}

func (b *snapBuilder) add(n *accessibility.Node, depth int, role, name string) {
	ref := fmt.Sprintf("e%d", len(b.out)+1)
	if n.BackendDOMNodeID != 0 {
		b.refs[ref] = n.BackendDOMNodeID
	}
	b.out = append(b.out, SnapNode{
		Ref:   ref,
		Depth: depth,
		Role:  role,
		Name:  clipText(name, 80),
		Value: clipText(axString(n.Value), 80),
		Props: axProps(n),
	})
}

func hasWord(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func editable(n *accessibility.Node) bool {
	for _, p := range n.Properties {
		if p.Name == "editable" && axString(p.Value) != "" {
			return true
		}
	}
	return false
}

func axString(v *accessibility.Value) string {
	if v == nil || len(v.Value) == 0 {
		return ""
	}
	var x any
	if err := json.Unmarshal([]byte(v.Value), &x); err != nil {
		return strings.Trim(string(v.Value), "\"")
	}
	switch t := x.(type) {
	case string:
		return strings.Join(strings.Fields(t), " ")
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func axProps(n *accessibility.Node) []string {
	var out []string
	for _, p := range n.Properties {
		name := string(p.Name)
		val := axString(p.Value)
		switch {
		case boolProps[name] && (val == "true" || val == "mixed"):
			if val == "mixed" {
				name += "=mixed"
			}
			out = append(out, name)
		case name == "level" && val != "":
			out = append(out, "level="+val)
		}
	}
	return out
}

func clipText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Text formats the snapshot as readable indented lines with refs, or diff if present.
func (s Snapshot) Text() string {
	if s.Unchanged {
		return "unchanged\n"
	}
	if s.Diff != "" {
		return s.Diff
	}
	var b strings.Builder
	if s.Title != "" {
		b.WriteString(s.Title)
		b.WriteByte('\n')
	}
	if s.URL != "" {
		b.WriteString(s.URL)
		b.WriteByte('\n')
	}
	for _, n := range s.Nodes {
		b.WriteString(strings.Repeat("  ", n.Depth))
		fmt.Fprintf(&b, "%s %s", n.Ref, n.Role)
		if n.Name != "" {
			fmt.Fprintf(&b, " %q", n.Name)
		}
		if n.Value != "" && n.Value != n.Name {
			fmt.Fprintf(&b, " value=%q", n.Value)
		}
		for _, p := range n.Props {
			fmt.Fprintf(&b, " [%s]", p)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
