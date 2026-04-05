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
	Host           string `json:"host"`
	Port           int    `json:"port"`
	User           string `json:"user"`
	Password       string `json:"password"`
	ExportDir      string `json:"export_dir"`
	Workers        int    `json:"workers"`
	BatchSize      int    `json:"batch_size"`
	Strategy       string `json:"strategy"`
	Compress       bool   `json:"compress"`
	Resumable      bool   `json:"resumable"`
	IncludeData    bool   `json:"include_data"`
	IncludeDesign  bool   `json:"include_design"`
	BulkInsertSize int    `json:"bulk_insert_size"`
}

func LoadSettings() *Settings {
	settings := &Settings{
		Host:           "localhost",
		Port:           3306,
		User:           "root",
		Password:       "",
		ExportDir:      "./export",
		Workers:        4,
		BatchSize:      10000,
		Strategy:       "auto",
		Compress:       false,
		Resumable:      false,
		IncludeData:    true,
		IncludeDesign:  true,
		BulkInsertSize: 1000,
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

	makeEntry := func(text, placeholder string) *widget.Entry {
		e := widget.NewEntry()
		e.SetText(text)
		e.SetPlaceHolder(placeholder)
		return e
	}

	// Inputs
	hostEntry := makeEntry(a.Settings.Host, "localhost")
	portEntry := makeEntry(fmt.Sprintf("%d", a.Settings.Port), "3306")
	userEntry := makeEntry(a.Settings.User, "root")
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetText(a.Settings.Password)

	exportDirEntry := makeEntry(a.Settings.ExportDir, "./export")
	workersEntry := makeEntry(fmt.Sprintf("%d", a.Settings.Workers), "4")
	batchSizeEntry := makeEntry(fmt.Sprintf("%d", a.Settings.BatchSize), "10000")
	bulkInsertSizeEntry := makeEntry(fmt.Sprintf("%d", a.Settings.BulkInsertSize), "1000")

	strategySelect := widget.NewSelect(
		[]string{"auto", "parallel", "streaming", "standard"},
		func(value string) {},
	)
	strategySelect.SetSelected(a.Settings.Strategy)

	// Browse
	browseBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				exportDirEntry.SetText(uri.Path())
			}
		}, a.Window)
	})
	exportDir := container.NewBorder(nil, nil, nil, browseBtn, exportDirEntry)

	// Checkboxes (compact grid)
	compressCheck := widget.NewCheck("Compress", nil)
	compressCheck.SetChecked(a.Settings.Compress)

	resumableCheck := widget.NewCheck("Resumable", nil)
	resumableCheck.SetChecked(a.Settings.Resumable)

	includeDataCheck := widget.NewCheck("Include Data", nil)
	includeDataCheck.SetChecked(a.Settings.IncludeData)

	includeDesignCheck := widget.NewCheck("Include Design", nil)
	includeDesignCheck.SetChecked(a.Settings.IncludeDesign)

	optionsGrid := container.NewGridWithColumns(2,
		compressCheck,
		resumableCheck,
		includeDataCheck,
		includeDesignCheck,
	)

	// Sections
	connectionCard := widget.NewCard("Connection", "", container.NewGridWithColumns(2,
		hostEntry, portEntry,
		userEntry, passwordEntry,
	))

	exportCard := widget.NewCard("Export Defaults", "", container.NewGridWithColumns(2,
		exportDir,
		strategySelect,
		workersEntry,
		batchSizeEntry,
		bulkInsertSizeEntry,
	))

	optionsCard := widget.NewCard("Options", "", optionsGrid)

	// Buttons
	saveBtn := widget.NewButtonWithIcon("Save", theme.ConfirmIcon(), func() {
		a.Settings.Host = hostEntry.Text
		fmt.Sscanf(portEntry.Text, "%d", &a.Settings.Port)
		a.Settings.User = userEntry.Text
		a.Settings.Password = passwordEntry.Text
		a.Settings.ExportDir = exportDirEntry.Text
		fmt.Sscanf(workersEntry.Text, "%d", &a.Settings.Workers)
		fmt.Sscanf(batchSizeEntry.Text, "%d", &a.Settings.BatchSize)
		fmt.Sscanf(bulkInsertSizeEntry.Text, "%d", &a.Settings.BulkInsertSize)
		a.Settings.Strategy = strategySelect.Selected
		a.Settings.Compress = compressCheck.Checked
		a.Settings.Resumable = resumableCheck.Checked
		a.Settings.IncludeData = includeDataCheck.Checked
		a.Settings.IncludeDesign = includeDesignCheck.Checked

		if err := a.Settings.Save(); err != nil {
			dialog.ShowError(err, a.Window)
			return
		}
		dialog.ShowInformation("Saved", "Settings updated", a.Window)
	})

	resetBtn := widget.NewButtonWithIcon("Reset", theme.CancelIcon(), func() {
		def := LoadSettings()
		hostEntry.SetText(def.Host)
		portEntry.SetText(fmt.Sprintf("%d", def.Port))
		userEntry.SetText(def.User)
		passwordEntry.SetText(def.Password)
		exportDirEntry.SetText(def.ExportDir)
		workersEntry.SetText(fmt.Sprintf("%d", def.Workers))
		batchSizeEntry.SetText(fmt.Sprintf("%d", def.BatchSize))
		bulkInsertSizeEntry.SetText(fmt.Sprintf("%d", def.BulkInsertSize))
		strategySelect.SetSelected(def.Strategy)
		compressCheck.SetChecked(def.Compress)
		resumableCheck.SetChecked(def.Resumable)
		includeDataCheck.SetChecked(def.IncludeData)
		includeDesignCheck.SetChecked(def.IncludeDesign)
	})

	buttons := container.NewGridWithColumns(2, saveBtn, resetBtn)

	// Final layout (WIDTH FIX)
	content := container.NewVBox(
		widget.NewLabelWithStyle("Settings", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		connectionCard,
		exportCard,
		optionsCard,
		buttons,
	)

	wrapped := container.NewCenter(
		container.NewGridWrap(
			fyne.NewSize(500, 650), // 👈 critical width fix
			container.NewPadded(content),
		),
	)

	return container.NewVScroll(wrapped)
}
