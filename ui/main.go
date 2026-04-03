package ui

import (
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

	// default view
	a.CurrentView = a.createDashboard()

	// ✅ container to swap views
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
		a.ContentContainer, // ✅ use stack
	)

	a.Window.SetContent(mainContainer)
}

func (a *App) createNavBar() *fyne.Container {
	dashboardBtn := widget.NewButtonWithIcon("Dashboard", theme.HomeIcon(), func() {
		a.CurrentView = a.createDashboard()
		a.updateContent()
	})

	exportBtn := widget.NewButtonWithIcon("Export", theme.DownloadIcon(), func() {
		a.CurrentView = a.createExportView()
		a.updateContent()
	})

	importBtn := widget.NewButtonWithIcon("Import", theme.UploadIcon(), func() {
		a.CurrentView = a.createImportView()
		a.updateContent()
	})

	settingsBtn := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		a.CurrentView = a.createSettingsView()
		a.updateContent()
	})

	aboutBtn := widget.NewButtonWithIcon("About", theme.InfoIcon(), func() {
		a.CurrentView = a.createAboutView()
		a.updateContent()
	})

	return container.NewHBox(
		dashboardBtn,
		exportBtn,
		importBtn,
		settingsBtn,
		aboutBtn,
	)
}

func (a *App) updateContent() {
	a.ContentContainer.Objects = []fyne.CanvasObject{a.CurrentView}
	a.ContentContainer.Refresh()

	a.updateStatus("View updated")
}

// func (a *App) updateContent() {
// 	// Get the current content container
// 	currentContent := a.Window.Content()

// 	// Create new container with updated view
// 	// The current container has 3 parts: top (navbar), bottom (status), center (content)
// 	if currentContainer, ok := currentContent.(*fyne.Container); ok && len(currentContainer.Objects) >= 3 {
// 		newContainer := container.NewBorder(
// 			currentContainer.Objects[0], // navbar (top)
// 			currentContainer.Objects[1], // status bar (bottom)
// 			nil,                         // left
// 			nil,                         // right
// 			a.CurrentView,               // new center content
// 		)
// 		a.Window.SetContent(newContainer)
// 	}
// 	a.updateStatus("View updated")
// }

func (a *App) updateStatus(message string) {
	a.StatusBar.SetText(message)
}
