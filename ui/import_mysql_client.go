package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// -------------------- UI SCREEN --------------------

func (a *App) createMySQLClientImportView() fyne.CanvasObject {

	host := widget.NewEntry()
	host.SetText(a.Settings.Host)

	port := widget.NewEntry()
	port.SetText(fmt.Sprintf("%d", a.Settings.Port))

	user := widget.NewEntry()
	user.SetText(a.Settings.User)

	pass := widget.NewPasswordEntry()
	pass.SetText(a.Settings.Password)

	db := widget.NewEntry()
	db.SetPlaceHolder("database_name")

	dir := widget.NewEntry()
	dir.SetPlaceHolder("/path/to/sql/files")

	workersEntry := widget.NewEntry()
	workersEntry.SetText("2")

	// --- Logs ---
	logArea := widget.NewMultiLineEntry()
	logArea.SetMinRowsVisible(20)
	logArea.Wrapping = fyne.TextWrapWord

	appendLog := func(msg string) {
		fyne.Do(func() {
			logArea.SetText(logArea.Text + msg + "\n")
			logArea.CursorRow++
		})
	}

	progress := widget.NewProgressBarInfinite()
	progress.Hide()

	runBtn := widget.NewButtonWithIcon("Import via MySQL Client", theme.MediaPlayIcon(), nil)

	runBtn.OnTapped = func() {

		if db.Text == "" || dir.Text == "" {
			appendLog("❌ Database & folder required")
			return
		}

		runBtn.Disable()
		progress.Show()
		appendLog("🚀 Starting structured import...")

		go func() {
			defer func() {
				fyne.Do(func() {
					runBtn.Enable()
					progress.Hide()
				})
			}()

			err := a.runMySQLFolderImport(
				host.Text,
				port.Text,
				user.Text,
				pass.Text,
				db.Text,
				dir.Text,
				workersEntry.Text,
				appendLog,
			)

			if err != nil {
				appendLog("❌ Failed: " + err.Error())
				return
			}

			appendLog("✅ Import completed")
		}()
	}

	clearBtn := widget.NewButtonWithIcon("Clear Logs", theme.DeleteIcon(), func() {
		logArea.SetText("")
	})

	form := widget.NewForm(
		widget.NewFormItem("Host", host),
		widget.NewFormItem("Port", port),
		widget.NewFormItem("User", user),
		widget.NewFormItem("Password", pass),
		widget.NewFormItem("Database", db),
		widget.NewFormItem("Dump Folder", dir),
		widget.NewFormItem("Workers", workersEntry),
	)

	left := container.NewVBox(
		widget.NewCard("MySQL Client Import", "", form),
		container.NewHBox(runBtn, clearBtn),
		progress,
	)

	right := widget.NewCard("Logs", "", container.NewScroll(logArea))

	split := container.NewHSplit(left, right)
	split.Offset = 0.35

	return container.NewPadded(split)
}

// -------------------- CORE LOGIC --------------------

func (a *App) runMySQLFolderImport(
	host, port, user, password, database, folder, workers string,
	logFn func(string),
) error {

	// ---- STEP 1: DESIGN FILES (STRICT ORDER) ----
	designFiles := []string{
		"design_tables.sql",
		"design_views.sql",
		"design_functions.sql",
		"design_procedures.sql",
		"design_triggers.sql",
		"design_events.sql",
	}

	logFn("📐 Importing DESIGN files (sequential)...")

	for _, fileName := range designFiles {
		fullPath := filepath.Join(folder, fileName)

		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			logFn(fmt.Sprintf("⚠️ Skipped (not found): %s", fileName))
			continue
		}

		logFn("▶ " + fileName)

		err := runSingleMySQLImport(host, port, user, password, database, fullPath)
		if err != nil {
			return fmt.Errorf("design import failed (%s): %v", fileName, err)
		}

		logFn("✅ Done: " + fileName)
	}

	// ---- STEP 2: DATA FILES (PARALLEL) ----
	files, err := filepath.Glob(filepath.Join(folder, "*.sql"))
	if err != nil {
		return err
	}

	// remove design files from list
	filtered := make([]string, 0)
	designMap := make(map[string]bool)
	for _, d := range designFiles {
		designMap[d] = true
	}

	for _, f := range files {
		if !designMap[filepath.Base(f)] {
			filtered = append(filtered, f)
		}
	}

	if len(filtered) == 0 {
		logFn("ℹ️ No data files found")
		return nil
	}

	sort.Strings(filtered)

	w, err := strconv.Atoi(workers)
	if err != nil || w <= 0 {
		w = 1
	}

	logFn(fmt.Sprintf("📦 Importing DATA files: %d | Workers: %d", len(filtered), w))

	fileChan := make(chan string, len(filtered))
	var wg sync.WaitGroup

	for i := 0; i < w; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for file := range fileChan {
				name := filepath.Base(file)
				logFn(fmt.Sprintf("[Worker %d] ▶ %s", id, name))

				err := runSingleMySQLImport(host, port, user, password, database, file)
				if err != nil {
					logFn(fmt.Sprintf("❌ %s failed: %v", name, err))
					continue
				}

				logFn(fmt.Sprintf("✅ Done: %s", name))
			}
		}(i + 1)
	}

	for _, f := range filtered {
		fileChan <- f
	}
	close(fileChan)

	wg.Wait()

	return nil
}

// -------------------- SINGLE FILE IMPORT --------------------

func runSingleMySQLImport(
	host, port, user, password, database, file string,
) error {

	cmd := exec.Command(
		"mysql",
		"--max_allowed_packet=1G",
		"-h", host,
		"-P", port,
		"-u", user,
		fmt.Sprintf("-p%s", password),
		database,
	)

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd.Stdin = f

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v | output: %s", err, string(out))
	}

	return nil
}
