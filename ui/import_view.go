package ui

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *App) createImportView() fyne.CanvasObject {
	// --- Inputs ---
	hostEntry := widget.NewEntry()
	hostEntry.SetText(a.Settings.Host)

	portEntry := widget.NewEntry()
	portEntry.SetText(fmt.Sprintf("%d", a.Settings.Port))

	userEntry := widget.NewEntry()
	userEntry.SetText(a.Settings.User)

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetText(a.Settings.Password)

	databaseEntry := widget.NewEntry()
	databaseEntry.SetPlaceHolder("database_name")

	inputDirEntry := widget.NewEntry()
	inputDirEntry.SetText(a.Settings.ExportDir)

	workersEntry := widget.NewEntry()
	workersEntry.SetText("1")

	// --- Log Area (Optimized) ---
	logArea := widget.NewMultiLineEntry()
	logArea.SetMinRowsVisible(20)
	logArea.Wrapping = fyne.TextWrapWord

	const maxLogLines = 200
	logChan := make(chan string, 1000)
	var logs []string

	// UI updater (throttled)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			var updated bool
		loop:
			for i := 0; i < 100; i++ {
				select {
				case msg := <-logChan:
					logs = append(logs, msg)
					if len(logs) > maxLogLines {
						logs = logs[len(logs)-maxLogLines:]
					}
					updated = true
				default:
					break loop
				}
			}
			if updated {
				text := strings.Join(logs, "\n")
				fyne.Do(func() {
					logArea.SetText(text)
					logArea.CursorRow = len(logs)
					logArea.Refresh()
				})
			}
		}
	}()

	appendLog := func(msg string) {
		select {
		case logChan <- msg:
		default:
			// drop if full
		}
	}

	// --- Progress Bar ---
	progress := widget.NewProgressBarInfinite()
	progress.Hide()

	// --- Buttons ---
	importBtn := widget.NewButtonWithIcon("Start Import", theme.ConfirmIcon(), nil)
	clearBtn := widget.NewButtonWithIcon("Clear Logs", theme.DeleteIcon(), func() {
		logs = nil
		logArea.SetText("")
	})

	importBtn.OnTapped = func() {
		workers := strings.TrimSpace(workersEntry.Text)
		if workers == "" || workers == "0" {
			appendLog("❌ Workers must be >= 1")
			return
		}

		importBtn.Disable()
		progress.Show()
		appendLog("🚀 Starting import...")

		go func() {
			defer func() {
				fyne.Do(func() {
					importBtn.Enable()
					progress.Hide()
				})
			}()

			cmd := fmt.Sprintf(
				"./mysqltool import --host %s --port %s --user %s --password %s --database %s --input %s --workers %s",
				hostEntry.Text,
				portEntry.Text,
				userEntry.Text,
				passwordEntry.Text,
				databaseEntry.Text,
				inputDirEntry.Text,
				workers,
			)

			appendLog("CMD: " + cmd)

			if err := a.runCommand(cmd, appendLog); err != nil {
				appendLog("❌ Import failed: " + err.Error())
				return
			}

			appendLog("✅ Import completed successfully")
		}()
	}

	// --- Form ---
	form := widget.NewForm(
		widget.NewFormItem("Host", hostEntry),
		widget.NewFormItem("Port", portEntry),
		widget.NewFormItem("User", userEntry),
		widget.NewFormItem("Password", passwordEntry),
		widget.NewFormItem("Database", databaseEntry),
		widget.NewFormItem("Input Directory", inputDirEntry),
		widget.NewFormItem("Workers", workersEntry),
	)

	formCard := widget.NewCard("MySQL Import Configuration", "", form)
	logCard := widget.NewCard("Live Logs", "", container.NewScroll(logArea))

	// --- Layout ---
	formContainer := container.NewVBox(
		formCard,
		container.NewHBox(importBtn, clearBtn),
		progress,
	)

	split := container.NewHSplit(
		container.NewPadded(formContainer),
		container.NewPadded(logCard),
	)
	split.Offset = 0.35 // 35% left, 65% right

	return container.NewPadded(split)
}

// --- Command runner ---
func (a *App) runCommand(command string, logFn func(string)) error {
	cmd := exec.Command("bash", "-c", command)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	go streamOutput(stdout, logFn)
	go streamOutput(stderr, logFn)

	return cmd.Wait()
}

// --- Stream output with large buffer ---
func streamOutput(r io.Reader, logFn func(string)) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 1024*1024) // 1MB buffer
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		logFn(scanner.Text())
	}
}
