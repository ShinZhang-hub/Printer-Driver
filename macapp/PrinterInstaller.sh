#!/bin/bash

DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$DIR/printer-installer-darwin"
DRIVERS_DIR="$DIR/../Resources/drivers"
LOG="/tmp/printer-installer-result.log"
STATUS_FILE="/tmp/printer-installer-status.txt"

eval "$("$BINARY" --drivers "$DRIVERS_DIR" --ui-env 2>/dev/null)"

# --- Clean stale status file ---
rm -f "$STATUS_FILE" 2>/dev/null

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

# --- Detect location by subnet (no SNMP, config only) ---
DISCOVERED=$("$BINARY" --drivers "$DRIVERS_DIR" --discover --no-snmp 2>/dev/null)
DETECTED_IP=$(echo "$DISCOVERED" | grep "^IP=" | head -1 | cut -d= -f2)
DETECTED_MODEL=$(echo "$DISCOVERED" | grep "^Model=" | head -1 | cut -d= -f2)
DETECTED_LOCATION=$(echo "$DISCOVERED" | grep "^Location=" | head -1 | cut -d= -f2)

ALL_PRINTER_NAMES=""
ALL_PRINTER_IPS=""
DETECTED_NAME=""
if [ -n "$DETECTED_LOCATION" ]; then
	RESOLVED=$("$BINARY" --drivers "$DRIVERS_DIR" --no-snmp --resolve-location "$DETECTED_LOCATION" 2>/dev/null)
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
	F2=true
	while IFS=',' read -ra NAMES; do
		for name in "${NAMES[@]}"; do
			name=$(echo "$name" | sed 's/^ *//;s/ *$//')
			[ -z "$name" ] && continue
			if [ "$name" != "$DETECTED_LOCATION" ]; then
				[ "$F2" = true ] && LOC_ITEMS_NODETECT="\"$name\"" || LOC_ITEMS_NODETECT="$LOC_ITEMS_NODETECT, \"$name\""
				F2=false
			fi
		done
	done <<< "$ALL_LOCATIONS"
fi

# --- Build printer info: name, IP ---
PRINTER_INFO_JS=""
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
		
		DISPLAY="$pn ($PN_IP)"
		if [ -z "$DELETE_ITEMS" ]; then DELETE_ITEMS="\"$DISPLAY\""
		else DELETE_ITEMS="$DELETE_ITEMS, \"$DISPLAY\""; fi
		if [ -z "$PRINTER_IPS" ]; then PRINTER_IPS="$pn=$PN_IP"
		else PRINTER_IPS="$PRINTER_IPS|$pn=$PN_IP"; fi
	done
fi

# --- Escape ---
js_escape() { local s="$1"; s="${s//\\/\\\\}"; s="${s//\"/\\\"}"; echo "\"$s\""; }

# Build location-to-IPs mapping for toggle
LOC_IP_MAP_JS=""
# Build location-to-IPs mapping for toggle + conflict map
LOC_IP_MAP_JS=""
CONFLICT_MAP_JS=""
if [ -n "$ALL_LOCATIONS" ]; then
	while IFS=',' read -ra NAMES; do
		for name in "${NAMES[@]}"; do
			name=$(echo "$name" | sed 's/^ *//;s/ *$//')
			[ -z "$name" ] && continue
			RESOLVED_LOC=$("$BINARY" --drivers "$DRIVERS_DIR" --no-snmp --resolve-location "$name" 2>/dev/null)
			LOC_IPS=""
			while IFS= read -r line; do
				case "$line" in IP=*) LOC_IPS="${LOC_IPS}${LOC_IPS:+,}$(echo "$line" | cut -d= -f2-)" ;; esac
			done <<< "$RESOLVED_LOC"
			if [ -z "$LOC_IP_MAP_JS" ]; then LOC_IP_MAP_JS="\"$name\":\"$LOC_IPS\""
			else LOC_IP_MAP_JS="$LOC_IP_MAP_JS, \"$name\":\"$LOC_IPS\""; fi
			# Check conflict: does any printer exist at these IPs?
			CONFLICT_FOUND=false
			for ip in $(echo "$LOC_IPS" | tr ',' ' '); do
				[ -z "$ip" ] && continue
				EXIST=$("$BINARY" --drivers "$DRIVERS_DIR" --printer-at-ip "$ip" 2>/dev/null)
				if [ -n "$EXIST" ]; then CONFLICT_FOUND=true; break; fi
			done
			if [ -z "$CONFLICT_MAP_JS" ]; then CONFLICT_MAP_JS="\"$name\":$CONFLICT_FOUND"
			else CONFLICT_MAP_JS="$CONFLICT_MAP_JS, \"$name\":$CONFLICT_FOUND"; fi
		done
	done <<< "$ALL_LOCATIONS"
