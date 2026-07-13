#!/bin/bash

DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$DIR/printer-installer-darwin"
DRIVERS_DIR="$DIR/../Resources/drivers"
DRVARG="--drivers '$DRIVERS_DIR'"

eval "$("$BINARY" $DRVARG --ui-env 2>/dev/null)"

# --- Rosetta installation for Apple Silicon (silent, forced) ---
if [ "$(uname -m)" = "arm64" ] && ! /usr/bin/arch -x86_64 /bin/ls >/dev/null 2>&1; then
	osascript -e "do shell script \"softwareupdate --install-rosetta --agree-to-license\" with administrator privileges" 2>/dev/null
fi

# --- Shift key → admin panel ---
SHIFT=$(osascript -l JavaScript -e "ObjC.import('Cocoa'); ($.NSEvent.modifierFlags & 131072) != 0 ? '1' : '0'" 2>/dev/null)
if [ "$SHIFT" = "1" ]; then
	osascript -e "do shell script \"'$BINARY' $DRVARG --admin > '/tmp/printer-installer-result.log' 2>&1\" with administrator privileges with prompt \"$ADMIN_INSTALL_PROMPT\""
	exit 0
fi

# --- Step 1: Discover printer ---
DISCOVERED=$("$BINARY" $DRVARG --discover 2>/dev/null)
DETECTED_IP=$(echo "$DISCOVERED" | grep "^IP=" | head -1 | cut -d= -f2)
DETECTED_MODEL=$(echo "$DISCOVERED" | grep "^Model=" | head -1 | cut -d= -f2)
DETECTED_LOCATION=$(echo "$DISCOVERED" | grep "^Location=" | head -1 | cut -d= -f2)

# --- Step 2: Resolve location ---
LOCATION_ARG=""
INSTALL_NAME=""
INSTALL_IP=""

if [ -n "$DETECTED_LOCATION" ]; then
	RESOLVED=$("$BINARY" $DRVARG --resolve-location "$DETECTED_LOCATION" 2>/dev/null)
	INSTALL_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
	INSTALL_IP=$(echo "$RESOLVED" | grep "^IP=" | head -1 | cut -d= -f2)
	LOCATION_ARG="--location '$DETECTED_LOCATION'"
fi

if [ -n "$INSTALL_IP" ]; then
	IP="$INSTALL_IP"
	NAME="$INSTALL_NAME"
else
	IP="$DETECTED_IP"
	NAME="$DETECTED_MODEL"
fi

# --- Step 3: Check for existing printer at same IP ---
EXISTING_NAME=""
if [ -n "$IP" ]; then
	EXISTING_NAME=$("$BINARY" $DRVARG --printer-at-ip "$IP" 2>/dev/null)
fi

# --- Step 4: List all locations for picker ---
ALL_LOCATIONS=$("$BINARY" $DRVARG --list-locations 2>/dev/null)

# --- Step 5: Build summary text ---
SUMMARY=""

# Location
if [ -n "$DETECTED_LOCATION" ]; then
	SUMMARY="$SUMMARY
DetectedLocation: $DETECTED_LOCATION"
else
	SUMMARY="$SUMMARY
DetectedLocation: (none)"
fi

# Printer
SUMMARY="$SUMMARY
PrinterName: $NAME
PrinterIP: $IP
Model: $DETECTED_MODEL"

# Conflict
if [ -n "$EXISTING_NAME" ]; then
	SUMMARY="$SUMMARY
⚠ Existing: $EXISTING_NAME (same IP)"
else
	SUMMARY="$SUMMARY
Existing: none (new install)"
fi

# Other locations
if [ -n "$ALL_LOCATIONS" ]; then
	SUMMARY="$SUMMARY
OtherLocations: $ALL_LOCATIONS"
fi

# --- Step 6: Build checkbox actions ---
ITEMS=""
FIRST=true
add_item() {
	local val="$1"
	if [ "$FIRST" = true ]; then
		ITEMS="\"$val\""
		FIRST=false
	else
		ITEMS="$ITEMS, \"$val\""
	fi
}

if [ -n "$EXISTING_NAME" ]; then
	add_item "Overwrite: $EXISTING_NAME"
fi
add_item "Set as default printer"

# --- Step 7: Show single dialog with summary + checkboxes ---
osascript 2>/dev/null <<ENDOSA
set summaryText to "$(echo "$SUMMARY" | sed 's/\"/\\\"/g')"

set actions to choose from list {$ITEMS} with prompt summaryText with title "Printer Installer" with multiple selections allowed with empty selection allowed
if actions is false then return

set AppleScript's text item delimiters to linefeed
set resultText to summaryText & return & return & "--- Selected ---" & return & (actions as string)
display dialog resultText buttons {"$OK_LABEL"} default button "$OK_LABEL" with title "Printer Installer - Analysis"
ENDOSA
