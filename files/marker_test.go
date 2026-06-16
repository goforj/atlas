package files

import "testing"

func TestMergeMarkerBlockAppendsWhenMissing(t *testing.T) {
	got := MergeMarkerBlock("# Notes\n", DefaultMarker, "generated")
	want := "# Notes\n\n<!-- goforj-atlas:start -->\ngenerated\n<!-- goforj-atlas:end -->\n"
	if got != want {
		t.Fatalf("unexpected merge:\n%s", got)
	}
}

func TestMergeMarkerBlockReplacesExisting(t *testing.T) {
	existing := "before\n\n<!-- goforj-atlas:start -->\nold\n<!-- goforj-atlas:end -->\n\nafter\n"
	got := MergeMarkerBlock(existing, DefaultMarker, "new")
	want := "before\n\n<!-- goforj-atlas:start -->\nnew\n<!-- goforj-atlas:end -->\n\nafter\n"
	if got != want {
		t.Fatalf("unexpected merge:\n%s", got)
	}
}

func TestMergeMarkerBlockRemovesDuplicates(t *testing.T) {
	existing := "<!-- goforj-atlas:start -->\nold\n<!-- goforj-atlas:end -->\n\nmiddle\n\n<!-- goforj-atlas:start -->\ndupe\n<!-- goforj-atlas:end -->\n"
	got := MergeMarkerBlock(existing, DefaultMarker, "new")
	want := "<!-- goforj-atlas:start -->\nnew\n<!-- goforj-atlas:end -->\n\nmiddle\n"
	if got != want {
		t.Fatalf("unexpected merge:\n%s", got)
	}
}
