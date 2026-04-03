package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *App) createImportView() fyne.CanvasObject {
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

	importBtn := widget.NewButtonWithIcon("Start Import", theme.ConfirmIcon(), func() {
		msg := fmt.Sprintf("Importing to database '%s'", databaseEntry.Text)
		dialog.ShowInformation("Import Started", msg, a.Window)
		a.updateStatus("Import started...")
	})

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Host", Widget: hostEntry},
			{Text: "Port", Widget: portEntry},
			{Text: "User", Widget: userEntry},
			{Text: "Password", Widget: passwordEntry},
			{Text: "Database", Widget: databaseEntry},
			{Text: "Input Directory", Widget: inputDirEntry},
		},
	}

	content := container.NewBorder(
		nil,
		importBtn,
		nil,
		nil,
		form,
	)

	return container.NewScroll(content)
}
