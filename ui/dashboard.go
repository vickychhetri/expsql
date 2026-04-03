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
	// Welcome message
	welcomeLabel := widget.NewLabelWithStyle("Welcome to MySQL DataStream", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	// Stats cards
	stats := a.createStatsCards()

	// Quick actions
	quickActions := a.createQuickActions()

	// System info
	systemInfo := a.createSystemInfo()

	// Layout
	content := container.NewVBox(
		welcomeLabel,
		widget.NewSeparator(),
		stats,
		widget.NewSeparator(),
		container.NewGridWithColumns(2, quickActions, systemInfo),
	)

	return container.NewScroll(content)
}

func (a *App) createStatsCards() fyne.CanvasObject {
	card1 := a.createStatCard("Total Exports", "0", theme.DownloadIcon())
	card2 := a.createStatCard("Total Imports", "0", theme.UploadIcon())
	card3 := a.createStatCard("Last Backup", time.Now().Format("2006-01-02"), theme.HistoryIcon())
	card4 := a.createStatCard("Database Size", "N/A", theme.StorageIcon())

	return container.NewGridWithColumns(4, card1, card2, card3, card4)
}

func (a *App) createStatCard(title, value string, icon fyne.Resource) fyne.CanvasObject {
	card := widget.NewCard(
		title,
		"",
		container.NewVBox(
			widget.NewIcon(icon),
			widget.NewLabelWithStyle(value, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		),
	)
	return card
}

func (a *App) createQuickActions() fyne.CanvasObject {
	exportBtn := widget.NewButtonWithIcon("Quick Export", theme.DownloadIcon(), func() {
		a.CurrentView = a.createExportView()
		a.updateContent()
	})

	importBtn := widget.NewButtonWithIcon("Quick Import", theme.UploadIcon(), func() {
		a.CurrentView = a.createImportView()
		a.updateContent()
	})

	settingsBtn := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		a.CurrentView = a.createSettingsView()
		a.updateContent()
	})

	card := widget.NewCard(
		"Quick Actions",
		"",
		container.NewVBox(exportBtn, importBtn, settingsBtn),
	)
	return card
}

func (a *App) createSystemInfo() fyne.CanvasObject {
	goVersion := widget.NewLabel(fmt.Sprintf("Go Version: %s", runtime.Version()))
	osInfo := widget.NewLabel(fmt.Sprintf("OS: %s", runtime.GOOS))
	cpuCores := widget.NewLabel(fmt.Sprintf("CPU Cores: %d", runtime.NumCPU()))

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memory := widget.NewLabel(fmt.Sprintf("Memory: %.2f MB", float64(memStats.Alloc)/1024/1024))

	card := widget.NewCard(
		"System Information",
		"",
		container.NewVBox(goVersion, osInfo, cpuCores, memory),
	)
	return card
}
