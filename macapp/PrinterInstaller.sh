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
	osascript 2>/dev/null <<ENDADMIN
do shell script "'$BINARY' --drivers '$DRIVERS_DIR' --admin > /tmp/printer-installer-result.log 2>&1" with administrator privileges with prompt "$ADMIN_INSTALL_PROMPT"
ENDADMIN
	exit 0
fi

# --- Discover ---
DISCOVERED=$("$BINARY" --drivers "$DRIVERS_DIR" --discover 2>/dev/null)
DETECTED_IP=$(echo "$DISCOVERED" | grep "^IP=" | head -1 | cut -d= -f2)
DETECTED_MODEL=$(echo "$DISCOVERED" | grep "^Model=" | head -1 | cut -d= -f2)
DETECTED_LOCATION=$(echo "$DISCOVERED" | grep "^Location=" | head -1 | cut -d= -f2)

# --- Build printer list for detected location ---
ALL_PRINTER_NAMES=""
ALL_PRINTER_IPS=""
DETECTED_NAME=""
if [ -n "$DETECTED_LOCATION" ]; then
	RESOLVED=$("$BINARY" --drivers "$DRIVERS_DIR" --resolve-location "$DETECTED_LOCATION" 2>/dev/null)
	while IFS= read -r line; do
		case "$line" in
			Name=*) ALL_PRINTER_NAMES="${ALL_PRINTER_NAMES}${ALL_PRINTER_NAMES:+,}$(echo "$line" | cut -d= -f2-)" ;;
			IP=*)   ALL_PRINTER_IPS="${ALL_PRINTER_IPS}${ALL_PRINTER_IPS:+,}$(echo "$line" | cut -d= -f2-)" ;;
		esac
	done <<< "$RESOLVED"
	DETECTED_NAME=$(echo "$ALL_PRINTER_NAMES" | cut -d, -f1)
	DETECTED_IP=$(echo "$ALL_PRINTER_IPS" | cut -d, -f1)
fi

ALL_LOCATIONS=$("$BINARY" --drivers "$DRIVERS_DIR" --list-locations 2>/dev/null)
EXISTING_NAME=""
[ -n "$DETECTED_IP" ] && EXISTING_NAME=$("$BINARY" --drivers "$DRIVERS_DIR" --printer-at-ip "$DETECTED_IP" 2>/dev/null)
ALL_PRINTERS=$("$BINARY" --drivers "$DRIVERS_DIR" --debug-printers 2>/dev/null)

# --- Build location lists ---
LOC_ITEMS_ALL=""       # all locations
LOC_ITEMS_NODETECT=""  # exclude detected
if [ -n "$ALL_LOCATIONS" ]; then
	F1=true; F2=true
	while IFS=',' read -ra NAMES; do
		for name in "${NAMES[@]}"; do
			name=$(echo "$name" | sed 's/^ *//;s/ *$//')
			[ -z "$name" ] && continue
			[ "$F1" = true ] && LOC_ITEMS_ALL="\"$name\"" || LOC_ITEMS_ALL="$LOC_ITEMS_ALL, \"$name\""
			F1=false
			if [ "$name" != "$DETECTED_LOCATION" ]; then
				[ "$F2" = true ] && LOC_ITEMS_NODETECT="\"$name\"" || LOC_ITEMS_NODETECT="$LOC_ITEMS_NODETECT, \"$name\""
				F2=false
			fi
		done
	done <<< "$ALL_LOCATIONS"
fi

# --- Build delete list: ALL printers with their location resolution ---
DELETE_ITEMS=""
DELETE_LOCKED=""  # which one should be disabled initially (matches detected location)
if [ -n "$ALL_PRINTERS" ]; then
	IFS=',' read -ra PNAMES <<< "$ALL_PRINTERS"
	for pn in "${PNAMES[@]}"; do
		pn=$(echo "$pn" | sed 's/^ *//;s/ *$//')
		[ -z "$pn" ] && continue
		# Check if this printer belongs to a known location
		PLOC=$("$BINARY" --drivers "$DRIVERS_DIR" --printer-location "$pn" 2>/dev/null || echo "")
		if [ -z "$DELETE_ITEMS" ]; then
			DELETE_ITEMS="\"$pn\""
		else
			DELETE_ITEMS="$DELETE_ITEMS, \"$pn\""
		fi
		# Track which printer maps to which location
		if [ -z "$PRINTER_MAP" ]; then
			PRINTER_MAP="$pn=$PLOC"
		else
			PRINTER_MAP="$PRINTER_MAP|$pn=$PLOC"
		fi
	done
