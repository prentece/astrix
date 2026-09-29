package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Cores da Identidade Visual Astrix
var (
	ColorPrimary   = lipgloss.Color("#FFCC00") // Cabelo e Bigode (Amarelo Asterix)
	ColorGold      = lipgloss.Color("#FFD700") // Tachas do Cinto (Douradas)
	ColorSkin      = lipgloss.Color("#FFDAB9") // Tom de Pele (Peach/Salmão suave)
	ColorSuccess   = lipgloss.Color("#2E8B57") // Cinto (Verde)
	ColorError     = lipgloss.Color("#E3242B") // Calças (Vermelho Asterix)
	ColorWarning   = lipgloss.Color("#FFCC00") // Amarelo
	ColorMuted     = lipgloss.Color("#A9A9A9") // Capacete (Cinza/Prateado claro para rótulos)
	ColorDarkMuted = lipgloss.Color("#4B5563") // Cinzento escuro para pontuação JSON
	ColorText      = lipgloss.Color("#FFFFFF") // Asas do Capacete (Branco brilhante para valores)
	ColorBrown     = lipgloss.Color("#8B4513") // Cantil e Sapatos (Marrom)
	ColorDark      = lipgloss.Color("#1A1A1A") // Camisa (Preto)
)

// Estilos de Texto
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	LabelStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(22)

	ValueStyle = lipgloss.NewStyle().
			Foreground(ColorText)
)

// ClearScreen limpa o terminal e move o cursor para o topo.
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

// EnterAltScreen ativa o buffer alternativo do terminal para evitar poluir o histórico de comandos.
func EnterAltScreen() {
	fmt.Print("\033[?1049h\033[H")
}

// ExitAltScreen desativa o buffer alternativo e restaura a tela original do terminal.
func ExitAltScreen() {
	fmt.Print("\033[?1049l")
}

// Banner renderiza o cabeçalho minimalista do Astrix.
func Banner(subtitle string) string {
	title := TitleStyle.Render("ASTRIX")
	if subtitle != "" {
		return fmt.Sprintf("%s  %s\n", title, SubtitleStyle.Render("— "+subtitle))
	}
	return title + "\n"
}

// HeaderCard renderiza o cabeçalho minimalista com metadados do projeto.
func HeaderCard(title, subtitle, status, path string, isReady bool) string {
	statusStyle := lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	if !isReady {
		statusStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	}

	return fmt.Sprintf("%s\n%s %s\n%s %s\n%s",
		TitleStyle.Render(title),
		SubtitleStyle.Render("Contexto:"), ValueStyle.Render(subtitle),
		SubtitleStyle.Render("Status:  "), statusStyle.Render(status),
		SubtitleStyle.Render(path),
	)
}

// ActionHeader exibe o subtítulo da ação selecionada em vermelho para separação visual imediata.
func ActionHeader(title string) string {
	bullet := lipgloss.NewStyle().Bold(true).Foreground(ColorError).Render("›")
	t := lipgloss.NewStyle().Bold(true).Foreground(ColorError).Render(title)
	return fmt.Sprintf("  %s %s", bullet, t)
}

// SuccessBox renderiza uma mensagem de sucesso minimalista de linha única.
func SuccessBox(message, detail string) string {
	badge := lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess).Render("[OK]")
	msg := ValueStyle.Render(message)
	if detail != "" {
		return fmt.Sprintf("%s %s — %s\n", badge, msg, SubtitleStyle.Render(detail))
	}
	return fmt.Sprintf("%s %s\n", badge, msg)
}

// WarningBox renderiza uma mensagem de aviso minimalista de linha única.
func WarningBox(message, detail string) string {
	badge := lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).Render("[AVISO]")
	msg := ValueStyle.Render(message)
	if detail != "" {
		return fmt.Sprintf("%s %s — %s\n", badge, msg, SubtitleStyle.Render(detail))
	}
	return fmt.Sprintf("%s %s\n", badge, msg)
}

// ErrorBox renderiza uma mensagem de erro minimalista de linha única.
func ErrorBox(message, detail string) string {
	badge := lipgloss.NewStyle().Bold(true).Foreground(ColorError).Render("[ERRO]")
	msg := ValueStyle.Render(message)
	if detail != "" {
		return fmt.Sprintf("%s %s — %s\n", badge, msg, SubtitleStyle.Render(detail))
	}
	return fmt.Sprintf("%s %s\n", badge, msg)
}

