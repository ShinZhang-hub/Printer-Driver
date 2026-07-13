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

# --- Shift key → admin ---
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
INSTALL_NAME=""
INSTALL_IP=""
if [ -n "$DETECTED_LOCATION" ]; then
	RESOLVED=$("$BINARY" $DRVARG --resolve-location "$DETECTED_LOCATION" 2>/dev/null)
	INSTALL_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
	INSTALL_IP=$(echo "$RESOLVED" | grep "^IP=" | head -1 | cut -d= -f2)
fi

ALL_LOCATIONS=$("$BINARY" $DRVARG --list-locations 2>/dev/null)
TARGET_IP="${INSTALL_IP:-$DETECTED_IP}"
TARGET_NAME="${INSTALL_NAME:-$DETECTED_MODEL}"

# --- Conflict + other printers ---
EXISTING_NAME=""
[ -n "$TARGET_IP" ] && EXISTING_NAME=$("$BINARY" $DRVARG --printer-at-ip "$TARGET_IP" 2>/dev/null)
ALL_PRINTERS=$("$BINARY" $DRVARG --debug-printers 2>/dev/null)

# --- Build confirm text ---
CONFIRM_TEXT=$(echo "$CONFIRM_FMT" | sed "s/%s/$DETECTED_LOCATION/")

# --- Build other printer list ---
OTHER_NAMES=""
OPRINTERS=""
if [ -n "$ALL_PRINTERS" ]; then
	IFS=',' read -ra PNAMES <<< "$ALL_PRINTERS"
	for pn in "${PNAMES[@]}"; do
		pn=$(echo "$pn" | sed 's/^ *//;s/ *$//')
		[ -z "$pn" ] && continue
		[ "$pn" = "$EXISTING_NAME" ] && continue
		[ "$pn" = "$TARGET_NAME" ] && continue
		if [ -z "$OTHER_NAMES" ]; then
			OTHER_NAMES="\"$pn\""
		else
			OTHER_NAMES="$OTHER_NAMES, \"$pn\""
		fi
	done
fi

# --- Build location list ---
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

# --- Escape for JSON ---
escape_json() { echo "$1" | python3 -c "import json,sys; print(json.dumps(sys.stdin.read().strip()))" 2>/dev/null; }

# --- Call JXA dialog ---
RESULT=$(osascript -l JavaScript 2>/dev/null <<ENDJXA
ObjC.import('Cocoa')

// --- Receive data from shell ---
var locItems = [$LOC_ITEMS]
var otherNames = [$OTHER_NAMES]
var conflictName = $(escape_json "$EXISTING_NAME")
var detectedLoc = $(escape_json "$DETECTED_LOCATION")
var targetName = $(escape_json "$TARGET_NAME")
var targetIP = $(escape_json "$TARGET_IP")
var model = $(escape_json "$DETECTED_MODEL")

// --- i18n ---
var confirmText = $(escape_json "$CONFIRM_TEXT")
var overwriteLabel = $(escape_json "$OVERWRITE_LABEL")
var delLabel = $(escape_json "$DEL_BTN")
var pickerPrompt = $(escape_json "$PICKER_PROMPT")
var okLabel = "$OK_LABEL"
var cancelLabel = "$CANCEL_LABEL"

// --- Layout ---
var M = 20  // margin
var LH = 24 // line height
var CW = 460 // content width
var X1 = M
var X2 = X1 + 20 // indented

var views = []
var Y = 0

function label(text, x, bold) {
    var h = LH
    var f = $.NSTextField.alloc.initWithFrame($.NSMakeRect(x, Y, CW - x, h))
    f.stringValue = text
    f.editable = false
    f.bordered = false
    f.drawsBackground = false
    f.font = bold ? $.NSFont.boldSystemFontOfSize(12) : $.NSFont.systemFontOfSize(11)
    f.sizeToFit()
    h = f.frame.size.height + 2
    f.frame = $.NSMakeRect(x, Y, CW - x, h)
    views.push(f)
    Y += h + 2
    return f
}

function checkbox(text, x, tag, checked) {
    var h = LH
    var btn = $.NSButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW - x, h))
    btn.title = text
    btn.setButtonType($.NSSwitchButton)
    btn.tag = tag
    btn.font = $.NSFont.systemFontOfSize(12)
    if (checked) btn.state = $.NSOnState
    views.push(btn)
    Y += h + 2
    return btn
}

