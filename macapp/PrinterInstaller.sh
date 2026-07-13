#!/bin/bash

DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$DIR/printer-installer-darwin"
DRIVERS_DIR="$DIR/../Resources/drivers"

eval "$("$BINARY" --drivers "$DRIVERS_DIR" --ui-env 2>/dev/null)"

# --- Rosetta ---
if [ "$(uname -m)" = "arm64" ] && ! /usr/bin/arch -x86_64 /bin/ls >/dev/null 2>&1; then
	osascript -e "do shell script \"softwareupdate --install-rosetta --agree-to-license\" with administrator privileges" 2>/dev/null
fi

# --- Shift → admin ---
SHIFT=$(osascript -l JavaScript -e "ObjC.import('Cocoa'); ($.NSEvent.modifierFlags & 131072) != 0 ? '1' : '0'" 2>/dev/null)
if [ "$SHIFT" = "1" ]; then
	osascript -e "do shell script \"'$BINARY' --drivers \"$DRIVERS_DIR\" --admin > '/tmp/printer-installer-result.log' 2>&1\" with administrator privileges with prompt \"$ADMIN_INSTALL_PROMPT\""
	exit 0
fi

# --- Discover ---
DISCOVERED=$("$BINARY" --drivers "$DRIVERS_DIR" --discover 2>/dev/null)
DETECTED_IP=$(echo "$DISCOVERED" | grep "^IP=" | head -1 | cut -d= -f2)
DETECTED_MODEL=$(echo "$DISCOVERED" | grep "^Model=" | head -1 | cut -d= -f2)
DETECTED_LOCATION=$(echo "$DISCOVERED" | grep "^Location=" | head -1 | cut -d= -f2)

DETECTED_NAME=""
if [ -n "$DETECTED_LOCATION" ]; then
	RESOLVED=$("$BINARY" --drivers "$DRIVERS_DIR" --resolve-location "$DETECTED_LOCATION" 2>/dev/null)
	DETECTED_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
	DETECTED_IP2=$(echo "$RESOLVED" | grep "^IP=" | head -1 | cut -d= -f2)
	[ -n "$DETECTED_IP2" ] && DETECTED_IP="$DETECTED_IP2"
fi

ALL_LOCATIONS=$("$BINARY" --drivers "$DRIVERS_DIR" --list-locations 2>/dev/null)
EXISTING_NAME=""
[ -n "$DETECTED_IP" ] && EXISTING_NAME=$("$BINARY" --drivers "$DRIVERS_DIR" --printer-at-ip "$DETECTED_IP" 2>/dev/null)
ALL_PRINTERS=$("$BINARY" --drivers "$DRIVERS_DIR" --debug-printers 2>/dev/null)

# --- Build filtered location list (exclude detected) ---
LOC_ITEMS=""
LOC_ITEMS_NO_DETECTED=""
if [ -n "$ALL_LOCATIONS" ]; then
	FIRST=true; FIRST2=true
	while IFS=',' read -ra NAMES; do
		for name in "${NAMES[@]}"; do
			name=$(echo "$name" | sed 's/^ *//;s/ *$//')
			[ -z "$name" ] && continue
			[ "$FIRST" = true ] && LOC_ITEMS="\"$name\"" || LOC_ITEMS="$LOC_ITEMS, \"$name\""
			FIRST=false
			if [ "$name" != "$DETECTED_LOCATION" ]; then
				[ "$FIRST2" = true ] && LOC_ITEMS_NO_DETECTED="\"$name\"" || LOC_ITEMS_NO_DETECTED="$LOC_ITEMS_NO_DETECTED, \"$name\""
				FIRST2=false
			fi
		done
	done <<< "$ALL_LOCATIONS"
fi

# --- Delete items ---
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

js_escape() {
	local s="$1"; s="${s//\\/\\\\}"; s="${s//\"/\\\"}"; echo "\"$s\""
}
CONFIRM_TEXT=$(echo "$CONFIRM_FMT" | sed "s/%s/$DETECTED_LOCATION/")

cat > /tmp/printer-installer-ui.jxa <<ENDJXA
ObjC.import('Cocoa')

var locItems = [$LOC_ITEMS]
var locItemsNoDetected = [$LOC_ITEMS_NO_DETECTED]
var deleteItems = [$DELETE_ITEMS]
var detectedLoc = $(js_escape "$DETECTED_LOCATION")
var detectedName = $(js_escape "$DETECTED_NAME")
var detectedIP = $(js_escape "$DETECTED_IP")
var model = $(js_escape "$DETECTED_MODEL")
var conflictName = $(js_escape "$EXISTING_NAME")

var title = $(js_escape "$TITLE")
var confirmText = $(js_escape "$CONFIRM_TEXT")
var overwriteLabel = $(js_escape "$OVERWRITE_LABEL")
var skipLabel = $(js_escape "$SKIP_BTN")
var pickerPrompt = $(js_escape "$PICKER_PROMPT")
var conflictLabel = $(js_escape "$CONFLICT_LABEL")
var delPrompt = $(js_escape "$CHOOSE_PROMPT")

var CW = 480, M = 20, X1 = M, X2 = M + 16, LH = 22
var Y = 4, views = []
var pickerPopup

