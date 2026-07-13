#!/bin/bash

DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$DIR/printer-installer-darwin"
DRIVERS_DIR="$DIR/../Resources/drivers"
LOG="/tmp/printer-installer-result.log"
STATUS_FILE="/tmp/printer-installer-status.txt"

eval "$("$BINARY" --drivers "$DRIVERS_DIR" --ui-env 2>/dev/null)"

# --- Rosetta ---
if [ "$(uname -m)" = "arm64" ] && ! /usr/bin/arch -x86_64 /bin/ls >/dev/null 2>&1; then
	osascript -e "do shell script \"softwareupdate --install-rosetta --agree-to-license\" with administrator privileges" 2>/dev/null
fi

# --- Shift → admin ---
SHIFT=$(osascript -l JavaScript -e "ObjC.import('Cocoa'); ($.NSEvent.modifierFlags & 131072) != 0 ? '1' : '0'" 2>/dev/null)
if [ "$SHIFT" = "1" ]; then
	osascript -e "do shell script \"'$BINARY' --drivers '$DRIVERS_DIR' --admin > /tmp/printer-installer-result.log 2>&1\" with administrator privileges" 2>/dev/null
	exit 0
fi

# --- Discover ---
DISCOVERED=$("$BINARY" --drivers "$DRIVERS_DIR" --discover 2>/dev/null)
DETECTED_IP=$(echo "$DISCOVERED" | grep "^IP=" | head -1 | cut -d= -f2)
DETECTED_MODEL=$(echo "$DISCOVERED" | grep "^Model=" | head -1 | cut -d= -f2)
DETECTED_LOCATION=$(echo "$DISCOVERED" | grep "^Location=" | head -1 | cut -d= -f2)

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

# --- Location lists ---
LOC_ITEMS_ALL=""
LOC_ITEMS_NODETECT=""
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

# --- Build printer info: name, IP, location ---
PRINTER_INFO_JS=""        # for JXA: {name, ip, loc}
INSTALL_IPS=""             # comma-sep IPs at detected location
if [ -n "$ALL_PRINTER_IPS" ]; then
	INSTALL_IPS="$ALL_PRINTER_IPS"
fi

DELETE_ITEMS=""
PRINTER_MAP=""
PRINTER_IPS=""  # parallel to DELETE_ITEMS: IP for each printer

if [ -n "$ALL_PRINTERS" ]; then
	IFS=',' read -ra PNAMES <<< "$ALL_PRINTERS"
	for pn in "${PNAMES[@]}"; do
		pn=$(echo "$pn" | sed 's/^ *//;s/ *$//')
		[ -z "$pn" ] && continue
		# Get IP from lpstat -v
		PN_IP=$(lpstat -v 2>/dev/null | grep "[^a-zA-Z]$pn" | head -1 | sed -n 's/.*socket:\/\/\([0-9.]*\).*/\1/p')
		[ -z "$PN_IP" ] && PN_IP=$(lpstat -v 2>/dev/null | grep "$pn" | head -1 | sed -n 's/.*:\/\/\([0-9.]*\).*/\1/p')
		[ -z "$PN_IP" ] && PN_IP="?"
		
		PLOC=$("$BINARY" --drivers "$DRIVERS_DIR" --printer-location "$pn" 2>/dev/null || echo "")
		
		# Display format: Printer-BG (30.61.34.29)
		DISPLAY="$pn ($PN_IP)"
		if [ -z "$DELETE_ITEMS" ]; then DELETE_ITEMS="\"$DISPLAY\""
		else DELETE_ITEMS="$DELETE_ITEMS, \"$DISPLAY\""; fi
		if [ -z "$PRINTER_MAP" ]; then PRINTER_MAP="$pn=$PLOC"
		else PRINTER_MAP="$PRINTER_MAP|$pn=$PLOC"; fi
		if [ -z "$PRINTER_IPS" ]; then PRINTER_IPS="$pn=$PN_IP"
		else PRINTER_IPS="$PRINTER_IPS|$pn=$PN_IP"; fi
	done
fi