fi

CONFIRM_TEXT=$(echo "$CONFIRM_FMT" | sed "s/%s/$DETECTED_LOCATION/")
PRINTER_SUMMARY="$DETECTED_NAME"
[ $(echo "$ALL_PRINTER_NAMES" | tr ',' '\n' | wc -l | tr -d ' ') -gt 1 ] && PRINTER_SUMMARY="$ALL_PRINTER_NAMES"

# Build printer-to-IP JS map
PRINTER_IP_MAP_JS=""
if [ -n "$PRINTER_IPS" ]; then
	IFS='|' read -ra IPMAP <<< "$PRINTER_IPS"
	for m in "${IPMAP[@]}"; do
		pn=$(echo "$m" | cut -d= -f1); pip=$(echo "$m" | cut -d= -f2-)
		[ -z "$PRINTER_IP_MAP_JS" ] && PRINTER_IP_MAP_JS="\"$pn\":\"$pip\"" || PRINTER_IP_MAP_JS="$PRINTER_IP_MAP_JS, \"$pn\":\"$pip\""
	done
fi

cat > /tmp/printer-installer-ui.jxa <<ENDJXA
ObjC.import('Cocoa')

// Force dialog to front
$.NSRunningApplication.currentApplication.activateWithOptions($.NSApplicationActivateIgnoringOtherApps)

var locItemsNoDetect = [$LOC_ITEMS_NODETECT]
var deleteItems = [$DELETE_ITEMS]
var printerIPMap = {$PRINTER_IP_MAP_JS}
var installIPs = $(js_escape "$ALL_PRINTER_IPS")
var installIPList = installIPs ? installIPs.split(",") : []
var locIPMap = {$LOC_IP_MAP_JS}
var conflictMap = {$CONFLICT_MAP_JS}
var detectedLoc = $(js_escape "$DETECTED_LOCATION")
var detectedNames = $(js_escape "$PRINTER_SUMMARY")
var detectedIP = $(js_escape "$DETECTED_IP")
var conflictName = $(js_escape "$EXISTING_NAME")

var title = $(js_escape "$TITLE")
var confirmText = $(js_escape "$CONFIRM_TEXT")
var overwriteLabel = $(js_escape "$OVERWRITE_LABEL")
var skipLabel = $(js_escape "$SKIP_BTN")
var pickerPrompt = $(js_escape "$PICKER_PROMPT")
var conflictLabel = $(js_escape "$CONFLICT_LABEL")
var existPromptFmt = $(js_escape "$EXISTING_PRINTERS")

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
chkConfirm = ck(confirmText, X1, true, false, false)

// 2. Location picker — hidden when confirmed, shown when unchecked
var pickerPopup = pp(locItemsNoDetect, X2)
pickerPopup.hidden = true

hr()

// 3. Conflict (always show, enabled only when printers exist at chosen location IPs)
txt(conflictLabel, X1)
var conflictPopup = pp([skipLabel, overwriteLabel], X2)
var hasConflict = conflictMap[detectedLoc] === true
conflictPopup.enabled = hasConflict
hr()

// 4. Delete
if (deleteItems.length > 0 && deleteItems[0] != "") {
	// Add fake test printers for scroll testing
	deleteItems.push("Fake-A (10.0.0.1)", "Fake-B (10.0.0.2)", "Fake-C (10.0.0.3)", "Fake-D (10.0.0.4)", "Fake-E (10.0.0.5)", "Fake-F (10.0.0.6)")
	txt(existPromptFmt.replace("%d", deleteItems.length), X1)
	var delViews = []
	for (var i = 0; i < deleteItems.length; i++) {
		var pname = deleteItems[i]
		var parts = pname.split(" (")
		var realName = parts[0]
		var pip = printerIPMap[realName] || ""
		var disabled = installIPList.indexOf(pip) >= 0
		var cb = ck(pname, X2, false, disabled)
		delBoxes.push(cb)
		delViews.push(cb)
	}
	// Remove checkboxes from views (they were added by ck)
	for (var j = 0; j < delViews.length; j++) {
		var idx = views.lastIndexOf(delViews[j])
		if (idx >= 0) views.splice(idx, 1)
	}
	// Scroll container — fixed height, immediately below label
	var itemH = LH + 2
	var maxVis = Math.min(deleteItems.length, 5)
	var scrollH = maxVis * itemH + 4
	var delH = delViews.length * itemH + 2
	var delContainer = $.NSView.alloc.initWithFrame($.NSMakeRect(0, 0, CW - X2 - 20, delH))
	for (var j = 0; j < delViews.length; j++) {
		var dv = delViews[j]
		dv.frame = $.NSMakeRect(0, delH - (j + 1) * itemH - 2, dv.frame.size.width, itemH)
		delContainer.addSubview(dv)
	}
	var scroll = $.NSScrollView.alloc.initWithFrame($.NSMakeRect(X2, Y, CW - X2 - 20, scrollH))
	scroll.hasVerticalScroller = true
	scroll.documentView = delContainer
	scroll.borderType = $.NSNoBorder
	views.push(scroll)
	Y += scrollH + 4
}
}

