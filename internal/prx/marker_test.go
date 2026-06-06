package prx

import "testing"

func TestReadMarkerPlainReturnsOnlyInnerContent(t *testing.T) {
	body := "intro\n\n<!-- ai-summary:start -->\nold summary\n<!-- ai-summary:end -->\noutro\n"

	got, err := readMarker(body, "ai-summary", true)
	if err != nil {
		t.Fatalf("readMarker returned error: %v", err)
	}
	if got != "old summary" {
		t.Fatalf("readMarker() = %q, want %q", got, "old summary")
	}
}

func TestReplaceMarkerPreservesOutsideContentAndMarkers(t *testing.T) {
	body := "intro\n\n<!-- ai-summary:start -->\nold summary\n<!-- ai-summary:end -->\noutro\n"

	got, oldContent, newContent, err := replaceMarker(body, "ai-summary", "new summary\n")
	if err != nil {
		t.Fatalf("replaceMarker returned error: %v", err)
	}
	want := "intro\n\n<!-- ai-summary:start -->\nnew summary\n<!-- ai-summary:end -->\noutro\n"
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
	body := "<!-- ai-summary:start -->\none\n<!-- ai-summary:end -->\n<!-- ai-summary:start -->\ntwo\n<!-- ai-summary:end -->"

	_, _, _, err := replaceMarker(body, "ai-summary", "new\n")
	if err != errMarkerAmbiguous {
		t.Fatalf("replaceMarker error = %v, want errMarkerAmbiguous", err)
	}
}

func TestInsertMarkerIfMissingAppendsNamedBlock(t *testing.T) {
	got := insertMarkerIfMissing("intro\n", "ai-summary", "new summary\n")
	want := "intro\n\n<!-- ai-summary:start -->\nnew summary\n<!-- ai-summary:end -->"
	if got != want {
		t.Fatalf("insertMarkerIfMissing() = %q, want %q", got, want)
	}
}
