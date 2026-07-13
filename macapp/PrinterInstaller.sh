#!/bin/bash

DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$DIR/printer-installer-darwin"
DRIVERS_DIR="$DIR/../Resources/drivers"
DRVARG="--drivers '$DRIVERS_DIR'"

eval "$("$BINARY" $DRVARG --ui-env 2>/dev/null)"

# --- Rosetta ---
if [ "$(uname -m)" = "arm64" ] && ! /usr/bin/arch -x86_64 /bin/ls >/dev/null 2>&1; then
	osascript -e "do shell script \"softwareupdate --install-rosetta --agree-to-license\" with administrator privileges" 2>/dev/null
fi

# --- Shift → admin ---
SHIFT=$(osascript -l JavaScript -e "ObjC.import('Cocoa'); ($.NSEvent.modifierFlags & 131072) != 0 ? '1' : '0'" 2>/dev/null)
if [ "$SHIFT" = "1" ]; then
	osascript -e "do shell script \"'$BINARY' $DRVARG --admin > '/tmp/printer-installer-result.log' 2>&1\" with administrator privileges with prompt \"$ADMIN_INSTALL_PROMPT\""
	exit 0
fi

# --- Discover ---
DISCOVERED=$("$BINARY" $DRVARG --discover 2>/dev/null)
DETECTED_IP=$(echo "$DISCOVERED" | grep "^IP=" | head -1 | cut -d= -f2)
DETECTED_MODEL=$(echo "$DISCOVERED" | grep "^Model=" | head -1 | cut -d= -f2)
DETECTED_LOCATION=$(echo "$DISCOVERED" | grep "^Location=" | head -1 | cut -d= -f2)

# --- Resolve ---
DETECTED_NAME=""
if [ -n "$DETECTED_LOCATION" ]; then
	RESOLVED=$("$BINARY" $DRVARG --resolve-location "$DETECTED_LOCATION" 2>/dev/null)
	DETECTED_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
	DETECTED_IP2=$(echo "$RESOLVED" | grep "^IP=" | head -1 | cut -d= -f2)
	[ -n "$DETECTED_IP2" ] && DETECTED_IP="$DETECTED_IP2"
fi

ALL_LOCATIONS=$("$BINARY" $DRVARG --list-locations 2>/dev/null)
EXISTING_NAME=""
[ -n "$DETECTED_IP" ] && EXISTING_NAME=$("$BINARY" $DRVARG --printer-at-ip "$DETECTED_IP" 2>/dev/null)
ALL_PRINTERS=$("$BINARY" $DRVARG --debug-printers 2>/dev/null)

# --- Location dropdown items ---
LOC_ITEMS=""
if [ -n "$ALL_LOCATIONS" ]; then
	FIRST=true
	while IFS=',' read -ra NAMES; do
		for name in "${NAMES[@]}"; do
			name=$(echo "$name" | sed 's/^ *//;s/ *$//')
			[ -z "$name" ] && continue
			[ "$FIRST" = true ] && LOC_ITEMS="\"$name\"" || LOC_ITEMS="$LOC_ITEMS, \"$name\""
			FIRST=false
		done
	done <<< "$ALL_LOCATIONS"
fi

# --- Delete items (exclude detected printer and conflicting printer) ---
DELETE_ITEMS=""
if [ -n "$ALL_PRINTERS" ]; then
	IFS=',' read -ra PNAMES <<< "$ALL_PRINTERS"
	for pn in "${PNAMES[@]}"; do
		pn=$(echo "$pn" | sed 's/^ *//;s/ *$//')
		[ -z "$pn" ] && continue
		[ "$pn" = "$DETECTED_NAME" ] && continue
		[ "$pn" = "$EXISTING_NAME" ] && continue
		if [ -z "$DELETE_ITEMS" ]; then
			DELETE_ITEMS="\"$pn\""
		else
			DELETE_ITEMS="$DELETE_ITEMS, \"$pn\""
		fi
	done
fi

# --- Escape ---
js_escape() {
	local s="$1"
	s="${s//\\/\\\\}"
	s="${s//\"/\\\"}"
	echo "\"$s\""
}

CONFIRM_TEXT=$(echo "$CONFIRM_FMT" | sed "s/%s/$DETECTED_LOCATION/")