fi

# --- Escape ---
js_escape() { local s="$1"; s="${s//\\/\\\\}"; s="${s//\"/\\\"}"; echo "\"$s\""; }
CONFIRM_TEXT=$(echo "$CONFIRM_FMT" | sed "s/%s/$DETECTED_LOCATION/")
# Split at \n, pass as two parts for JXA to join
CONFIRM_L1=$(echo -e "$CONFIRM_TEXT" | head -1)
CONFIRM_L2=$(echo -e "$CONFIRM_TEXT" | tail -1)

# Build multi-printer summary
PRINTER_SUMMARY="$DETECTED_NAME"
if [ $(echo "$ALL_PRINTER_NAMES" | tr ',' '\n' | wc -l | tr -d ' ') -gt 1 ]; then
	PRINTER_SUMMARY="$ALL_PRINTER_NAMES"
fi

# Build printer-to-location mapping for JXA
PRINTER_MAP_JS=""
if [ -n "$PRINTER_MAP" ]; then
	IFS='|' read -ra PMAP <<< "$PRINTER_MAP"
	for m in "${PMAP[@]}"; do
		pn=$(echo "$m" | cut -d= -f1)
		pl=$(echo "$m" | cut -d= -f2-)
		[ -z "$PRINTER_MAP_JS" ] && PRINTER_MAP_JS="\"$pn\":\"$pl\"" || PRINTER_MAP_JS="$PRINTER_MAP_JS, \"$pn\":\"$pl\""
	done
fi

cat > /tmp/printer-installer-ui.jxa <<ENDJXA
ObjC.import('Cocoa')

var locItemsAll = [$LOC_ITEMS_ALL]
var locItemsNoDetect = [$LOC_ITEMS_NODETECT]
var deleteItems = [$DELETE_ITEMS]
var printerMap = {$PRINTER_MAP_JS}
var detectedLoc = $(js_escape "$DETECTED_LOCATION")
var detectedNames = $(js_escape "$PRINTER_SUMMARY")
var detectedIP = $(js_escape "$DETECTED_IP")
var model = $(js_escape "$DETECTED_MODEL")
var conflictName = $(js_escape "$EXISTING_NAME")

var title = $(js_escape "$TITLE")
var confirmText = $(js_escape "$CONFIRM_L1") + "\n" + $(js_escape "$CONFIRM_L2")
var overwriteLabel = $(js_escape "$OVERWRITE_LABEL")
var skipLabel = $(js_escape "$SKIP_BTN")
var pickerPrompt = $(js_escape "$PICKER_PROMPT")
var conflictLabel = $(js_escape "$CONFLICT_LABEL")
var delPrompt = $(js_escape "$CHOOSE_PROMPT")

var CW = 480, M = 20, X1 = M, X2 = M + 16, LH = 22
var Y = 4, views = []
var pickerPopup, chkConfirm, delBoxes = []

function txt(s, x) {
	var f = $.NSTextField.alloc.initWithFrame($.NSMakeRect(x, Y, CW - x, LH))
	f.stringValue = s; f.editable = false; f.bordered = false; f.drawsBackground = false
	f.font = $.NSFont.systemFontOfSize(12)
	views.push(f); Y += LH + 2
}
function ck(s, x, checked, disabled, multiline) {
	var h = multiline ? LH * 2 + 4 : LH + 2
	var b = $.NSButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW - x, h))
	b.title = s; b.setButtonType($.NSSwitchButton)
	b.font = $.NSFont.systemFontOfSize(12)
	if (checked) b.state = $.NSOnState
	if (disabled) b.enabled = false
	views.push(b); Y += h + 2
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
	b.title = ""; b.boxType = $.NSSeparator; views.push(b); Y += 8
}