// Toggle
ObjC.registerSubclass({
	name: "TH",
	methods: {"t:": {types:["void",["id"]], implementation:function(s) {
		var on = (chkConfirm.state == $.NSOnState)
		pickerPopup.hidden = on
		var curLoc = on ? detectedLoc : pickerPopup.titleOfSelectedItem.js
		conflictPopup.enabled = (conflictMap[curLoc] === true)
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
pickerPopup.target = chkConfirm.target; pickerPopup.action = 't:'

// Assemble
Y += 8
var acc = $.NSView.alloc.initWithFrame($.NSMakeRect(0, 0, CW, Y))
for (var i = 0; i < views.length; i++) {
	var v = views[i], r = v.frame
	v.frame = $.NSMakeRect(r.origin.x, Y - r.origin.y - r.size.height - 4, r.size.width, r.size.height)
	acc.addSubview(v)
}

var line1 = detectedLoc + "  |  " + detectedNames + "  |  IP: " + detectedIP
var alert = $.NSAlert.alloc.init
alert.messageText = title
alert.informativeText = line1
var iconPath = $(js_escape "$DIR/../Resources/AppIcon.icns")
var icon = $.NSImage.alloc.initWithContentsOfFile(iconPath)
if (icon) alert.icon = icon
alert.accessoryView = acc
alert.addButtonWithTitle("$OK_LABEL")
alert.addButtonWithTitle("$CANCEL_LABEL")
alert.window.level = $.NSFloatingWindowLevel

if (alert.runModal != $.NSAlertFirstButtonReturn) { "CANCEL" } else {
	var lines = []
	lines.push("CONFIRM=" + (chkConfirm.state == $.NSOnState ? "true" : "false"))
	lines.push("LOCATION=" + (pickerPopup.titleOfSelectedItem.js || detectedLoc.js))
	lines.push("OVERWRITE=" + (conflictPopup.indexOfSelectedItem == 1 ? "true" : "false"))
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

# --- Build combined install+delete script ---
SKIP_MSG=""
COMBINED_SCRIPT=""

if [ -n "$CHOSEN_LOC" ]; then
	# Resolve CHOSEN location's printers (not detected)
	CHOSEN_RESOLVED=$("$BINARY" --drivers "$DRIVERS_DIR" --resolve-location "$CHOSEN_LOC" 2>/dev/null)
	CHOSEN_IPS=""
	CHOSEN_NAMES=""
	while IFS= read -r line; do
		case "$line" in
			IP=*) CHOSEN_IPS="${CHOSEN_IPS}${CHOSEN_IPS:+,}$(echo "$line" | cut -d= -f2-)" ;;
			Name=*) CHOSEN_NAMES="${CHOSEN_NAMES}${CHOSEN_NAMES:+,}$(echo "$line" | cut -d= -f2-)" ;;
		esac
	done <<< "$CHOSEN_RESOLVED"

	if [ "$DO_OVERWRITE" = "false" ]; then
		SKIP_ALL=true
		for ip in $(echo "$CHOSEN_IPS" | tr ',' ' '); do
			[ -z "$ip" ] && continue
			EXIST=$("$BINARY" --drivers "$DRIVERS_DIR" --printer-at-ip "$ip" 2>/dev/null)
			if [ -z "$EXIST" ]; then SKIP_ALL=false; break; fi
		done
		if [ "$SKIP_ALL" = true ]; then
			SKIP_MSG=$(echo "$SKIP_INSTALL_MSG" | sed "s/%s/$CHOSEN_NAMES/")
		else
			COMBINED_SCRIPT="'$BINARY' --drivers '$DRIVERS_DIR' --location '$CHOSEN_LOC' > '$LOG' 2>&1"
		fi
	else
		COMBINED_SCRIPT="'$BINARY' --drivers '$DRIVERS_DIR' --location '$CHOSEN_LOC' > '$LOG' 2>&1"
	fi
fi
if [ -n "$TO_DELETE" ]; then
	CHOSEN_FIRST_NAME=$(echo "$CHOSEN_NAMES" | cut -d, -f1)
	: > /tmp/printer-installer-delete.txt
	IFS=',' read -ra DLIST <<< "$TO_DELETE"
	for d in "${DLIST[@]}"; do
		d=$(echo "$d" | sed 's/^ *//;s/ *$//')
		[ -z "$d" ] && continue
		[ "$d" = "$CHOSEN_FIRST_NAME" ] && continue
		echo "$d" >> /tmp/printer-installer-delete.txt
	done
	if [ -s /tmp/printer-installer-delete.txt ]; then
		COMBINED_SCRIPT="$COMBINED_SCRIPT"$'\n'"'$BINARY' --delete-printers-file /tmp/printer-installer-delete.txt >> '$LOG' 2>&1"
	fi
fi

if [ -n "$COMBINED_SCRIPT" ]; then
	: > "$LOG"
	COMBINED_SCRIPT="rm -f '$STATUS_FILE' 2>/dev/null"$'\n'"$COMBINED_SCRIPT"
	ERR=$(osascript -e "do shell script \"$COMBINED_SCRIPT\" with administrator privileges with prompt \"$ADMIN_INSTALL_PROMPT\"" 2>&1)
	EXIT_CODE=$?
	rm -f /tmp/printer-installer-delete.txt
	if [ $EXIT_CODE -ne 0 ]; then
		case "$ERR" in *[Cc]ancel*|*-128*|*not\ authorized*) exit 0 ;; esac
		ERR_MSG=$(head -20 "$LOG" 2>/dev/null | tr -d '"' || echo "Unknown error")
		osascript -e "display dialog \"$FAIL_PREFIX\\n$ERR_MSG\" buttons {\"$OK_LABEL\"} default button \"$OK_LABEL\" with icon stop" 2>/dev/null
		exit 1
	fi
	SCRIPT_RAN=true
fi

# --- Success (unified dialog for all outcomes) ---
SUCCESS_MSG=""

if [ "$SCRIPT_RAN" = true ] || [ -n "$SKIP_MSG" ]; then
	if [ -n "$SKIP_MSG" ]; then
		SUCCESS_MSG="$SKIP_MSG"
	fi
	if [ "$SCRIPT_RAN" = true ]; then
		if [ "$DO_OVERWRITE" = "true" ]; then
			[ -n "$SUCCESS_MSG" ] && SUCCESS_MSG="$SUCCESS_MSG"$'\n'
			SUCCESS_MSG="${SUCCESS_MSG}$(echo "$OVERWRITTEN_MSG" | sed "s/%s/$CHOSEN_NAMES/")"
		elif [ -z "$SKIP_MSG" ]; then
			SUCCESS_MSG="$(echo "$INSTALLED_LABEL" | sed "s/%s/$CHOSEN_NAMES/")"
		fi
	fi
fi

# Append delete results if any
if [ -n "$TO_DELETE" ]; then
	DEL_NAMES=""
	IFS=',' read -ra DLIST2 <<< "$TO_DELETE"
	for d in "${DLIST2[@]}"; do
		d=$(echo "$d" | sed 's/^ *//;s/ *$//')
		[ -z "$d" ] && continue
		[ "$d" = "$CHOSEN_FIRST_NAME" ] && continue
		[ -z "$DEL_NAMES" ] && DEL_NAMES="$d" || DEL_NAMES="$DEL_NAMES, $d"
	done
	if [ -n "$DEL_NAMES" ]; then
		REMOVE_LINE="$(echo "$REMOVED_MSG" | sed "s/%s/$DEL_NAMES/")"
		if [ -z "$SUCCESS_MSG" ]; then
			SUCCESS_MSG="$REMOVE_LINE"
		else
			SUCCESS_MSG="$SUCCESS_MSG"$'\n\n'"$REMOVE_LINE"
		fi
	fi
fi

[ -n "$SUCCESS_MSG" ] && osascript -e "display dialog \"$SUCCESS_MSG\" buttons {\"$OK_LABEL\"} default button \"$OK_LABEL\" giving up after 5" 2>/dev/null