# --- Write JXA ---
JXA_SCRIPT="/tmp/printer-installer-ui.jxa"
cat > "$JXA_SCRIPT" <<ENDJXA
ObjC.import('Cocoa')

var locItems = [$LOC_ITEMS]
var deleteItems = [$DELETE_ITEMS]
var conflictName = $(js_escape "$EXISTING_NAME")
var detectedLoc = $(js_escape "$DETECTED_LOCATION")
var detectedName = $(js_escape "$DETECTED_NAME")
var detectedIP = $(js_escape "$DETECTED_IP")
var model = $(js_escape "$DETECTED_MODEL")

var title = $(js_escape "$TITLE")
var confirmText = $(js_escape "$CONFIRM_TEXT")
var overwriteLabel = $(js_escape "$OVERWRITE_LABEL")
var skipLabel = $(js_escape "$SKIP_BTN")
var pickerPrompt = $(js_escape "$PICKER_PROMPT")
var conflictLabel = $(js_escape "$CONFLICT_LABEL")
var delPrompt = $(js_escape "$CHOOSE_PROMPT")
var okLabel = "$OK_LABEL"
var cancelLabel = "$CANCEL_LABEL"

// --- Layout ---
var CW = 480, M = 20, X1 = M, X2 = M + 20, LH = 22
var views = [], hiddenViews = [], Y = 4
var pickerLabel, popupLoc, lastSep1

function row(t, x, tag) {
	var f = $.NSTextField.alloc.initWithFrame($.NSMakeRect(x, Y, CW-x, LH))
	f.stringValue = t; f.editable = false; f.bordered = false; f.drawsBackground = false
	f.font = $.NSFont.systemFontOfSize(12)
	if (tag != null) f.tag = tag
	views.push(f); Y += LH + 2; return f
}
function chk(t, x, tag, checked) {
	var b = $.NSButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW-x, LH+2))
	b.title = t; b.setButtonType($.NSSwitchButton); b.tag = tag
	b.font = $.NSFont.systemFontOfSize(12)
	if (checked) b.state = $.NSOnState
	views.push(b); Y += LH + 2; return b
}
function pop(items, x, tag) {
	var p = $.NSPopUpButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW-x-20, 24))
	for (var i = 0; i < items.length; i++) p.addItemWithTitle(items[i])
	p.tag = tag; p.font = $.NSFont.systemFontOfSize(12)
	views.push(p); Y += 28; return p
}
function sep() {
	var b = $.NSBox.alloc.initWithFrame($.NSMakeRect(X1, Y, CW-X1, 1))
	b.boxType = $.NSSeparator; views.push(b); Y += 8; return b
}

// 1. Location confirm
var chkConfirm = chk(confirmText, X1, 10, true)

// 2. Location picker (hidden by default, folds in/out)
pickerLabel = row(pickerPrompt, X1, 21)
popupLoc = pop(locItems, X2, 22)
pickerLabel.hidden = true
popupLoc.hidden = true
hiddenViews.push(pickerLabel, popupLoc)

lastSep1 = sep()

// 3. Conflict (popup Skip/Overwrite)
if (conflictName != "") {
	row(conflictLabel, X1, 31)
	var conflictPopup = pop([skipLabel, overwriteLabel], X2, 32)
	sep()
}

// 4. Delete other printers
if (deleteItems.length > 0 && deleteItems[0] != "") {
	row(delPrompt, X1, 41)
	var delBoxes = []
	for (var i = 0; i < deleteItems.length; i++) {
		delBoxes.push(chk(deleteItems[i], X2, 200+i, false))
	}
}

// --- Assemble accessory ---
Y += 4; var totalH = Y
var acc = $.NSView.alloc.initWithFrame($.NSMakeRect(0, 0, CW, totalH))

function relayout() {
	// Filter out hidden views and recalculate
	var vy = 4
	for (var i = 0; i < views.length; i++) {
		var v = views[i]
		if (v.hidden) { v.setFrameOrigin($.NSMakePoint(-100, -100)); continue }
		var r = v.frame
		v.frame = $.NSMakeRect(r.origin.x, totalH - vy - r.size.height, r.size.width, r.size.height)
		vy += r.size.height + (v.tag && v.tag >= 20 ? 2 : 2)
	}
	// Adjust total height
	var newH = vy + 8
	acc.setFrameSize($.NSMakeSize(CW, newH))
}

