package prx

import (
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
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		switch {
		case oldLines[i] == newLines[j]:
			out.WriteString("  ")
			out.WriteString(oldLines[i])
			out.WriteByte('\n')
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out.WriteString("- ")
			out.WriteString(oldLines[i])
			out.WriteByte('\n')
			i++
		default:
			out.WriteString("+ ")
			out.WriteString(newLines[j])
			out.WriteByte('\n')
			j++
		}
	}
	for ; i < len(oldLines); i++ {
		out.WriteString("- ")
		out.WriteString(oldLines[i])
		out.WriteByte('\n')
	}
	for ; j < len(newLines); j++ {
		out.WriteString("+ ")
		out.WriteString(newLines[j])
		out.WriteByte('\n')
	}
	return out.String()
}

func splitLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}
