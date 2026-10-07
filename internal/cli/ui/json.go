package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FormatJSONHighlight aplica syntax highlighting limpo e minimalista a um snippet JSON.
func FormatJSONHighlight(rawJSON string) string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorError) // Vermelho (Calças do Asterix)
	stringStyle := lipgloss.NewStyle().Foreground(ColorPrimary)       // Amarelo (Cabelo e Bigode)
	punctStyle := lipgloss.NewStyle().Foreground(ColorDarkMuted)      // Cinzento escuro (sem poluição)
	numBoolStyle := lipgloss.NewStyle().Foreground(ColorGold)         // Dourado (Tachas)
	valStyle := lipgloss.NewStyle().Foreground(ColorText)             // Branco brilhante

	lines := strings.Split(strings.TrimRight(rawJSON, "\n"), "\n")
	var formattedLines []string

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		indent := line[:len(line)-len(trimmed)]

		if trimmed == "{" || trimmed == "}" || trimmed == "[" || trimmed == "]" ||
			trimmed == "}," || trimmed == "]," {
			formattedLines = append(formattedLines, indent+punctStyle.Render(trimmed))
			continue
		}

		colonIdx := strings.Index(trimmed, ": ")
		if colonIdx != -1 {
			k := trimmed[:colonIdx]
			v := trimmed[colonIdx+2:]

			hlKey := keyStyle.Render(k)
			hlColon := punctStyle.Render(": ")

			var hlVal string
			comma := ""
			if strings.HasSuffix(v, ",") {
				comma = punctStyle.Render(",")
				v = strings.TrimSuffix(v, ",")
			}

			if v == "{" || v == "[" {
				hlVal = punctStyle.Render(v)
			} else if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
				// Inline array como ["mcp", "serve"]
				items := strings.Split(v[1:len(v)-1], ", ")
				var formattedItems []string
				for _, item := range items {
					itemTrimmed := strings.TrimSpace(item)
					if strings.HasPrefix(itemTrimmed, "\"") && strings.HasSuffix(itemTrimmed, "\"") {
						formattedItems = append(formattedItems, stringStyle.Render(itemTrimmed))
					} else {
						formattedItems = append(formattedItems, valStyle.Render(itemTrimmed))
					}
				}
				hlVal = punctStyle.Render("[") + strings.Join(formattedItems, punctStyle.Render(", ")) + punctStyle.Render("]")
			} else if strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
				hlVal = stringStyle.Render(v)
			} else if v == "true" || v == "false" || v == "null" {
				hlVal = numBoolStyle.Render(v)
			} else {
				hlVal = numBoolStyle.Render(v)
			}

			formattedLines = append(formattedLines, indent+hlKey+hlColon+hlVal+comma)
		} else {
			formattedLines = append(formattedLines, indent+valStyle.Render(trimmed))
		}
	}

	return strings.Join(formattedLines, "\n")
}
