package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/petrusjohannesmaas/esque/internal/config"
	"github.com/petrusjohannesmaas/esque/internal/remote"
	"github.com/petrusjohannesmaas/esque/internal/tui"
)

func main() {
	path, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "esque:", err)
		os.Exit(1)
	}
	model, err := tui.New(path, tui.Deps{OpenStore: remote.NewS3, Now: time.Now})
	if err != nil {
		fmt.Fprintln(os.Stderr, "esque:", err)
		os.Exit(1)
	}
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "esque:", err)
		os.Exit(1)
	}
}
