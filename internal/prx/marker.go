package prx

import (
	"fmt"
	"strings"
)

type MarkerRange struct {
	Marker     string
	StartToken string
	EndToken   string
	Start      int
	Content    int
	End        int
	AfterEnd   int
}

func markerTokens(marker string) (string, string) {
	return "<!-- " + marker + ":start -->", "<!-- " + marker + ":end -->"
}

func findMarkerRange(body, marker string) (MarkerRange, error) {
	startToken, endToken := markerTokens(marker)
	start := strings.Index(body, startToken)
	if start < 0 {
		return MarkerRange{}, fmt.Errorf("marker not found")
	}
	content := start + len(startToken)
	endRel := strings.Index(body[content:], endToken)
	if endRel < 0 {
		return MarkerRange{}, fmt.Errorf("marker not found")
	}
	end := content + endRel
	afterEnd := end + len(endToken)
	return MarkerRange{
		Marker:     marker,
		StartToken: startToken,
		EndToken:   endToken,
		Start:      start,
		Content:    content,
		End:        end,
		AfterEnd:   afterEnd,
	}, nil
}

func markerBlock(marker, content string) string {
	startToken, endToken := markerTokens(marker)
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return startToken + "\n" + endToken
	}
	return startToken + "\n" + content + "\n" + endToken
}

func readMarker(body, marker string, plain bool) (string, error) {
	r, err := findMarkerRange(body, marker)
	if err != nil {
		return "", err
	}
	if !plain {
		return body[r.Start:r.AfterEnd], nil
	}
	return trimMarkerContent(body[r.Content:r.End]), nil
}

func replaceMarker(body, marker, replacement string) (string, string, string, error) {
	r, err := findMarkerRange(body, marker)
	if err != nil {
		return "", "", "", err
	}
	oldContent := trimMarkerContent(body[r.Content:r.End])
	newBlock := markerBlock(marker, replacement)
	return body[:r.Start] + newBlock + body[r.AfterEnd:], oldContent, strings.TrimSuffix(replacement, "\n"), nil
}

func insertMarkerIfMissing(body, marker, content string) string {
	block := markerBlock(marker, content)
	if strings.TrimSpace(body) == "" {
		return block
	}
	separator := "\n\n"
	if strings.HasSuffix(body, "\n") {
		separator = "\n"
	}
	return body + separator + block
}

func trimMarkerContent(content string) string {
	content = strings.TrimPrefix(content, "\r\n")
	content = strings.TrimPrefix(content, "\n")
	content = strings.TrimSuffix(content, "\r\n")
	content = strings.TrimSuffix(content, "\n")
	return content
}

func containsMarker(body, marker string) bool {
	_, err := findMarkerRange(body, marker)
	return err == nil
}
