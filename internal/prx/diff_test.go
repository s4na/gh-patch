package prx

import (
	"strings"
	"testing"
)

func TestRenderLineDiffIncludesOldAndNewLineNumbers(t *testing.T) {
	got := renderLineDiff("keep\nold summary\nold risk note\n", "keep\nnew summary\nnew risk note\n")
	want := strings.Join([]string{
		"old | new | op | content",
		"  1 |   1 |    | keep",
		"  2 |     |  - | old summary",
		"  3 |     |  - | old risk note",
		"    |   2 |  + | new summary",
		"    |   3 |  + | new risk note",
		"",
	}, "\n")

	if got != want {
		t.Fatalf("renderLineDiff() = %q, want %q", got, want)
	}
}

func TestRenderLineDiffPadsMultiDigitLineNumbers(t *testing.T) {
	oldContent := strings.Join([]string{"01", "02", "03", "04", "05", "06", "07", "08", "09", "10"}, "\n")
	got := renderLineDiff(oldContent, oldContent+"\n11")

	if !strings.Contains(got, "old | new | op | content\n") {
		t.Fatalf("renderLineDiff() = %q, want header", got)
	}
	if !strings.Contains(got, "  1 |   1 |    | 01\n") {
		t.Fatalf("renderLineDiff() = %q, want one-digit line numbers padded to width 2", got)
	}
	if !strings.Contains(got, " 10 |  10 |    | 10\n") {
		t.Fatalf("renderLineDiff() = %q, want padded unchanged line 10", got)
	}
	if !strings.Contains(got, "    |  11 |  + | 11\n") {
		t.Fatalf("renderLineDiff() = %q, want padded added line 11", got)
	}
}