relayout()

// --- Toggle handler ---
ObjC.registerSubclass({
	name: "ToggleHandler",
	methods: {"toggle:": {types:["void",["id"]], implementation:function(s) {
		var show = (chkConfirm.state != $.NSOnState)
		for (var i = 0; i < hiddenViews.length; i++) {
			hiddenViews[i].hidden = !show
		}
		relayout()
	}}}
})
var handler = $.ToggleHandler.alloc.init
chkConfirm.target = handler
chkConfirm.action = 'toggle:'

// --- Show ---
var info = detectedLoc + "  |  " + detectedName + "  |  IP: " + detectedIP + "  |  " + model
var alert = $.NSAlert.alloc.init
alert.messageText = title
alert.informativeText = info
alert.accessoryView = acc
alert.addButtonWithTitle(okLabel)
alert.addButtonWithTitle(cancelLabel)

if (alert.runModal != $.NSAlertFirstButtonReturn) { "" } else {
	var lines = []
	lines.push("CONFIRM=" + (chkConfirm.state == $.NSOnState ? "true" : "false"))
	lines.push("LOCATION=" + (popupLoc.titleOfSelectedItem.js || detectedLoc.js))
	if (typeof conflictPopup != 'undefined') {
		lines.push("OVERWRITE=" + (conflictPopup.indexOfSelectedItem == 1 ? "true" : "false"))
	} else {
		lines.push("OVERWRITE=false")
	}
	if (typeof delBoxes != 'undefined') {
		for (var i = 0; i < delBoxes.length; i++) {
			if (delBoxes[i].state == $.NSOnState) {
				lines.push("DELETE=" + delBoxes[i].title.js)
			}
		}
	}
	lines.join("\n")
}
ENDJXA

RESULT=$(osascript -l JavaScript "$JXA_SCRIPT" 2>/dev/null)

# --- Parse ---
[ -z "$RESULT" ] && exit 0

CONFIRMED=$(echo "$RESULT" | grep "^CONFIRM=" | cut -d= -f2)
PICKED_LOC=$(echo "$RESULT" | grep "^LOCATION=" | cut -d= -f2-)
DO_OVERWRITE=$(echo "$RESULT" | grep "^OVERWRITE=" | cut -d= -f2)
TO_DELETE=$(echo "$RESULT" | grep "^DELETE=" | cut -d= -f2- | tr '\n' ', ' | sed 's/, $//')

if [ "$CONFIRMED" = "true" ]; then
	CHOSEN_LOC="$DETECTED_LOCATION"
else
	CHOSEN_LOC="$PICKED_LOC"
fi

CHOSEN_NAME=""
if [ -n "$CHOSEN_LOC" ]; then
	RESOLVED=$("$BINARY" $DRVARG --resolve-location "$CHOSEN_LOC" 2>/dev/null)
	CHOSEN_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
fi

FILTERED_DELETE=""
if [ -n "$TO_DELETE" ]; then
	IFS=',' read -ra DLIST <<< "$TO_DELETE"
	for d in "${DLIST[@]}"; do
		d=$(echo "$d" | sed 's/^ *//;s/ *$//')
		[ -z "$d" ] && continue
		[ "$d" = "$CHOSEN_NAME" ] && continue
		[ -z "$FILTERED_DELETE" ] && FILTERED_DELETE="$d" || FILTERED_DELETE="$FILTERED_DELETE, $d"
	done
fi

echo ""
echo "========== Analysis =========="
echo "Confirmed:   $CONFIRMED"
echo "Location:    $CHOSEN_LOC"
echo "Printer:     ${CHOSEN_NAME:-$DETECTED_NAME}"
[ "$DO_OVERWRITE" = "true" ] && echo "Overwrite:   yes"
[ "$DO_OVERWRITE" = "false" ] && echo "Skip:        yes"
[ -n "$FILTERED_DELETE" ] && echo "To delete:   $FILTERED_DELETE"
[ -z "$FILTERED_DELETE" ] && echo "To delete:   none"
echo "=============================="
