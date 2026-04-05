package ui

import (
	"fmt"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *App) createDashboard() fyne.CanvasObject {

	title := widget.NewLabelWithStyle(
		"MySQL DataStream Dashboard",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	subTitle := widget.NewLabel("Monitor exports, imports, and system performance")

	stats := a.createStatsCards()

	quickActions := a.createQuickActions()
	systemInfo := a.createSystemInfo()
	runtimeInfo := a.createRuntimeInfo()

	grid := container.NewGridWithColumns(2,
		quickActions,
		container.NewVBox(systemInfo, runtimeInfo),
	)

	content := container.NewVBox(
		title,
		subTitle,
		widget.NewSeparator(),
		stats,
		widget.NewSeparator(),
		grid,
	)

	return container.NewPadded(container.NewScroll(content))
}

// -------------------- STATS --------------------

func (a *App) createStatsCards() fyne.CanvasObject {

	card := func(title, value string, icon fyne.Resource) fyne.CanvasObject {
		return widget.NewCard(
			"",
			"",
			container.NewVBox(
				widget.NewIcon(icon),
				widget.NewLabel(title),
				widget.NewLabelWithStyle(value, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			),
		)
	}

	return container.NewGridWithColumns(4,
		card("Total Exports", "0", theme.DownloadIcon()),
		card("Total Imports", "0", theme.UploadIcon()),
		card("Last Backup", time.Now().Format("02 Jan 2006"), theme.HistoryIcon()),
		card("DB Size", "N/A", theme.StorageIcon()),
	)
}

// -------------------- QUICK ACTIONS --------------------

func (a *App) createQuickActions() fyne.CanvasObject {

	btn := func(label string, icon fyne.Resource, action func()) fyne.CanvasObject {
		b := widget.NewButtonWithIcon(label, icon, action)
		b.Importance = widget.HighImportance
		return b
	}

	exportBtn := btn("Quick Export", theme.DownloadIcon(), func() {
		a.CurrentView = a.createExportView()
		a.updateContent()
	})

	importBtn := btn("Quick Import", theme.UploadIcon(), func() {
		a.CurrentView = a.createImportView()
		a.updateContent()
	})

	settingsBtn := btn("Settings", theme.SettingsIcon(), func() {
		a.CurrentView = a.createSettingsView()
		a.updateContent()
	})

	mysqlImportBtn := widget.NewButton("MySQL Client Import", func() {
		a.CurrentView = a.createMySQLClientImportView()
		a.updateContent()
	})

	return widget.NewCard(
		"Quick Actions",
		"",
		container.NewVBox(
			exportBtn,
			importBtn,
			settingsBtn,
			mysqlImportBtn,
		),
	)
}

// -------------------- SYSTEM INFO --------------------

func (a *App) createSystemInfo() fyne.CanvasObject {

	info := container.NewVBox()

	update := func() {
		info.Objects = []fyne.CanvasObject{
			widget.NewLabel(fmt.Sprintf("OS: %s", runtime.GOOS)),
			widget.NewLabel(fmt.Sprintf("ARCH: %s", runtime.GOARCH)),
			widget.NewLabel(fmt.Sprintf("CPU Cores: %d", runtime.NumCPU())),
			widget.NewLabel(fmt.Sprintf("Go Version: %s", runtime.Version())),
		}
		info.Refresh()
	}

	update()

	return widget.NewCard("System Info", "", info)
}

// -------------------- RUNTIME INFO (LIVE) --------------------

func (a *App) createRuntimeInfo() fyne.CanvasObject {

	memLabel := widget.NewLabel("")
	goroutineLabel := widget.NewLabel("")
	gcLabel := widget.NewLabel("")

	update := func() {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		memLabel.SetText(fmt.Sprintf("Memory Used: %.2f MB", float64(m.Alloc)/1024/1024))
		goroutineLabel.SetText(fmt.Sprintf("Goroutines: %d", runtime.NumGoroutine()))
		gcLabel.SetText(fmt.Sprintf("GC Cycles: %d", m.NumGC))
	}

	update()

	// auto refresh every 2 sec
	go func() {
		for {
			time.Sleep(2 * time.Second)
			fyne.Do(update)
		}
	}()

	return widget.NewCard(
		"Runtime Metrics (Live)",
		"",
		container.NewVBox(
			memLabel,
			goroutineLabel,
			gcLabel,
		),
	)
}