# --- Escape ---
js_escape() { local s="$1"; s="${s//\\/\\\\}"; s="${s//\"/\\\"}"; echo "\"$s\""; }

# Build location-to-IPs mapping for toggle
LOC_IP_MAP_JS=""
if [ -n "$ALL_LOCATIONS" ]; then
	while IFS=',' read -ra NAMES; do
		for name in "${NAMES[@]}"; do
			name=$(echo "$name" | sed 's/^ *//;s/ *$//')
			[ -z "$name" ] && continue
			RESOLVED_LOC=$("$BINARY" --drivers "$DRIVERS_DIR" --resolve-location "$name" 2>/dev/null)
			LOC_IPS=""
			while IFS= read -r line; do
				case "$line" in IP=*) LOC_IPS="${LOC_IPS}${LOC_IPS:+,}$(echo "$line" | cut -d= -f2-)" ;; esac
			done <<< "$RESOLVED_LOC"
			if [ -z "$LOC_IP_MAP_JS" ]; then LOC_IP_MAP_JS="\"$name\":\"$LOC_IPS\""
			else LOC_IP_MAP_JS="$LOC_IP_MAP_JS, \"$name\":\"$LOC_IPS\""; fi
		done
	done <<< "$ALL_LOCATIONS"
fi

CONFIRM_TEXT=$(echo "$CONFIRM_FMT" | sed "s/%s/$DETECTED_LOCATION/")
CONFIRM_L1=$(echo -e "$CONFIRM_TEXT" | head -1)
CONFIRM_L2=$(echo -e "$CONFIRM_TEXT" | tail -1)
PRINTER_SUMMARY="$DETECTED_NAME"
[ $(echo "$ALL_PRINTER_NAMES" | tr ',' '\n' | wc -l | tr -d ' ') -gt 1 ] && PRINTER_SUMMARY="$ALL_PRINTER_NAMES"

# Build printer-to-location and printer-to-IP JS maps
PRINTER_MAP_JS=""
PRINTER_IP_MAP_JS=""
if [ -n "$PRINTER_MAP" ]; then
	IFS='|' read -ra PMAP <<< "$PRINTER_MAP"
	for m in "${PMAP[@]}"; do
		pn=$(echo "$m" | cut -d= -f1); pl=$(echo "$m" | cut -d= -f2-)
		[ -z "$PRINTER_MAP_JS" ] && PRINTER_MAP_JS="\"$pn\":\"$pl\"" || PRINTER_MAP_JS="$PRINTER_MAP_JS, \"$pn\":\"$pl\""
	done
fi
if [ -n "$PRINTER_IPS" ]; then
	IFS='|' read -ra IPMAP <<< "$PRINTER_IPS"
	for m in "${IPMAP[@]}"; do
		pn=$(echo "$m" | cut -d= -f1); pip=$(echo "$m" | cut -d= -f2-)
		[ -z "$PRINTER_IP_MAP_JS" ] && PRINTER_IP_MAP_JS="\"$pn\":\"$pip\"" || PRINTER_IP_MAP_JS="$PRINTER_IP_MAP_JS, \"$pn\":\"$pip\""
	done
fi

cat > /tmp/printer-installer-ui.jxa <<ENDJXA
ObjC.import('Cocoa')

var locItemsAll = [$LOC_ITEMS_ALL]
var locItemsNoDetect = [$LOC_ITEMS_NODETECT]
var deleteItems = [$DELETE_ITEMS]
var printerMap = {$PRINTER_MAP_JS}
var printerIPMap = {$PRINTER_IP_MAP_JS}
var installIPs = $(js_escape "$ALL_PRINTER_IPS")
var installIPList = installIPs ? installIPs.split(",") : []
var locIPMap = {$LOC_IP_MAP_JS}
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
	views.push(f); Y += LH + 1
}
function ck(s, x, checked, disabled, multiline) {
	var h = multiline ? LH * 2 + 4 : LH + 2
	var b = $.NSButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW - x, h))
	b.title = s; b.setButtonType($.NSSwitchButton)
	b.font = $.NSFont.systemFontOfSize(12)
	if (checked) b.state = $.NSOnState
	if (disabled) b.enabled = false
	views.push(b); Y += h + 1; return b
}
function pp(items, x) {
	if (items.length == 0) return null
	var p = $.NSPopUpButton.alloc.initWithFrame($.NSMakeRect(x, Y, CW - x - 20, 24))
	for (var i = 0; i < items.length; i++) p.addItemWithTitle(items[i])
	p.font = $.NSFont.systemFontOfSize(12)
	views.push(p); Y += 26; return p
}
function hr() {
	var b = $.NSView.alloc.initWithFrame($.NSMakeRect(X1, Y, CW - X1, 1))
	b.wantsLayer = true
	b.layer.backgroundColor = $.NSColor.separatorColor.CGColor
	views.push(b); Y += 6
}

