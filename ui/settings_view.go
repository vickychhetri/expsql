package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Settings struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	Password      string `json:"password"`
	ExportDir     string `json:"export_dir"`
	Workers       int    `json:"workers"`
	BatchSize     int    `json:"batch_size"`
	Strategy      string `json:"strategy"`
	Compress      bool   `json:"compress"`
	Resumable     bool   `json:"resumable"`
	IncludeData   bool   `json:"include_data"`
	IncludeDesign bool   `json:"include_design"`
}

func LoadSettings() *Settings {
	settings := &Settings{
		Host:          "localhost",
		Port:          3306,
		User:          "root",
		Password:      "",
		ExportDir:     "./export",
		Workers:       4,
		BatchSize:     10000,
		Strategy:      "auto",
		Compress:      false,
		Resumable:     false,
		IncludeData:   true,
		IncludeDesign: true,
	}

	// Try to load from file
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".mysqltool.json")

	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, settings)
	}

	return settings
}

func (s *Settings) Save() error {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".mysqltool.json")

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func (a *App) createSettingsView() fyne.CanvasObject {
	// General settings
	hostEntry := widget.NewEntry()
	hostEntry.SetText(a.Settings.Host)
	hostEntry.SetPlaceHolder("localhost")

	portEntry := widget.NewEntry()
	portEntry.SetText(fmt.Sprintf("%d", a.Settings.Port))
	portEntry.SetPlaceHolder("3306")

	userEntry := widget.NewEntry()
	userEntry.SetText(a.Settings.User)
	userEntry.SetPlaceHolder("root")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetText(a.Settings.Password)
	passwordEntry.SetPlaceHolder("Password")

	// Default export settings
	exportDirEntry := widget.NewEntry()
	exportDirEntry.SetText(a.Settings.ExportDir)
	exportDirEntry.SetPlaceHolder("./export")

	workersEntry := widget.NewEntry()
	workersEntry.SetText(fmt.Sprintf("%d", a.Settings.Workers))
	workersEntry.SetPlaceHolder("4")

	batchSizeEntry := widget.NewEntry()
	batchSizeEntry.SetText(fmt.Sprintf("%d", a.Settings.BatchSize))
	batchSizeEntry.SetPlaceHolder("10000")

	strategySelect := widget.NewSelect(
		[]string{"auto", "parallel", "streaming", "standard"},
		func(value string) {
			a.Settings.Strategy = value
		},
	)
	strategySelect.SetSelected(a.Settings.Strategy)

	// Checkboxes
	compressCheck := widget.NewCheck("Compress by default", func(checked bool) {
		a.Settings.Compress = checked
	})
	compressCheck.SetChecked(a.Settings.Compress)

	resumableCheck := widget.NewCheck("Resumable by default", func(checked bool) {
		a.Settings.Resumable = checked
	})
	resumableCheck.SetChecked(a.Settings.Resumable)

	includeDataCheck := widget.NewCheck("Include data by default", func(checked bool) {
		a.Settings.IncludeData = checked
	})
	includeDataCheck.SetChecked(a.Settings.IncludeData)

	includeDesignCheck := widget.NewCheck("Include design by default", func(checked bool) {
		a.Settings.IncludeDesign = checked
	})
	includeDesignCheck.SetChecked(a.Settings.IncludeDesign)

	// Browse button for export directory
	browseBtn := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				exportDirEntry.SetText(uri.Path())
			}
		}, a.Window)
	})

	exportDirContainer := container.NewBorder(nil, nil, nil, browseBtn, exportDirEntry)

	// Save button
	saveBtn := widget.NewButtonWithIcon("Save Settings", theme.ConfirmIcon(), func() {
		// Update settings
		a.Settings.Host = hostEntry.Text
		fmt.Sscanf(portEntry.Text, "%d", &a.Settings.Port)
		a.Settings.User = userEntry.Text
		a.Settings.Password = passwordEntry.Text
		a.Settings.ExportDir = exportDirEntry.Text
		fmt.Sscanf(workersEntry.Text, "%d", &a.Settings.Workers)
		fmt.Sscanf(batchSizeEntry.Text, "%d", &a.Settings.BatchSize)
		a.Settings.Strategy = strategySelect.Selected
		a.Settings.Compress = compressCheck.Checked
		a.Settings.Resumable = resumableCheck.Checked
		a.Settings.IncludeData = includeDataCheck.Checked
		a.Settings.IncludeDesign = includeDesignCheck.Checked

		if err := a.Settings.Save(); err != nil {
			dialog.ShowError(err, a.Window)
		} else {
			dialog.ShowInformation("Success", "Settings saved successfully!", a.Window)
			a.updateStatus("Settings saved")
		}
	})

	// Reset button
	resetBtn := widget.NewButtonWithIcon("Reset to Defaults", theme.CancelIcon(), func() {
		defaultSettings := &Settings{}
		hostEntry.SetText(defaultSettings.Host)
		portEntry.SetText(fmt.Sprintf("%d", defaultSettings.Port))
		userEntry.SetText(defaultSettings.User)
		passwordEntry.SetText(defaultSettings.Password)
		exportDirEntry.SetText(defaultSettings.ExportDir)
		workersEntry.SetText(fmt.Sprintf("%d", defaultSettings.Workers))
		batchSizeEntry.SetText(fmt.Sprintf("%d", defaultSettings.BatchSize))
		strategySelect.SetSelected(defaultSettings.Strategy)
		compressCheck.SetChecked(defaultSettings.Compress)
		resumableCheck.SetChecked(defaultSettings.Resumable)
		includeDataCheck.SetChecked(defaultSettings.IncludeData)
		includeDesignCheck.SetChecked(defaultSettings.IncludeDesign)
	})

	// Form
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Default Host", Widget: hostEntry, HintText: "MySQL server hostname"},
			{Text: "Default Port", Widget: portEntry, HintText: "MySQL server port"},
			{Text: "Default User", Widget: userEntry, HintText: "Database username"},
			{Text: "Default Password", Widget: passwordEntry, HintText: "Database password"},
			{Text: "Default Export Directory", Widget: exportDirContainer, HintText: "Directory for exports"},
			{Text: "Default Workers", Widget: workersEntry, HintText: "Number of workers (1-32)"},
			{Text: "Default Batch Size", Widget: batchSizeEntry, HintText: "Rows per batch (1000-100000)"},
			{Text: "Default Strategy", Widget: strategySelect, HintText: "Export strategy"},
		},
	}

	buttons := container.NewGridWithColumns(2, saveBtn, resetBtn)

	content := container.NewVBox(
		widget.NewLabelWithStyle("Settings", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		form,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Default Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		compressCheck,
		resumableCheck,
		includeDataCheck,
		includeDesignCheck,
		widget.NewSeparator(),
		buttons,
	)

	return container.NewScroll(content)
}
