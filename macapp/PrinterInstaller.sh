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

# --- Resolve detected location ---
DETECTED_NAME=""
if [ -n "$DETECTED_LOCATION" ]; then
	RESOLVED=$("$BINARY" $DRVARG --resolve-location "$DETECTED_LOCATION" 2>/dev/null)
	DETECTED_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
	DETECTED_IP2=$(echo "$RESOLVED" | grep "^IP=" | head -1 | cut -d= -f2)
	[ -n "$DETECTED_IP2" ] && DETECTED_IP="$DETECTED_IP2"
fi

# --- All locations for dropdown ---
ALL_LOCATIONS=$("$BINARY" $DRVARG --list-locations 2>/dev/null)

# --- Conflict check (against detected IP) ---
EXISTING_NAME=""
[ -n "$DETECTED_IP" ] && EXISTING_NAME=$("$BINARY" $DRVARG --printer-at-ip "$DETECTED_IP" 2>/dev/null)

# --- All printers (for delete list, exclude detected printer) ---
ALL_PRINTERS=$("$BINARY" $DRVARG --debug-printers 2>/dev/null)

# ===== Build data for JXA =====

# Location dropdown items
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

# Delete printer checkbox items (exclude detected printer)
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

# --- JS escape ---
js_escape() {
	local s="$1"
	s="${s//\\/\\\\}"
	s="${s//\"/\\\"}"
	echo "\"$s\""
}

CONFIRM_TEXT=$(echo "$CONFIRM_FMT" | sed "s/%s/$DETECTED_LOCATION/")

# --- Show dialog ---
RESULT=$(osascript -l JavaScript 2>/dev/null <<ENDJXA
ObjC.import('Cocoa')

// Data from shell
var locItems = [$LOC_ITEMS]
var deleteItems = [$DELETE_ITEMS]
var conflictName = $(js_escape "$EXISTING_NAME")
var detectedLoc = $(js_escape "$DETECTED_LOCATION")
var detectedName = $(js_escape "$DETECTED_NAME")
var detectedIP = $(js_escape "$DETECTED_IP")
var model = $(js_escape "$DETECTED_MODEL")

// i18n
var title = $(js_escape "$TITLE")
var confirmText = $(js_escape "$CONFIRM_TEXT")
var overwriteLabel = $(js_escape "$OVERWRITE_LABEL")
var skipLabel = $(js_escape "$SKIP_BTN")
var pickerPrompt = $(js_escape "$PICKER_PROMPT")
var delPrompt = $(js_escape "$CHOOSE_PROMPT")
var okLabel = "$OK_LABEL"
var cancelLabel = "$CANCEL_LABEL"

// --- Layout ---
var CW = 480, M = 20, LH = 24, X1 = M, X2 = M + 20
var views = [], Y = 6

function lbl(t, x, bold) {
	var f = $.NSTextField.alloc.initWithFrame($.NSMakeRect(x, Y, CW-x, LH))
	f.stringValue = t; f.editable = false; f.bordered = false; f.drawsBackground = false
	f.font = bold ? $.NSFont.boldSystemFontOfSize(12) : $.NSFont.systemFontOfSize(11)
	views.push(f); Y += LH + 4; return f
}
function chk(t, x, tag, checked) {
	var b = $.NSButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW-x, LH+2))
	b.title = t; b.setButtonType($.NSSwitchButton); b.tag = tag
	b.font = $.NSFont.systemFontOfSize(12)
	if (checked) b.state = $.NSOnState
	views.push(b); Y += LH + 4; return b
}
function pop(items, x, tag) {
	var p = $.NSPopUpButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW-x-20, 24))
	for (var i = 0; i < items.length; i++) p.addItemWithTitle(items[i])
	p.tag = tag; p.font = $.NSFont.systemFontOfSize(12)
	views.push(p); Y += 30; return p
}
function sep() {
	var b = $.NSBox.alloc.initWithFrame($.NSMakeRect(X1, Y, CW-X1, 1))
	b.boxType = $.NSSeparator; views.push(b); Y += 10; return b
}

// Section 1: Location confirm  [1. 位置确认]
var chkConfirm = chk(confirmText, X1, 10, true)

