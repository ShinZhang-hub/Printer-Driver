package fyneui

import (
	_ "embed"

	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
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

// WorkFunc performs the delete/install work after the user confirms. It runs in
// the background while the status window is shown, and must return the result
// text to display (empty means nothing to show).
type WorkFunc func(*Result) string

func Run(detectedLoc string, allLocations []string, deletePrinters []string, printersIPs map[string]string, locIPs map[string][]string, locNames map[string][]string, work WorkFunc) (*Result, string) {
	a := app.New()
	applyCJKTheme(a)
	w := a.NewWindow(i18n.T("WINDOW_TITLE"))
	w.SetIcon(fyne.NewStaticResource("printer", iconPng))

	var result *Result
	messageCh := make(chan string, 1)

	otherLocs := make([]string, 0)
	for _, l := range allLocations {
		if l != detectedLoc {
			otherLocs = append(otherLocs, l)
		}
	}

	// Summary label
	summaryLabel := widget.NewLabel("")
	summaryLabel.Alignment = fyne.TextAlignCenter
	updateSummary := func(loc string) {
		ips := locIPs[loc]
		ipText := "IP: -"
		if len(ips) > 0 {
			ipText = "IP: " + strings.Join(ips, ", ")
		}
		locTxt := i18n.T("NO_LOCATION")
		if loc != "" {
			locTxt = loc
		}
		namesTxt := ""
		if names := locNames[loc]; len(names) > 0 {
			namesTxt = strings.Join(names, ", ")
		}
		segments := []string{locTxt}
		if namesTxt != "" {
			segments = append(segments, namesTxt)
		}
		segments = append(segments, ipText)
		summaryLabel.SetText(strings.Join(segments, "  |  "))
	}
	updateSummary(detectedLoc)

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
			updateSummary(detectedLoc)
		} else {
			locSelect.Show()
			updateDisabled(locSelect.Selected)
			updateSummary(locSelect.Selected)
		}
	}

	locSelect.OnChanged = func(s string) {
		updateDisabled(s)
		updateSummary(s)
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

		// Swap to a small status window while the work runs in the background.
		w.SetCloseIntercept(func() {})
		w.SetFixedSize(false)
		w.Resize(fyne.NewSize(360, 170))
		w.SetFixedSize(true)
		w.CenterOnScreen()
		statusLabel := widget.NewLabelWithStyle(i18n.T("INSTALLING"), fyne.TextAlignCenter, fyne.TextStyle{})
		statusBar := widget.NewProgressBarInfinite()
		w.SetContent(container.NewPadded(container.NewCenter(container.NewVBox(
			statusLabel,
			container.NewPadded(statusBar),
		))))
		statusBar.Start()

		go func() {
			msg := ""
			func() {
				defer func() {
					if r := recover(); r != nil {
						msg = fmt.Sprintf("installation error: %v", r)
					}
				}()
				if work != nil {
					msg = work(result)
				}
			}()
			messageCh <- msg
			fyne.Do(func() {
				statusBar.Stop()
				w.Close()
			})
		}()
	})

	cancelBtn := widget.NewButton(i18n.T("CANCEL_LABEL"), func() {
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

	btnBox := container.NewCenter(
		container.NewHBox(cancelBtn, installBtn),
	)

	content := container.NewBorder(
		top, container.NewPadded(btnBox), nil, nil,
		container.NewScroll(delList),
	)

	w.SetContent(container.NewPadded(content))

	// Size the window to fit the content exactly so the delete list only
	// scrolls once the window reaches its maximum height. Widgets are measured
	// with the active theme, so the taller CJK fonts used on Japanese/Chinese
	// systems are accounted for automatically (Yu Gothic lines are ~30% taller
	// than Inter, which previously pushed 3 rows into the scroll area).
	rowH := float32(38)
	if len(delChecks) > 0 {
		rowH = delChecks[0].MinSize().Height
	}
	const pad = float32(8) // each NewPadded adds theme padding on both sides
	const maxH = float32(680)
	height := pad + top.MinSize().Height + btnBox.MinSize().Height + pad + float32(len(delChecks))*rowH + 4
	if height > maxH {
		height = maxH
	}
	w.Resize(fyne.NewSize(520, height))
	w.SetFixedSize(true)
	w.CenterOnScreen()
	go func() {
		time.Sleep(100 * time.Millisecond)
		bringToFront()
	}()
	w.ShowAndRun()

	message := ""
	select {
	case message = <-messageCh:
	default:
	}
	return result, message
}