function txt(s, x) {
	var f = $.NSTextField.alloc.initWithFrame($.NSMakeRect(x, Y, CW - x, LH))
	f.stringValue = s; f.editable = false; f.bordered = false; f.drawsBackground = false
	f.font = $.NSFont.systemFontOfSize(12)
	views.push(f); Y += LH + 2
}
function ck(s, x, checked) {
	var b = $.NSButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW - x, LH + 2))
	b.title = s; b.setButtonType($.NSSwitchButton)
	b.font = $.NSFont.systemFontOfSize(12)
	if (checked) b.state = $.NSOnState
	views.push(b); Y += LH + 2
	return b
}
function pp(items, x) {
	var p = $.NSPopUpButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW - x - 20, 24))
	for (var i = 0; i < items.length; i++) p.addItemWithTitle(items[i])
	p.font = $.NSFont.systemFontOfSize(12)
	views.push(p); Y += 28; return p
}
function sp() {
	var b = $.NSBox.alloc.initWithFrame($.NSMakeRect(X1, Y, CW - X1, 1))
	b.title = ""
	b.boxType = $.NSSeparator; views.push(b); Y += 8
}

// 1. Location confirm
var chkConfirm = ck(confirmText, X1, true)

// 2. Location picker (disabled when #1 checked)
pickerPopup = pp(locItemsNoDetected, X2)
pickerPopup.enabled = false

sp()

// 3. Conflict
if (conflictName != "") {
	var conflictTxt = conflictLabel + " (" + conflictName + ")"
	txt(conflictTxt, X1)
	var conflictPopup = pp([skipLabel, overwriteLabel], X2)
	sp()
}

// 4. Delete other printers
if (deleteItems.length > 0 && deleteItems[0] != "") {
	txt(delPrompt, X1)
	var delBoxes = []
	for (var i = 0; i < deleteItems.length; i++) {
		delBoxes.push(ck(deleteItems[i], X2, false))
	}
}

// --- Toggle: #1 disables/enables #2 ---
ObjC.registerSubclass({
	name: "ToggleHandler",
	methods: {"toggle:": {types:["void",["id"]], implementation:function(s) {
		pickerPopup.enabled = (chkConfirm.state != $.NSOnState)
	}}}
})
var handler = $.ToggleHandler.alloc.init
chkConfirm.target = handler
chkConfirm.action = 'toggle:'

// --- Assemble ---
Y += 8
var acc = $.NSView.alloc.initWithFrame($.NSMakeRect(0, 0, CW, Y))
for (var i = 0; i < views.length; i++) {
	var v = views[i], r = v.frame
	v.frame = $.NSMakeRect(r.origin.x, Y - r.origin.y - r.size.height - 4, r.size.width, r.size.height)
	acc.addSubview(v)
}

var info = detectedLoc + "  |  " + detectedName + "  |  IP: " + detectedIP + "  |  " + model
var alert = $.NSAlert.alloc.init
alert.messageText = title
alert.informativeText = info
alert.accessoryView = acc
alert.addButtonWithTitle("$OK_LABEL")
alert.addButtonWithTitle("$CANCEL_LABEL")

if (alert.runModal != $.NSAlertFirstButtonReturn) { "" } else {
	var lines = []
	lines.push("CONFIRM=" + (chkConfirm.state == $.NSOnState ? "true" : "false"))
	lines.push("LOCATION=" + (pickerPopup.titleOfSelectedItem.js || detectedLoc.js))
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

RESULT=$(osascript -l JavaScript /tmp/printer-installer-ui.jxa 2>&1)

# --- Parse ---
[ -z "$RESULT" ] && exit 0
CONFIRMED=$(echo "$RESULT" | grep "^CONFIRM=" | cut -d= -f2)
PICKED_LOC=$(echo "$RESULT" | grep "^LOCATION=" | cut -d= -f2-)
DO_OVERWRITE=$(echo "$RESULT" | grep "^OVERWRITE=" | cut -d= -f2)
TO_DELETE=$(echo "$RESULT" | grep "^DELETE=" | cut -d= -f2- | tr '\n' ',' | sed 's/,$//')

if [ "$CONFIRMED" = "true" ]; then CHOSEN_LOC="$DETECTED_LOCATION"
else CHOSEN_LOC="$PICKED_LOC"; fi

CHOSEN_NAME=""
if [ -n "$CHOSEN_LOC" ]; then
	RESOLVED=$("$BINARY" --drivers "$DRIVERS_DIR" --resolve-location "$CHOSEN_LOC" 2>/dev/null)
	CHOSEN_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
fi

# --- Filter delete list ---
FILTERED=""
if [ -n "$TO_DELETE" ]; then
	IFS=',' read -ra DLIST <<< "$TO_DELETE"
	for d in "${DLIST[@]}"; do d=$(echo "$d" | sed 's/^ *//;s/ *$//'); [ -z "$d" ] && continue; [ "$d" = "$CHOSEN_NAME" ] && continue
		[ -z "$FILTERED" ] && FILTERED="$d" || FILTERED="$FILTERED, $d"; done
fi

echo ""
echo "========== Analysis =========="
echo "Confirmed:   $CONFIRMED"
echo "Location:    $CHOSEN_LOC"
echo "Printer:     ${CHOSEN_NAME:-$DETECTED_NAME}"
[ "$DO_OVERWRITE" = "true" ] && echo "Overwrite:   yes" || echo "Skip:        yes"
[ -n "$FILTERED" ] && echo "To delete:   $FILTERED" || echo "To delete:   none"
echo "=============================="