// Section 2: Location picker  [2. 位置选择]
lbl(pickerPrompt, X1, true)
var popupLoc = pop(locItems, X2, 20)

sep()

// Section 3: Conflict  [3. 冲突处理]
if (conflictName != "") {
	var conflictLabel = overwriteLabel + ": " + conflictName + " (IP: " + detectedIP + ")"
	var chkOverwrite = chk(conflictLabel, X1, 30, false)
}

sep()

// Section 4: Delete other printers  [4. 删除其他打印机]
if (deleteItems.length > 0 && deleteItems[0] != "") {
	lbl(delPrompt, X1, true)
	var delBoxes = []
	for (var i = 0; i < deleteItems.length; i++) {
		delBoxes.push(chk(deleteItems[i], X2, 200+i, false))
	}
}

// --- Assemble accessory view ---
Y += 8; var totalH = Y
var acc = $.NSView.alloc.initWithFrame($.NSMakeRect(0, 0, CW, totalH))
for (var i = 0; i < views.length; i++) {
	var v = views[i], r = v.frame
	v.frame = $.NSMakeRect(r.origin.x, totalH - r.origin.y - r.size.height, r.size.width, r.size.height)
	acc.addSubview(v)
}

// --- Show ---
var info = "Location: " + detectedLoc + "  |  Printer: " + detectedName + "  |  IP: " + detectedIP + "  |  Model: " + model
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
	if (typeof chkOverwrite != 'undefined') {
		lines.push("OVERWRITE=" + (chkOverwrite.state == $.NSOnState ? "true" : "false"))
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
)

# --- Parse result ---
[ -z "$RESULT" ] && exit 0

CONFIRMED=$(echo "$RESULT" | grep "^CONFIRM=" | cut -d= -f2)
PICKED_LOC=$(echo "$RESULT" | grep "^LOCATION=" | cut -d= -f2-)
DO_OVERWRITE=$(echo "$RESULT" | grep "^OVERWRITE=" | cut -d= -f2)
TO_DELETE=$(echo "$RESULT" | grep "^DELETE=" | cut -d= -f2- | tr '\n' ', ' | sed 's/, $//')

# --- Filter: exclude current location's printer from delete list ---
# Resolve the chosen location to get its printer name
if [ "$CONFIRMED" = "true" ]; then
	CHOSEN_LOC="$DETECTED_LOCATION"
else
	CHOSEN_LOC="$PICKED_LOC"
fi

CHOSEN_NAME=""
if [ -n "$CHOSEN_LOC" ] && [ "$CHOSEN_LOC" != "$DETECTED_LOCATION" ]; then
	RESOLVED=$("$BINARY" $DRVARG --resolve-location "$CHOSEN_LOC" 2>/dev/null)
	CHOSEN_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
fi

# Remove chosen printer from delete list
FILTERED_DELETE=""
if [ -n "$TO_DELETE" ]; then
	IFS=',' read -ra DLIST <<< "$TO_DELETE"
	for d in "${DLIST[@]}"; do
		d=$(echo "$d" | sed 's/^ *//;s/ *$//')
		[ -z "$d" ] && continue
		[ "$d" = "$CHOSEN_NAME" ] && continue
		[ "$d" = "$DETECTED_NAME" ] && [ "$CONFIRMED" = "true" ] && continue
		[ -z "$FILTERED_DELETE" ] && FILTERED_DELETE="$d" || FILTERED_DELETE="$FILTERED_DELETE, $d"
	done
fi

# --- Output ---
echo ""
echo "========== Analysis =========="
echo "Confirmed:   $CONFIRMED"
echo "Location:    $CHOSEN_LOC"
echo "Printer:     ${CHOSEN_NAME:-$DETECTED_NAME}"
[ "$DO_OVERWRITE" = "true" ] && echo "Overwrite:   yes (will reinstall over existing)"
[ "$DO_OVERWRITE" = "false" ] && echo "Overwrite:   no (skip existing)"
[ -n "$FILTERED_DELETE" ] && echo "To delete:   $FILTERED_DELETE"
[ -z "$FILTERED_DELETE" ] && echo "To delete:   none"
echo "=============================="
