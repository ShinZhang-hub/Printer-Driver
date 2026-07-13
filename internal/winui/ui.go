package winui

import (
	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"
)

type Result struct {
	Location    string
	Overwrite   bool
	DeleteNames []string
	Cancelled   bool
}

func Run(detectedLoc string, allLocations []string, deletePrinters []string) *Result {
	otherLocs := make([]string, 0)
	for _, l := range allLocations {
		if l != detectedLoc {
			otherLocs = append(otherLocs, l)
		}
	}

	var (
		dlg          *walk.Dialog
		confirmCheck *walk.CheckBox
		locCombo     *walk.ComboBox
		conflictCombo *walk.ComboBox
	)

	var delCheckBoxes []*walk.CheckBox
	delWidgets := make([]decl.Widget, len(deletePrinters))
	for i, p := range deletePrinters {
		cb := &walk.CheckBox{}
		delCheckBoxes = append(delCheckBoxes, cb)
		delWidgets[i] = decl.CheckBox{
			Text:     p,
			AssignTo: &delCheckBoxes[i],
		}
	}

	result := &Result{}

	_, walkErr := decl.Dialog{
		AssignTo: &dlg,
		Title:    "Printer Installer",
		MinSize:  decl.Size{Width: 440, Height: 300},
		Layout:   decl.VBox{Margins: decl.Margins{10, 10, 10, 10}},
		Children: []decl.Widget{
			decl.Label{Text: "Location:"},
			decl.CheckBox{
				Text:      "Use: " + detectedLoc,
				Checked:   detectedLoc != "",
				AssignTo:  &confirmCheck,
				Visible:   detectedLoc != "",
				OnCheckedChanged: func() {
					if locCombo != nil {
						locCombo.SetVisible(!confirmCheck.Checked())
					}
				},
			},
			decl.ComboBox{
				AssignTo: &locCombo,
				Model:    otherLocs,
				Visible:  false,
			},
			decl.Label{Text: "Conflict handling:"},
			decl.ComboBox{
				AssignTo: &conflictCombo,
				Model:    []string{"Skip", "Overwrite"},
			},
			decl.GroupBox{
				Title:  "Existing printers:",
				Layout: decl.VBox{},
				Children: delWidgets,
			},
			decl.Composite{
				Layout: decl.HBox{Alignment: decl.AlignHFarVCenter},
				Children: []decl.Widget{
					decl.PushButton{
						Text: "Install",
						OnClicked: func() {
							loc := detectedLoc
							if !confirmCheck.Checked() && locCombo != nil {
								loc = locCombo.Text()
							}
							delNames := make([]string, 0)
							for _, cb := range delCheckBoxes {
								if cb.Checked() {
									delNames = append(delNames, cb.Text())
								}
							}
							result = &Result{
								Location:    loc,
								Overwrite:   conflictCombo.Text() == "Overwrite",
								DeleteNames: delNames,
							}
							dlg.Accept()
						},
					},
					decl.PushButton{
						Text: "Cancel",
						OnClicked: func() {
							result = &Result{Cancelled: true}
							dlg.Cancel()
						},
					},
				},
			},
		},
	}.Run(nil)

	if walkErr != nil && result == nil {
		return &Result{Cancelled: true}
	}

	return result
}
