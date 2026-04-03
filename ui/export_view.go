package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *App) createExportView() fyne.CanvasObject {
	// Connection settings
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

	databaseEntry := widget.NewEntry()
	databaseEntry.SetPlaceHolder("database_name")

	outputDirEntry := widget.NewEntry()
	outputDirEntry.SetText(a.Settings.ExportDir)

	workersEntry := widget.NewEntry()
	workersEntry.SetText(fmt.Sprintf("%d", a.Settings.Workers))

	batchSizeEntry := widget.NewEntry()
	batchSizeEntry.SetText(fmt.Sprintf("%d", a.Settings.BatchSize))

	// Strategy selection
	strategySelect := widget.NewSelect(
		[]string{"auto", "parallel", "streaming", "standard"},
		func(value string) {
			a.Settings.Strategy = value
		},
	)
	strategySelect.SetSelected(a.Settings.Strategy)

	// Checkboxes
	compressCheck := widget.NewCheck("Compress output", func(checked bool) {
		a.Settings.Compress = checked
	})
	compressCheck.SetChecked(a.Settings.Compress)

	resumableCheck := widget.NewCheck("Enable resumable export", func(checked bool) {
		a.Settings.Resumable = checked
	})
	resumableCheck.SetChecked(a.Settings.Resumable)

	includeDataCheck := widget.NewCheck("Include data", func(checked bool) {
		a.Settings.IncludeData = checked
	})
	includeDataCheck.SetChecked(true)

	includeDesignCheck := widget.NewCheck("Include design (schema)", func(checked bool) {
		a.Settings.IncludeDesign = checked
	})
	includeDesignCheck.SetChecked(true)

	// Tables selection
	tablesEntry := widget.NewMultiLineEntry()
	tablesEntry.SetPlaceHolder("Comma-separated table names (leave empty for all tables)")
	tablesEntry.SetMinRowsVisible(3)

	excludeTablesEntry := widget.NewMultiLineEntry()
	excludeTablesEntry.SetPlaceHolder("Comma-separated tables to exclude")
	excludeTablesEntry.SetMinRowsVisible(3)

	// Log area
	logArea := widget.NewMultiLineEntry()
	logArea.SetMinRowsVisible(10)
	// logArea.Disable()

	// Progress bar
	progressBar := widget.NewProgressBarInfinite()
	progressBar.Stop()
	progressBar.Hide()

	// Time display labels
	startTimeLabel := widget.NewLabel("Start time: --")
	endTimeLabel := widget.NewLabel("End time: --")
	durationLabel := widget.NewLabel("Duration: --")
	rateLabel := widget.NewLabel("Rate: --")

	// Validate form inputs
	validateForm := func() bool {
		if hostEntry.Text == "" {
			dialog.ShowInformation("Validation Error", "Host cannot be empty", a.Window)
			return false
		}
		if portEntry.Text == "" {
			dialog.ShowInformation("Validation Error", "Port cannot be empty", a.Window)
			return false
		}
		if userEntry.Text == "" {
			dialog.ShowInformation("Validation Error", "User cannot be empty", a.Window)
			return false
		}
		if databaseEntry.Text == "" {
			dialog.ShowInformation("Validation Error", "Database name cannot be empty", a.Window)
			return false
		}
		if outputDirEntry.Text == "" {
			dialog.ShowInformation("Validation Error", "Output directory cannot be empty", a.Window)
			return false
		}
		return true
	}

	// Export button
	exportBtn := widget.NewButtonWithIcon("Start Export", theme.ConfirmIcon(), func() {
		if !validateForm() {
			return
		}

		startTime := time.Now()

		// ✅ UI-safe update
		fyne.Do(func() {
			startTimeLabel.SetText(fmt.Sprintf("Start time: %s", startTime.Format("2006-01-02 15:04:05")))
			endTimeLabel.SetText("End time: --")
			durationLabel.SetText("Duration: --")
			rateLabel.SetText("Rate: --")
		})

		go func() {
			err := a.runExportCommand(
				hostEntry.Text, portEntry.Text, userEntry.Text, passwordEntry.Text,
				databaseEntry.Text, outputDirEntry.Text, workersEntry.Text, batchSizeEntry.Text,
				strategySelect.Selected, compressCheck.Checked, resumableCheck.Checked,
				includeDataCheck.Checked, includeDesignCheck.Checked,
				tablesEntry.Text, excludeTablesEntry.Text,
				logArea, progressBar,
			)

			endTime := time.Now()
			duration := endTime.Sub(startTime)

			// ✅ UI-safe updates
			fyne.Do(func() {
				endTimeLabel.SetText(fmt.Sprintf("End time: %s", endTime.Format("2006-01-02 15:04:05")))
				durationLabel.SetText(fmt.Sprintf("Duration: %s", duration))

				logText := logArea.Text
				if strings.Contains(logText, "rows") {
					rateLabel.SetText("Rate: Check log for details")
				}
			})

			// ✅ Dialog must also be inside fyne.Do
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(fmt.Errorf("Export failed: %v", err), a.Window)
					a.updateStatus("Export failed")
				} else {
					dialog.ShowInformation("Success", fmt.Sprintf("Export completed in %s", duration), a.Window)
					a.updateStatus("Export completed successfully")
				}
			})
		}()
	})

	// Browse button for output directory
	browseBtn := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				outputDirEntry.SetText(uri.Path())
			}
		}, a.Window)
	})

	outputContainer := container.NewBorder(nil, nil, nil, browseBtn, outputDirEntry)

	// Time info card
	timeCard := widget.NewCard("Export Timing", "",
		container.NewVBox(
			startTimeLabel,
			endTimeLabel,
			durationLabel,
			rateLabel,
		),
	)

	// Options container
	// optionsContainer := container.NewGridWithColumns(2,
	// 	compressCheck,
	// 	resumableCheck,
	// 	includeDataCheck,
	// 	includeDesignCheck,
	// )

	// Form
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Host", Widget: hostEntry},
			{Text: "Port", Widget: portEntry},
			{Text: "User", Widget: userEntry},
			{Text: "Password", Widget: passwordEntry},
			{Text: "Database", Widget: databaseEntry},
			{Text: "Output Directory", Widget: outputContainer},
			{Text: "Workers", Widget: workersEntry},
			{Text: "Batch Size", Widget: batchSizeEntry},
			{Text: "Strategy", Widget: strategySelect},
		},
	}

	// // Main content
	// content := container.NewBorder(
	// 	nil,
	// 	container.NewVBox(
	// 		widget.NewSeparator(),
	// 		exportBtn,
	// 		progressBar,
	// 		timeCard,
	// 		widget.NewSeparator(),
	// 		widget.NewLabel("Log Output:"),
	// 		logArea,
	// 	),
	// 	nil,
	// 	nil,
	// 	container.NewVBox(
	// 		form,
	// 		widget.NewSeparator(),
	// 		widget.NewLabelWithStyle("Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	// 		optionsContainer,
	// 		widget.NewSeparator(),
	// 		widget.NewLabelWithStyle("Tables (Optional)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	// 		tablesEntry,
	// 		widget.NewLabel("Exclude Tables"),
	// 		excludeTablesEntry,
	// 	),
	// )

	// return container.NewScroll(content)

	// ===== LEFT PANEL (FORM + OPTIONS) =====
	tablesEntry.SetMinRowsVisible(2)
	excludeTablesEntry.SetMinRowsVisible(2)

	leftPanel := container.NewVBox(
		form,
		widget.NewSeparator(),
		widget.NewLabel("Options"),
		container.NewGridWithColumns(2,
			compressCheck,
			resumableCheck,
			includeDataCheck,
			includeDesignCheck,
		),
		widget.NewSeparator(),
		widget.NewLabel("Tables"),
		tablesEntry,
		excludeTablesEntry,
	)

	// ===== RIGHT PANEL (EXPORT + LOG) =====
	logArea.SetMinRowsVisible(12)

	topBar := container.NewGridWithColumns(2,
		exportBtn,
		progressBar,
	)

	rightPanel := container.NewVBox(
		topBar,
		timeCard,
		widget.NewSeparator(),
		widget.NewLabel("Logs"),
		container.NewMax(logArea), // prevents stretching
	)

	// ===== MAIN SPLIT =====
	main := container.NewHSplit(leftPanel, rightPanel)
	main.SetOffset(0.45)

	// ===== FINAL RETURN (NO SCROLL) =====
	return container.NewPadded(main)
}