// 1. Location confirm
chkConfirm = ck(confirmText, X1, true, false, true)

// 2. Location picker — overlapping popups
var ppKeep = pp([detectedLoc], X2)
var saveY = Y
var ppPick = pp(locItemsNoDetect, X2)
if (ppPick) { ppPick.frame = ppKeep.frame; ppPick.hidden = true }
var pickerPopup = ppKeep
Y = saveY  // only count one popup

hr()

// 3. Conflict
if (conflictName != "") {
	txt(conflictLabel, X1)
	var conflictPopup = pp([skipLabel, overwriteLabel], X2)
	hr()
}

// 4. Delete
if (deleteItems.length > 0 && deleteItems[0] != "") {
	txt(delPrompt, X1)
	for (var i = 0; i < deleteItems.length; i++) {
		var pname = deleteItems[i]  // "Printer-BG (30.61.34.29)"
		var parts = pname.split(" (")
		var realName = parts[0]
		var pip = printerIPMap[realName] || ""
		// Disable if this printer's IP is in the install IPs for detected location
		var disabled = installIPList.indexOf(pip) >= 0
		delBoxes.push(ck(pname, X2, false, disabled))
	}
}

// Toggle
ObjC.registerSubclass({
	name: "TH",
	methods: {"t:": {types:["void",["id"]], implementation:function(s) {
		var on = (chkConfirm.state == $.NSOnState)
		ppKeep.hidden = !on
		if (ppPick) ppPick.hidden = on
		pickerPopup = on ? ppKeep : (ppPick || ppKeep)
		var curLoc = on ? detectedLoc : (ppPick ? ppPick.titleOfSelectedItem.js : detectedLoc)
		var curIPs = (locIPMap[curLoc] || "").split(",")
		for (var i = 0; i < delBoxes.length; i++) {
			var label = delBoxes[i].title.js
			var parts = label.split(" (")
			var realName = parts[0]
			var pip = printerIPMap[realName] || ""
			delBoxes[i].enabled = (curIPs.indexOf(pip) < 0)
		}
	}}}
})
chkConfirm.target = $.TH.alloc.init
chkConfirm.action = 't:'
// Also fire on popup selection change
if (ppPick) { ppPick.target = chkConfirm.target; ppPick.action = 't:' }

// Assemble
Y += 8
var acc = $.NSView.alloc.initWithFrame($.NSMakeRect(0, 0, CW, Y))
for (var i = 0; i < views.length; i++) {
	var v = views[i], r = v.frame
	v.frame = $.NSMakeRect(r.origin.x, Y - r.origin.y - r.size.height - 4, r.size.width, r.size.height)
	acc.addSubview(v)
}

var line1 = detectedLoc + "  |  " + detectedNames + "  |  IP: " + detectedIP
var line2 = model
var alert = $.NSAlert.alloc.init
alert.messageText = title
alert.informativeText = line1 + "\n" + line2
alert.accessoryView = acc
alert.addButtonWithTitle("$OK_LABEL")
alert.addButtonWithTitle("$CANCEL_LABEL")

