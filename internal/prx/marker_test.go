package prx

import "testing"

func TestReadMarkerPlainReturnsOnlyInnerContent(t *testing.T) {
	body := "intro\n\n<!-- summary:start -->\nold summary\n<!-- summary:end -->\noutro\n"

	got, err := readMarker(body, "summary", true)
	if err != nil {
		t.Fatalf("readMarker returned error: %v", err)
	}
	if got != "old summary" {
		t.Fatalf("readMarker() = %q, want %q", got, "old summary")
	}
}

func TestReplaceMarkerPreservesOutsideContentAndMarkers(t *testing.T) {
	body := "intro\n\n<!-- summary:start -->\nold summary\n<!-- summary:end -->\noutro\n"

	got, oldContent, newContent, err := replaceMarker(body, "summary", "new summary\n")
	if err != nil {
		t.Fatalf("replaceMarker returned error: %v", err)
	}
	want := "intro\n\n<!-- summary:start -->\nnew summary\n<!-- summary:end -->\noutro\n"
	if got != want {
		t.Fatalf("replaceMarker body = %q, want %q", got, want)
	}
	if oldContent != "old summary" {
		t.Fatalf("oldContent = %q, want old summary", oldContent)
	}
	if newContent != "new summary" {
		t.Fatalf("newContent = %q, want new summary", newContent)
	}
}

func TestReplaceMarkerRejectsDuplicateBlocks(t *testing.T) {
	body := "<!-- summary:start -->\none\n<!-- summary:end -->\n<!-- summary:start -->\ntwo\n<!-- summary:end -->"

	_, _, _, err := replaceMarker(body, "summary", "new\n")
	if err != errMarkerAmbiguous {
		t.Fatalf("replaceMarker error = %v, want errMarkerAmbiguous", err)
	}
}

func TestReplaceMarkerRejectsNestedDuplicateStart(t *testing.T) {
	body := "<!-- summary:start -->\none\n<!-- summary:start -->\ntwo\n<!-- summary:end -->\n<!-- summary:end -->"

	_, _, _, err := replaceMarker(body, "summary", "new\n")
	if err != errMarkerAmbiguous {
		t.Fatalf("replaceMarker error = %v, want errMarkerAmbiguous", err)
	}
}

func TestInsertMarkerIfMissingAppendsNamedBlock(t *testing.T) {
	got := insertMarkerIfMissing("intro\n", "summary", "new summary\n")
	want := "intro\n\n<!-- summary:start -->\nnew summary\n<!-- summary:end -->"
	if got != want {
		t.Fatalf("insertMarkerIfMissing() = %q, want %q", got, want)
	}
}
