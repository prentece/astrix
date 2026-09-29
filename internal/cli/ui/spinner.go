package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// RunWithSpinner executa uma função assíncrona enquanto renderiza um spinner animado em amarelo.
func RunWithSpinner(title string, action func() error) error {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	stop := make(chan struct{})
	done := make(chan struct{})

	spinnerStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	titleStyle := lipgloss.NewStyle().Foreground(ColorText)

	go func() {
		defer close(done)
		i := 0
		for {
			select {
			case <-stop:
				// Limpa a linha atual do spinner no terminal
				fmt.Print("\r\033[K")
				return
			default:
				frame := spinnerStyle.Render(frames[i%len(frames)])
				fmt.Printf("\r  %s %s", frame, titleStyle.Render(title))
				i++
				time.Sleep(75 * time.Millisecond)
			}
		}
	}()

	err := action()

	close(stop)
	<-done

	return err
}