if (alert.runModal != $.NSAlertFirstButtonReturn) { "CANCEL" } else {
	var lines = []
	lines.push("CONFIRM=" + (chkConfirm.state == $.NSOnState ? "true" : "false"))
	lines.push("LOCATION=" + (pickerPopup.titleOfSelectedItem.js || detectedLoc.js))
	if (typeof conflictPopup != 'undefined') {
		lines.push("OVERWRITE=" + (conflictPopup.indexOfSelectedItem == 1 ? "true" : "false"))
	} else { lines.push("OVERWRITE=false") }
	for (var i = 0; i < delBoxes.length; i++) {
		if (delBoxes[i].state == $.NSOnState) {
			var label = delBoxes[i].title.js
			var parts = label.split(" (")
			lines.push("DELETE=" + parts[0])  // send only the name
		}
	}
	lines.join("\n")
}
ENDJXA

RESULT=$(osascript -l JavaScript /tmp/printer-installer-ui.jxa 2>&1)

if [ "$RESULT" = "CANCEL" ] || [ -z "$RESULT" ]; then exit 0; fi

CONFIRMED=$(echo "$RESULT" | grep "^CONFIRM=" | cut -d= -f2)
PICKED_LOC=$(echo "$RESULT" | grep "^LOCATION=" | cut -d= -f2-)
DO_OVERWRITE=$(echo "$RESULT" | grep "^OVERWRITE=" | cut -d= -f2)
TO_DELETE=$(echo "$RESULT" | grep "^DELETE=" | cut -d= -f2- | tr '\n' ',' | sed 's/,$//')

if [ "$CONFIRMED" = "true" ]; then CHOSEN_LOC="$DETECTED_LOCATION"
else CHOSEN_LOC="$PICKED_LOC"; fi

# --- Install printers for chosen location ---
if [ -n "$CHOSEN_LOC" ]; then
	: > "$LOG"
	ERR=$(osascript -e "do shell script \"'$BINARY' --drivers '$DRIVERS_DIR' --location '$CHOSEN_LOC' > '$LOG' 2>&1\" with administrator privileges with prompt \"$ADMIN_INSTALL_PROMPT\"" 2>&1)
	EXIT_CODE=$?
	if [ $EXIT_CODE -ne 0 ]; then
		case "$ERR" in *[Cc]ancel*|*-128*|*not\ authorized*) exit 0 ;; esac
		ERR_MSG=$(head -20 "$LOG" 2>/dev/null | tr -d '"' || echo "Unknown error")
		osascript -e "display dialog \"$FAIL_PREFIX\\n$ERR_MSG\" buttons {\"$OK_LABEL\"} default button \"$OK_LABEL\" with icon stop" 2>/dev/null
		exit 1
	fi
fi

# --- Delete selected printers ---
if [ -n "$TO_DELETE" ]; then
	CHOSEN_NAME=""
	if [ -n "$CHOSEN_LOC" ]; then
		RESOLVED=$("$BINARY" --drivers "$DRIVERS_DIR" --resolve-location "$CHOSEN_LOC" 2>/dev/null)
		CHOSEN_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
	fi
	: > /tmp/printer-installer-delete.txt
	IFS=',' read -ra DLIST <<< "$TO_DELETE"
	for d in "${DLIST[@]}"; do
		d=$(echo "$d" | sed 's/^ *//;s/ *$//')
		[ -z "$d" ] && continue
		[ "$d" = "$CHOSEN_NAME" ] && continue
		echo "$d" >> /tmp/printer-installer-delete.txt
	done
	if [ -s /tmp/printer-installer-delete.txt ]; then
		osascript -e "do shell script \"'$BINARY' --delete-printers-file /tmp/printer-installer-delete.txt > '$LOG' 2>&1\" with administrator privileges with prompt \"$ADMIN_DELETE_PROMPT\"" 2>/dev/null
	fi
	rm -f /tmp/printer-installer-delete.txt
fi

# --- Success ---
RAW_MSG=""
[ -s "$STATUS_FILE" ] && RAW_MSG=$(tr -d '"' < "$STATUS_FILE")
osascript -e "display dialog \"✅ $RAW_MSG\" buttons {\"$OK_LABEL\"} default button \"$OK_LABEL\" giving up after 5" 2>/dev/null