function popup(items, x, tag, selIdx) {
    var h = 24
    var pop = $.NSPopUpButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW - x - 20, h))
    pop.removeAllItems()
    for (var i = 0; i < items.length; i++) {
        pop.addItemWithTitle(items[i])
    }
    if (selIdx !== undefined && selIdx < items.length) pop.selectItemAtIndex(selIdx)
    pop.tag = tag
    pop.font = $.NSFont.systemFontOfSize(12)
    views.push(pop)
    Y += h + 4
    return pop
}

function separator() {
    var h = 1
    var box = $.NSBox.alloc.initWithFrame($.NSMakeRect(X1, Y, CW - X1, h))
    box.boxType = $.NSSeparator
    views.push(box)
    Y += 8
}

// --- Build UI from bottom up ---
Y = 6

// Ok/Cancel buttons handled by NSAlert

// Section 4: Delete other printers
if (otherNames.length > 0 && otherNames[0] != "") {
    label(delLabel, X1, true)
    var delBoxes = []
    for (var i = 0; i < otherNames.length; i++) {
        delBoxes.push(checkbox(otherNames[i], X2, 200 + i, false))
    }
    separator()
}

// Section 3: Overwrite
if (conflictName != "") {
    var txt = overwriteLabel + ": " + conflictName
    var chkOverwrite = checkbox(txt, X1, 30, true)
    separator()
}

// Section 2: Location picker (initially hidden)
label(pickerPrompt, X1, true)
var popupLoc = popup(locItems, X2, 20, 0)
popupLoc.hidden = true

// Section 1: Location confirm
var chkConfirm = checkbox(confirmText, X1, 10, true)

// Toggle picker visibility
chkConfirm.action = 'toggleConfirm:'
chkConfirm.target = function() {
    popupLoc.hidden = (chkConfirm.state == $.NSOnState)
}

// --- Build accessory view ---
Y += 10
var totalH = Y
var accessory = $.NSView.alloc.initWithFrame($.NSMakeRect(0, 0, CW, totalH))
for (var i = 0; i < views.length; i++) {
    // Flip Y for Cocoa coordinates
    var v = views[i]
    var r = v.frame
    v.frame = $.NSMakeRect(r.origin.x, totalH - r.origin.y - r.size.height, r.size.width, r.size.height)
    accessory.addSubview(v)
}

// --- Show alert ---
var alert = $.NSAlert.alloc.init
alert.messageText = "Printer Installer"
var info = "Location: " + detectedLoc + " | Printer: " + targetName + " | IP: " + targetIP + " | Model: " + model
alert.informativeText = info
alert.accessoryView = accessory
alert.addButtonWithTitle(okLabel)
alert.addButtonWithTitle(cancelLabel)
alert.window.initialFirstResponder = alert.buttons.objectAtIndex(0)

var response = alert.runModal

// --- Collect results ---
if (response == $.NSAlertFirstButtonReturn) {
    var result = {
        confirm: chkConfirm.state == $.NSOnState,
        location: popupLoc.titleOfSelectedItem.js,
        overwrite: typeof chkOverwrite != 'undefined' ? (chkOverwrite.state == $.NSOnState) : true,
        deletePrinters: []
    }
    if (typeof delBoxes != 'undefined') {
        for (var i = 0; i < delBoxes.length; i++) {
            if (delBoxes[i].state == $.NSOnState) {
                result.deletePrinters.push(delBoxes[i].title.js)
            }
        }
    }
    JSON.stringify(result)
} else {
    ""
}
ENDJXA
)

# --- Parse result ---
if [ -z "$RESULT" ]; then
	exit 0
fi

CONFIRMED=$(echo "$RESULT" | python3 -c "import json,sys; d=json.load(sys.stdin); print('yes' if d['confirm'] else 'no')" 2>/dev/null)
PICKED_LOC=$(echo "$RESULT" | python3 -c "import json,sys; print(json.load(sys.stdin)['location'])" 2>/dev/null)
OVERWRITE=$(echo "$RESULT" | python3 -c "import json,sys; print('yes' if json.load(sys.stdin)['overwrite'] else 'no')" 2>/dev/null)
DELETES=$(echo "$RESULT" | python3 -c "import json,sys; print(','.join(json.load(sys.stdin)['deletePrinters']))" 2>/dev/null)

if [ "$CONFIRMED" = "yes" ]; then
	FINAL_LOC="$DETECTED_LOCATION"
else
	FINAL_LOC="$PICKED_LOC"
fi

echo ""
echo "========== Analysis =========="
echo "Location:   $FINAL_LOC"
echo "Overwrite:  $OVERWRITE"
echo "To delete:  ${DELETES:-none}"
echo "=============================="
