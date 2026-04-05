package ui

import (
	"fmt"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (a *App) createAboutView() fyne.CanvasObject {

	makeText := func(s string, bold bool) *widget.Label {
		t := widget.NewLabel(s)
		t.Alignment = fyne.TextAlignCenter
		t.Wrapping = fyne.TextWrapWord
		if bold {
			t.TextStyle = fyne.TextStyle{Bold: true}
		}
		return t
	}

	title := makeText("MySQL DataStream", true)
	version := makeText("v1.0.0", false)

	header := container.NewVBox(title, version)

	desc := widget.NewCard("", "",
		makeText("High-performance MySQL export/import tool for large-scale data operations.", false),
	)

	features := widget.NewCard("Key Features", "",
		container.NewVBox(
			makeText("• Concurrent processing", false),
			makeText("• Large dataset support", false),
			makeText("• Multiple export strategies", false),
			makeText("• Schema & data separation", false),
		),
	)

	tech := widget.NewCard("System", "",
		container.NewVBox(
			makeText(runtime.Version(), false),
			makeText(fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), false),
		),
	)

	author := widget.NewCard("", "",
		container.NewVBox(
			makeText("Vicky Chhetri", true),
			makeText("vickychhetri.com", false),
		),
	)

	footer := makeText("© 2026", false)

	content := container.NewVBox(
		header,
		desc,
		features,
		tech,
		author,
		footer,
	)

	wrapped := container.NewCenter(
		container.NewGridWrap(
			fyne.NewSize(420, 600),
			container.NewPadded(content),
		),
	)

	return container.NewVScroll(wrapped)
}
