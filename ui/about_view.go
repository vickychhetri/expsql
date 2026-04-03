package ui

import (
	"fmt"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (a *App) createAboutView() fyne.CanvasObject {

	title := widget.NewLabelWithStyle(
		"MySQL DataStream",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	version := widget.NewLabelWithStyle(
		"Version 1.0.0",
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)

	header := container.NewVBox(title, version)

	// Description Card
	descCard := widget.NewCard(
		"About",
		"",
		widget.NewLabel("High-performance MySQL export/import tool designed for large-scale data operations."),
	)

	// Features Card
	features := container.NewVBox(
		widget.NewLabel("• Concurrent export/import with configurable workers"),
		widget.NewLabel("• Handles large databases (50GB+)"),
		widget.NewLabel("• Multiple export strategies"),
		widget.NewLabel("• Separate schema and data files"),
	)

	featuresCard := widget.NewCard("Features", "", features)

	// Technical Info Card
	tech := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Go Version: %s", runtime.Version())),
		widget.NewLabel(fmt.Sprintf("OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)),
	)

	techCard := widget.NewCard("Technical Info", "", tech)

	// Author Card
	author := container.NewVBox(
		widget.NewLabelWithStyle("Vicky Chhetri", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Website: vickychhetri.com"),
		widget.NewLabel("GitHub: github.com/vickychhetri"),
	)

	authorCard := widget.NewCard("Author", "", author)

	// Footer
	footer := widget.NewLabelWithStyle(
		"© 2026 MySQL DataStream",
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)

	// Main layout (compact + centered)
	content := container.NewVBox(
		header,
		descCard,
		featuresCard,
		techCard,
		authorCard,
		footer,
	)

	return container.NewCenter(container.NewPadded(content))
}