// 1. Location confirm
chkConfirm = ck(confirmText, X1, true, false, true)

// 2. Location picker — two popups toggled by #1
var itemsDetect = [$(js_escape "$DETECTED_LOCATION")]
var ppKeep = pp(itemsDetect, X2)      // shown when checked: detected location
var ppPick = pp(locItemsNoDetect, X2) // shown when unchecked: other locations
var pickerPopup = ppPick  // alias for result reading
ppPick.hidden = true

sp()

// 3. Conflict
if (conflictName != "") {
	var ct = conflictLabel + " (" + conflictName + ")"
	txt(ct, X1)
	var conflictPopup = pp([skipLabel, overwriteLabel], X2)
	sp()
}

// 4. Delete other printers
if (deleteItems.length > 0 && deleteItems[0] != "") {
	txt(delPrompt, X1)
	for (var i = 0; i < deleteItems.length; i++) {
		var pname = deleteItems[i]
		var ploc = printerMap[pname] || ""
		var disabled = (ploc == detectedLoc)
		delBoxes.push(ck(pname, X2, false, disabled))
	}
}

// --- Toggle handler ---
ObjC.registerSubclass({
	name: "TH",
	methods: {"t:": {types:["void",["id"]], implementation:function(s) {
		var on = (chkConfirm.state == $.NSOnState)
		ppKeep.hidden = !on
		ppPick.hidden = on
		pickerPopup = on ? ppKeep : ppPick
		var curLoc = on ? detectedLoc : ppPick.titleOfSelectedItem.js
		for (var i = 0; i < delBoxes.length; i++) {
			var ploc = printerMap[delBoxes[i].title.js] || ""
			delBoxes[i].enabled = (ploc != curLoc)
		}
	}}}
})
chkConfirm.target = $.TH.alloc.init
chkConfirm.action = 't:'

// --- Assemble ---
Y += 8
var acc = $.NSView.alloc.initWithFrame($.NSMakeRect(0, 0, CW, Y))
for (var i = 0; i < views.length; i++) {
	var v = views[i], r = v.frame
	v.frame = $.NSMakeRect(r.origin.x, Y - r.origin.y - r.size.height - 4, r.size.width, r.size.height)
	acc.addSubview(v)
}

var line1 = detectedLoc + "  |  " + detectedName + "  |  IP: " + detectedIP
var line2 = model
var alert = $.NSAlert.alloc.init
alert.messageText = title
alert.informativeText = line1 + "\n" + line2
alert.accessoryView = acc
alert.addButtonWithTitle("$OK_LABEL")
alert.addButtonWithTitle("$CANCEL_LABEL")

if (alert.runModal != $.NSAlertFirstButtonReturn) { "" } else {
	var lines = []
	lines.push("CONFIRM=" + (chkConfirm.state == $.NSOnState ? "true" : "false"))
	lines.push("LOCATION=" + (pickerPopup.titleOfSelectedItem.js || detectedLoc.js))
	if (typeof conflictPopup != 'undefined') {
		lines.push("OVERWRITE=" + (conflictPopup.indexOfSelectedItem == 1 ? "true" : "false"))
	} else { lines.push("OVERWRITE=false") }
	for (var i = 0; i < delBoxes.length; i++) {
		if (delBoxes[i].state == $.NSOnState) lines.push("DELETE=" + delBoxes[i].title.js)
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
if [ -n "$CHOSEN_LOC" ] && [ "$CHOSEN_LOC" != "$DETECTED_LOCATION" ]; then
	RESOLVED=$("$BINARY" --drivers "$DRIVERS_DIR" --resolve-location "$CHOSEN_LOC" 2>/dev/null)
	CHOSEN_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
else
	CHOSEN_NAME="$DETECTED_NAME"
fi

echo ""
echo "========== Analysis =========="
echo "Confirmed:   $CONFIRMED"
echo "Location:    $CHOSEN_LOC"
echo "Printer:     $CHOSEN_NAME"
[ "$DO_OVERWRITE" = "true" ] && echo "Overwrite:   yes" || echo "Skip:        yes"
[ -n "$TO_DELETE" ] && echo "To remove:   $TO_DELETE" || echo "To remove:   none"
echo "=============================="