func (a *App) runExportCommand(
	host, port, user, password, database, outputDir, workers, batchSize, strategy string,
	compress, resumable, includeData, includeDesign bool,
	tables, excludeTables string,
	logArea *widget.Entry, progressBar *widget.ProgressBarInfinite,
) error {

	ui := func(fn func()) {
		fyne.Do(fn)
	}

	// Build args
	args := []string{
		"export-parallel",
		"--host", host,
		"--port", port,
		"--user", user,
		"--password", password,
		"--database", database,
		"--output", outputDir,
		"--workers", workers,
		"--rows-per-batch", batchSize,
		"--strategy", strategy,
	}

	if compress {
		args = append(args, "--compress")
	}
	if resumable {
		args = append(args, "--resumable")
	}
	if !includeData {
		args = append(args, "--include-data=false")
	}
	if !includeDesign {
		args = append(args, "--include-design=false")
	}
	if tables != "" {
		args = append(args, "--tables", tables)
	}
	if excludeTables != "" {
		args = append(args, "--exclude-tables", excludeTables)
	}

	// UI init
	ui(func() {
		progressBar.Show()
		progressBar.Start()
		logArea.SetText("🚀 Starting export...\n")

	})

	// Binary path
	binaryPath := "./mysqltool"
	if _, err := os.Stat(binaryPath); err != nil {
		binaryPath = "mysqltool"
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = "."

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// -------- LOG HANDLER (SHARED BUFFER) --------
	appendLog := func(existing, incoming string) string {
		combined := existing + incoming
		if len(combined) > 10000 {
			combined = combined[len(combined)-10000:]
		}
		return combined
	}

	// -------- STDOUT --------
	go func() {
		buf := make([]byte, 4096)
		buffer := ""

		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				buffer += string(buf[:n])
			}

			select {
			case <-ticker.C:
				if buffer != "" {
					text := buffer
					buffer = ""

					ui(func() {
						logArea.SetText(appendLog(logArea.Text, text))
						// move cursor to end (forces scroll)
						logArea.CursorRow = strings.Count(logArea.Text, "\n")
						logArea.CursorColumn = 0
						logArea.Refresh()
						// if strings.Contains(text, "Exported") || strings.Contains(text, "rows") {
						// 	if progressBar.Value < 0.95 {
						// 		progressBar.SetValue(progressBar.Value + 0.01)
						// 	}
						// }
					})
				}
			default:
			}

			if err != nil {
				break
			}
		}
	}()

	// -------- STDERR --------
	go func() {
		buf := make([]byte, 4096)
		buffer := ""

		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()

		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				buffer += string(buf[:n])
			}

			select {
			case <-ticker.C:
				if buffer != "" {
					text := buffer
					buffer = ""

					ui(func() {
						logArea.SetText(appendLog(logArea.Text, text))

						logArea.CursorRow = strings.Count(logArea.Text, "\n")
						logArea.CursorColumn = 0
						logArea.Refresh()

					})
				}
			default:
			}

			if err != nil {
				break
			}
		}
	}()

	// Wait for completion
	err = cmd.Wait()

	// Final UI update
	ui(func() {
		progressBar.Stop()
		progressBar.Hide()

		if err != nil {
			logArea.SetText(appendLog(logArea.Text,
				fmt.Sprintf("\n❌ Export failed: %v\n", err),
			))
			logArea.CursorRow = strings.Count(logArea.Text, "\n")
			logArea.CursorColumn = 0
			logArea.Refresh()
			a.updateStatus("Export failed")
		} else {
			// progressBar.SetValue(1.0)
			logArea.SetText(appendLog(logArea.Text,
				"\n✅ Export completed successfully!\n",
			))
			logArea.CursorRow = strings.Count(logArea.Text, "\n")
			logArea.CursorColumn = 0
			logArea.Refresh()

			a.updateStatus("Export completed")
		}
	})

	return err
}
