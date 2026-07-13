package fyneui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Result struct {
	Location    string
	Overwrite   bool
	DeleteNames []string
	Cancelled   bool
}

func Run(detectedLoc string, allLocations []string, deletePrinters []string) *Result {
	a := app.New()
	w := a.NewWindow("Printer Installer")

	var result *Result

	otherLocs := make([]string, 0)
	for _, l := range allLocations {
		if l != detectedLoc {
			otherLocs = append(otherLocs, l)
		}
	}

	confirmCheck := widget.NewCheck("Location: "+detectedLoc, func(on bool) {
	})
	confirmCheck.SetChecked(detectedLoc != "")
	if detectedLoc == "" {
		confirmCheck.Hide()
	}

	locSelect := widget.NewSelect(otherLocs, func(s string) {})
	if detectedLoc != "" {
		locSelect.Hide()
	} else if len(otherLocs) > 0 {
		locSelect.SetSelected(otherLocs[0])
		locSelect.Show()
	}

	confirmCheck.OnChanged = func(on bool) {
		if on {
			locSelect.Hide()
		} else {
			locSelect.Show()
		}
	}

	conflictSelect := widget.NewSelect([]string{"Skip", "Overwrite"}, func(s string) {})
	conflictSelect.SetSelected("Skip")

	delChecks := make([]*widget.Check, 0)
	delList := container.NewVBox()
	for _, p := range deletePrinters {
		cb := widget.NewCheck(p, func(bool) {})
		delChecks = append(delChecks, cb)
		delList.Add(cb)
	}

	installBtn := widget.NewButtonWithIcon("Install", theme.ConfirmIcon(), func() {
		loc := detectedLoc
		if locSelect.Visible() {
			loc = locSelect.Selected
		}
		delNames := make([]string, 0)
		for _, cb := range delChecks {
			if cb.Checked {
				delNames = append(delNames, cb.Text)
			}
		}
		result = &Result{
			Location:    loc,
			Overwrite:   conflictSelect.Selected == "Overwrite",
			DeleteNames: delNames,
		}
		w.Close()
	})

	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		result = &Result{Cancelled: true}
		w.Close()
	})

	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Printer Driver Installer", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
		),
		container.NewHBox(container.NewCenter(), installBtn, cancelBtn),
		nil, nil,
		container.NewVBox(
			confirmCheck,
			locSelect,
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Conflict:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			conflictSelect,
			widget.NewSeparator(),
			widget.NewLabelWithStyle(fmt.Sprintf("Existing printers (%d):", len(deletePrinters)), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewScroll(delList),
		),
	)

	w.SetContent(container.NewPadded(content))
	w.Resize(fyne.NewSize(440, 400))
	w.SetFixedSize(true)
	w.CenterOnScreen()
	w.ShowAndRun()

	return result
}
