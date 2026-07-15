package fyneui

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"printer-installer/internal/i18n"
)

//go:embed icon.png
var iconPng []byte

type Result struct {
	Location    string
	Overwrite   bool
	DeleteNames []string
	Cancelled   bool
}

func Run(detectedLoc string, allLocations []string, deletePrinters []string, printersIPs map[string]string, locIPs map[string][]string) *Result {
	a := app.New()
	w := a.NewWindow(i18n.T("WINDOW_TITLE"))
	w.SetIcon(fyne.NewStaticResource("printer", iconPng))

	var result *Result

	otherLocs := make([]string, 0)
	for _, l := range allLocations {
		if l != detectedLoc {
			otherLocs = append(otherLocs, l)
		}
	}

	// Summary label
	summaryText := detectedLoc + "  |  " + i18n.T("LOCATION_PREFIX", "") + "  |  IP: TODO"
	summaryLabel := widget.NewLabel(summaryText)
	summaryLabel.Alignment = fyne.TextAlignCenter

	// Section 1: Confirm
	confirmCheck := widget.NewCheck(i18n.T("CONFIRM_FMT", detectedLoc), func(on bool) {})
	confirmCheck.SetChecked(detectedLoc != "")
	if detectedLoc == "" {
		confirmCheck.Hide()
	}

	locSelect := widget.NewSelect(otherLocs, func(s string) {})
	if detectedLoc != "" {
		locSelect.Hide()
	} else if len(otherLocs) > 0 {
		locSelect.SetSelected(otherLocs[0])
	} else {
		locSelect.PlaceHolder = i18n.T("NO_LOCATION")
	}

	// Section 2: Conflict
	skipT := i18n.T("SKIP_BTN")
	overwriteT := i18n.T("OVERWRITE_LABEL")
	conflictSelect := widget.NewSelect([]string{skipT, overwriteT}, func(s string) {})
	conflictSelect.SetSelected(skipT)

	conflictLabel := widget.NewLabel(i18n.T("CONFLICT_LABEL"))

	// Section 3: Delete list
	installedIPs := make(map[string]bool, len(printersIPs))
	for _, ip := range printersIPs {
		installedIPs[ip] = true
	}

	delChecks := make([]*widget.Check, 0)
	delList := container.NewVBox()
	for _, p := range deletePrinters {
		cb := widget.NewCheck(p, func(bool) {})
		delChecks = append(delChecks, cb)
		delList.Add(cb)
	}
	delHeader := widget.NewLabel(i18n.T("EXISTING_PRINTERS", len(deletePrinters)))

	updateDisabled := func(loc string) {
		ips := locIPs[loc]
		ipSet := make(map[string]bool, len(ips))
		for _, ip := range ips {
			ipSet[ip] = true
		}
		for _, cb := range delChecks {
			if ipSet[printersIPs[cb.Text]] {
				cb.Disable()
				cb.SetChecked(false)
			} else {
				cb.Enable()
			}
		}
		hasConflict := false
		for _, ip := range ips {
			if installedIPs[ip] {
				hasConflict = true
				break
			}
		}
		if hasConflict {
			conflictSelect.Enable()
		} else {
			conflictSelect.Disable()
		}
	}

	if detectedLoc != "" {
		updateDisabled(detectedLoc)
	} else if len(otherLocs) > 0 {
		updateDisabled(otherLocs[0])
	}

	confirmCheck.OnChanged = func(on bool) {
		if on {
			locSelect.Hide()
			updateDisabled(detectedLoc)
		} else {
			locSelect.Show()
			updateDisabled(locSelect.Selected)
		}
	}

	locSelect.OnChanged = func(s string) {
		updateDisabled(s)
	}

	installBtn := widget.NewButton(i18n.T("OK_LABEL"), func() {
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
			Overwrite:   conflictSelect.Selected == overwriteT,
			DeleteNames: delNames,
		}
		w.Close()
	})

	cancelBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		result = &Result{Cancelled: true}
		w.Close()
	})

	// Build layout matching macOS JXA structure
	top := container.NewVBox(
		container.NewPadded(summaryLabel),
		widget.NewSeparator(),
		confirmCheck,
		container.NewPadded(locSelect),
		widget.NewSeparator(),
		conflictLabel,
		container.NewPadded(conflictSelect),
		widget.NewSeparator(),
		delHeader,
	)

	btnBox := container.NewHBox(
		cancelBtn,
		widget.NewSeparator(),
		installBtn,
	)

	content := container.NewBorder(
		top, btnBox, nil, nil,
		container.NewScroll(delList),
	)

	w.SetContent(container.NewPadded(content))
	w.Resize(fyne.NewSize(500, 480))
	w.SetFixedSize(true)
	w.CenterOnScreen()
	w.ShowAndRun()

	return result
}
