package prx

import (
	"fmt"
	"strings"
)

func renderMarkerDiff(marker, oldContent, newContent string) string {
	startToken, endToken := markerTokens(marker)
	return startToken + "\n" + renderLineDiff(oldContent, newContent) + endToken + "\n"
}

func renderWholeDiff(oldContent, newContent string) string {
	return renderLineDiff(oldContent, newContent)
}

func renderLineDiff(oldContent, newContent string) string {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)
	lcs := make([][]int, len(oldLines)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var out strings.Builder
	oldWidth := numberWidth(len(oldLines))
	newWidth := numberWidth(len(newLines))
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		switch {
		case oldLines[i] == newLines[j]:
			writeDiffLine(&out, oldWidth, newWidth, i+1, j+1, ' ', oldLines[i])
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			writeDiffLine(&out, oldWidth, newWidth, i+1, 0, '-', oldLines[i])
			i++
		default:
			writeDiffLine(&out, oldWidth, newWidth, 0, j+1, '+', newLines[j])
			j++
		}
	}
	for ; i < len(oldLines); i++ {
		writeDiffLine(&out, oldWidth, newWidth, i+1, 0, '-', oldLines[i])
	}
	for ; j < len(newLines); j++ {
		writeDiffLine(&out, oldWidth, newWidth, 0, j+1, '+', newLines[j])
	}
	return out.String()
}

func writeDiffLine(out *strings.Builder, oldWidth, newWidth, oldLine, newLine int, op rune, content string) {
	writeLineNumber(out, oldWidth, oldLine)
	out.WriteByte(' ')
	writeLineNumber(out, newWidth, newLine)
	out.WriteByte(' ')
	out.WriteRune(op)
	out.WriteByte(' ')
	out.WriteString(content)
	out.WriteByte('\n')
}

func writeLineNumber(out *strings.Builder, width, line int) {
	if line == 0 {
		out.WriteString(strings.Repeat(" ", width))
		return
	}
	out.WriteString(fmt.Sprintf("%*d", width, line))
}

func numberWidth(maxLine int) int {
	width := len(fmt.Sprintf("%d", maxLine))
	if width < 1 {
		return 1
	}
	return width
}

func splitLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}
