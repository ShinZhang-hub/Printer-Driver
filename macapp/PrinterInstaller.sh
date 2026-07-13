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

# --- Location items ---
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
	b.boxType = $.NSSeparator; views.push(b); Y += 8
}

ck(confirmText, X1, true)
sp()

if (locItems.length > 0) {
	txt(pickerPrompt, X1)
	pp(locItems, X2)
	sp()
}

if (conflictName != "") {
	txt(conflictLabel, X1)
	pp([skipLabel, overwriteLabel], X2)
	sp()
}

if (deleteItems.length > 0 && deleteItems[0] != "") {
	txt(delPrompt, X1)
	for (var i = 0; i < deleteItems.length; i++) ck(deleteItems[i], X2, false)
}

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
	"ok"
}
ENDJXA

RESULT=$(osascript -l JavaScript /tmp/printer-installer-ui.jxa 2>&1)
echo "result: $RESULT"
