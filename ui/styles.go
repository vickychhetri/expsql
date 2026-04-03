package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type MyTheme struct{}

func (m MyTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {

	// Primary (modern blue)
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 37, G: 99, B: 235, A: 255} // #2563EB

	// Success (soft green)
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 34, G: 197, B: 94, A: 255} // #22C55E

	// Warning (amber)
	case theme.ColorNameWarning:
		return color.NRGBA{R: 245, G: 158, B: 11, A: 255} // #F59E0B

	// Error (soft red)
	case theme.ColorNameError:
		return color.NRGBA{R: 239, G: 68, B: 68, A: 255} // #EF4444

	// Optional: improve background (dark/light aware)
	case theme.ColorNameBackground:
		if variant == theme.VariantDark {
			return color.NRGBA{R: 17, G: 24, B: 39, A: 255} // dark gray
		}
		return color.NRGBA{R: 249, G: 250, B: 251, A: 255} // light gray

	// Optional: better button color
	case theme.ColorNameButton:
		return color.NRGBA{R: 59, G: 130, B: 246, A: 255} // #3B82F6

	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (m MyTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m MyTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m MyTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
