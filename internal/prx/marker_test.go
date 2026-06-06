package prx

import "testing"

func TestReadMarkerPlainReturnsOnlyInnerContent(t *testing.T) {
	body := "intro\n\n<!-- section:start -->\nold content\n<!-- section:end -->\noutro\n"

	got, err := readMarker(body, "section", true)
	if err != nil {
		t.Fatalf("readMarker returned error: %v", err)
	}
	if got != "old content" {
		t.Fatalf("readMarker() = %q, want %q", got, "old content")
	}
}

func TestReplaceMarkerPreservesOutsideContentAndMarkers(t *testing.T) {
	body := "intro\n\n<!-- section:start -->\nold content\n<!-- section:end -->\noutro\n"

	got, oldContent, newContent, err := replaceMarker(body, "section", "new content\n")
	if err != nil {
		t.Fatalf("replaceMarker returned error: %v", err)
	}
	want := "intro\n\n<!-- section:start -->\nnew content\n<!-- section:end -->\noutro\n"
	if got != want {
		t.Fatalf("replaceMarker body = %q, want %q", got, want)
	}
	if oldContent != "old content" {
		t.Fatalf("oldContent = %q, want old content", oldContent)
	}
	if newContent != "new content" {
		t.Fatalf("newContent = %q, want new content", newContent)
	}
}

func TestReplaceMarkerRejectsDuplicateBlocks(t *testing.T) {
	body := "<!-- section:start -->\none\n<!-- section:end -->\n<!-- section:start -->\ntwo\n<!-- section:end -->"

	_, _, _, err := replaceMarker(body, "section", "new\n")
	if err != errMarkerAmbiguous {
		t.Fatalf("replaceMarker error = %v, want errMarkerAmbiguous", err)
	}
}

func TestReplaceMarkerRejectsNestedDuplicateStart(t *testing.T) {
	body := "<!-- section:start -->\none\n<!-- section:start -->\ntwo\n<!-- section:end -->\n<!-- section:end -->"

	_, _, _, err := replaceMarker(body, "section", "new\n")
	if err != errMarkerAmbiguous {
		t.Fatalf("replaceMarker error = %v, want errMarkerAmbiguous", err)
	}
}

func TestReplaceMarkerRejectsPartialMarkerBlock(t *testing.T) {
	body := "intro\n<!-- section:start -->\npartial\n"

	_, _, _, err := replaceMarker(body, "section", "new\n")
	if err != errMarkerAmbiguous {
		t.Fatalf("replaceMarker error = %v, want errMarkerAmbiguous", err)
	}
}

func TestReplaceMarkerRejectsReplacementContainingSameMarkerToken(t *testing.T) {
	body := "<!-- section:start -->\nold\n<!-- section:end -->"

	_, _, _, err := replaceMarker(body, "section", "new\n<!-- section:end -->\n")
	if err != errReplacementContainsMarkers {
		t.Fatalf("replaceMarker error = %v, want errReplacementContainsMarkers", err)
	}
}

func TestInsertMarkerIfMissingAppendsNamedBlock(t *testing.T) {
	got, err := insertMarkerIfMissing("intro\n", "section", "new content\n")
	if err != nil {
		t.Fatalf("insertMarkerIfMissing returned error: %v", err)
	}
	want := "intro\n\n<!-- section:start -->\nnew content\n<!-- section:end -->"
	if got != want {
		t.Fatalf("insertMarkerIfMissing() = %q, want %q", got, want)
	}
}

func TestInsertMarkerIfMissingRejectsReplacementContainingSameMarkerToken(t *testing.T) {
	_, err := insertMarkerIfMissing("intro\n", "section", "new\n<!-- section:start -->\n")
	if err != errReplacementContainsMarkers {
		t.Fatalf("insertMarkerIfMissing error = %v, want errReplacementContainsMarkers", err)
	}
}
