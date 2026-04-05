package ui

import (
	"log"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type App struct {
	Window           fyne.Window
	App              fyne.App
	CurrentView      fyne.CanvasObject
	ContentContainer *fyne.Container
	StatusBar        *widget.Label
	Settings         *Settings
}

func StartGUI() {
	myApp := app.NewWithID("com.mysqltool.datastream")

	icon, err := fyne.LoadResourceFromPath("assets/icon.png")
	if err != nil {
		log.Println("icon load failed:", err)
	} else {
		myApp.SetIcon(icon)
	}

	myWindow := myApp.NewWindow("MySQL DataStream")
	myWindow.Resize(fyne.NewSize(1000, 700))
	myWindow.CenterOnScreen()

	settings := LoadSettings()

	app := &App{
		Window:    myWindow,
		App:       myApp,
		Settings:  settings,
		StatusBar: widget.NewLabel("Ready"),
	}

	app.setupUI()
	myWindow.ShowAndRun()
}

func (a *App) setupUI() {
	navBar := a.createNavBar()

	a.CurrentView = a.createDashboard()

	a.ContentContainer = container.NewStack(a.CurrentView)

	statusContainer := container.NewBorder(
		nil, nil, nil, nil,
		container.NewVBox(
			widget.NewSeparator(),
			container.NewHBox(
				a.StatusBar,
				widget.NewLabel("|"),
				widget.NewLabel(runtime.GOOS+"/"+runtime.GOARCH),
			),
		),
	)

	mainContainer := container.NewBorder(
		navBar,
		statusContainer,
		nil,
		nil,
		a.ContentContainer,
	)

	a.Window.SetContent(mainContainer)
}

func (a *App) createNavBar() *fyne.Container {

	makeBtn := func(label string, icon fyne.Resource, view func() fyne.CanvasObject) *widget.Button {
		return widget.NewButtonWithIcon(label, icon, func() {
			a.CurrentView = view()
			a.updateContent()
		})
	}

	return container.NewHBox(
		makeBtn("Dashboard", theme.HomeIcon(), a.createDashboard),
		makeBtn("Export", theme.DownloadIcon(), a.createExportView),
		makeBtn("Import", theme.UploadIcon(), a.createImportView),
		makeBtn("Settings", theme.SettingsIcon(), a.createSettingsView),
		makeBtn("About", theme.InfoIcon(), a.createAboutView),
	)
}

func (a *App) updateContent() {
	a.ContentContainer.Objects = []fyne.CanvasObject{a.CurrentView}
	a.ContentContainer.Refresh()
	a.updateStatus("View updated")
}

func (a *App) updateStatus(message string) {
	a.StatusBar.SetText(message)
}