// KeyValue renderiza uma linha chave/valor alinhada na margem esquerda.
func KeyValue(key, value string) string {
	return fmt.Sprintf("%s %s", LabelStyle.Render(key+":"), ValueStyle.Render(value))
}

// HuhTheme cria o tema minimalista com seta de seleção e acento amarelo sem saltos de layout.
func HuhTheme() *huh.Theme {
	t := huh.ThemeBase()

	yellow := ColorPrimary
	muted := ColorMuted
	white := ColorText

	// Padroniza base sem margens divergentes entre focado e desfocado
	baseStyle := lipgloss.NewStyle().PaddingLeft(0)
	t.Focused.Base = baseStyle
	t.Blurred.Base = baseStyle

	t.Focused.Title = lipgloss.NewStyle().Bold(true).Foreground(yellow)
	t.Focused.Description = lipgloss.NewStyle().Foreground(muted)
	t.Focused.SelectSelector = lipgloss.NewStyle().Bold(true).Foreground(yellow).SetString("> ")
	t.Focused.Option = lipgloss.NewStyle().Foreground(white)
	t.Focused.SelectedOption = lipgloss.NewStyle().Bold(true).Foreground(yellow)
	t.Focused.UnselectedOption = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Bold(true).Foreground(yellow).SetString("[x] ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(muted).SetString("[ ] ")
	t.Focused.FocusedButton = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000000")).Background(yellow).Padding(0, 2)
	t.Focused.BlurredButton = lipgloss.NewStyle().Foreground(muted).Padding(0, 2)
	t.Focused.TextInput.Prompt = lipgloss.NewStyle().SetString("")
	t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(yellow)
	t.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(muted)
	t.Focused.TextInput.Text = lipgloss.NewStyle().Foreground(white)

	t.Blurred.Title = lipgloss.NewStyle().Foreground(muted)
	t.Blurred.Description = lipgloss.NewStyle().Foreground(muted)
	t.Blurred.SelectSelector = lipgloss.NewStyle().SetString("  ")
	t.Blurred.Option = lipgloss.NewStyle().Foreground(muted)
	t.Blurred.SelectedOption = lipgloss.NewStyle().Foreground(muted)
	t.Blurred.UnselectedOption = lipgloss.NewStyle().Foreground(muted)
	t.Blurred.SelectedPrefix = lipgloss.NewStyle().Foreground(muted).SetString("[x] ")
	t.Blurred.UnselectedPrefix = lipgloss.NewStyle().Foreground(muted).SetString("[ ] ")
	t.Blurred.TextInput.Prompt = lipgloss.NewStyle().SetString("")
	t.Blurred.TextInput.Text = lipgloss.NewStyle().Foreground(muted)

	t.Help.ShortKey = lipgloss.NewStyle().Bold(true).Foreground(yellow)
	t.Help.ShortDesc = lipgloss.NewStyle().Foreground(muted)
	t.Help.ShortSeparator = lipgloss.NewStyle().Foreground(ColorDarkMuted)
	t.Help.Ellipsis = lipgloss.NewStyle().Foreground(ColorDarkMuted)

	return t
}

// HuhKeyMap retorna o mapeamento de teclas com suporte prioritário a tecla Esc para voltar/cancelar.
func HuhKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "sair"),
	)
	km.Select.Prev = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "sair"),
	)
	km.Select.Next = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "avançar"),
	)
	km.Select.Up = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/↓", "navegar"),
	)
	km.Select.Down = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↑/↓", "navegar"),
	)
	km.Select.Submit = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "selecionar"),
	)
	km.Select.Filter = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "sair"),
	)
	km.MultiSelect.Toggle = key.NewBinding(
		key.WithKeys(" ", "x"),
		key.WithHelp("espaço", "marcar"),
	)
	km.MultiSelect.Up = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/↓", "navegar"),
	)
	km.MultiSelect.Down = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↑/↓", "navegar"),
	)
	km.MultiSelect.Prev = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "sair"),
	)
	km.MultiSelect.Submit = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "salvar"),
	)
	km.MultiSelect.Filter = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "sair"),
	)
	km.Input.Prev = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "sair"),
	)
	km.Confirm.Prev = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "sair"),
	)
	return km
}

