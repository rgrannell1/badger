package main

import (
	"fmt"

	"github.com/rivo/tview"
)

type TUI struct {
	app        *tview.Application
	facts      *Facts
	photoCount int
	rawCount   int
	videoCount int
}

/*
 * Create a progress-bar
 */
func NewProgressBar(count int64, facts *Facts) *TUI {
	tui := TUI{}

	app := tview.NewApplication()
	app.EnableMouse(false)

	tui.app = app

	return &tui
}

/*
 * Receive a media item,and update the progress bar
 */
func (tui *TUI) Update(media *Media) {

}

func (tui *TUI) SummaryText() *tview.TextView {
	return tview.NewTextView()
}

/*
 * Initialise a grid containing all progress-information to share
 */
func (tui *TUI) Grid() *tview.Grid {
	return tview.NewGrid().
		SetRows(1).SetColumns(1, 1).AddItem(tui.SummaryText(), 1, 1, 2, 1, 1, 1, true)
}

/*
 * Stat the tcell progress-bar
 */
func (tui *TUI) Start() int {
	grid := tui.Grid()

	if err := tui.app.SetRoot(grid, true).SetFocus(grid).Run(); err != nil {
		fmt.Printf("Badger: Application crashed! %v", err)
		return 1
	}

	return 0
}
