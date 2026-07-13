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

# --- Build prompt ---
PROMPT="========================================================================\n"
PROMPT="${PROMPT}Location: ${DETECTED_LOCATION:-Unknown}\n"
PROMPT="${PROMPT}Printer:  ${TARGET_NAME}\n"
PROMPT="${PROMPT}IP:       ${TARGET_IP:-Unknown}\n"
PROMPT="${PROMPT}Model:    ${DETECTED_MODEL}\n"
[ -n "$EXISTING_NAME" ] && PROMPT="${PROMPT}Conflict: ${EXISTING_NAME} (same IP)\n"
PROMPT="${PROMPT}========================================================================\n"
PROMPT="${PROMPT}Select items below (checkboxes):"

# --- Build checkboxes ---
ITEMS=""
add() {
	if [ -z "$ITEMS" ]; then ITEMS="\"$1\""; else ITEMS="$ITEMS, \"$1\""; fi
}

# 1. Location confirm
[ -n "$DETECTED_LOCATION" ] && add "$CONFIRM_TEXT"

# 2. Overwrite
[ -n "$EXISTING_NAME" ] && add "$OVERWRITE_LABEL: $EXISTING_NAME"

# 4. Delete printers
if [ -n "$ALL_PRINTERS" ]; then
	IFS=',' read -ra PNAMES <<< "$ALL_PRINTERS"
	for pn in "${PNAMES[@]}"; do
		pn=$(echo "$pn" | sed 's/^ *//;s/ *$//')
		[ -z "$pn" ] && continue
		[ "$pn" = "$EXISTING_NAME" ] && continue
		[ "$pn" = "$TARGET_NAME" ] && continue
		add "   $DEL_BTN: $pn"
	done
fi

# --- Show dialog ---
if [ -z "$ITEMS" ]; then
	echo "No options to show."
	exit 0
fi

RESULT=$(osascript 2>/dev/null -e "
set thePrompt to \"$(echo -e "$PROMPT" | sed 's/"/\\"/g')\"
set theItems to {$ITEMS}

set selected to choose from list theItems with prompt thePrompt with title \"Printer Installer\" with multiple selections allowed with empty selection allowed
if selected is false then return \"\"
set AppleScript's text item delimiters to \", \"
return selected as string
")

[ -z "$RESULT" ] && exit 0

# --- Location picker: show if confirm NOT checked ---
LOCATION_CONFIRMED=false
if echo "$RESULT" | grep -q "$CONFIRM_TEXT"; then
	LOCATION_CONFIRMED=true
	TARGET_LOCATION="$DETECTED_LOCATION"
else
	LOC_ITEMS=""
	FIRST=true
	while IFS=',' read -ra NAMES; do
		for name in "${NAMES[@]}"; do
			name=$(echo "$name" | sed 's/^ *//;s/ *$//')
			[ -z "$name" ] && continue
			[ "$FIRST" = true ] && LOC_ITEMS="\"$name\"" || LOC_ITEMS="$LOC_ITEMS, \"$name\""
			FIRST=false
		done
	done <<< "$ALL_LOCATIONS"

	[ -n "$LOC_ITEMS" ] && PICKED=$(osascript -e "set selected to choose from list {$LOC_ITEMS} with prompt \"$PICKER_PROMPT\"" -e "if selected is false then return \"\"" -e "return selected as string" 2>/dev/null)

	if [ -n "$PICKED" ]; then
		TARGET_LOCATION="$PICKED"
	else
		exit 0
	fi
fi

# Resolve final location
RESOLVED=$("$BINARY" $DRVARG --resolve-location "$TARGET_LOCATION" 2>/dev/null)
TARGET_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
TARGET_IP=$(echo "$RESOLVED" | grep "^IP=" | head -1 | cut -d= -f2)

# --- Result ---
echo ""
echo "========== Analysis =========="
echo "Location: $TARGET_LOCATION"
echo "Printer:  $TARGET_NAME"
echo "IP:       $TARGET_IP"
echo "Model:    $DETECTED_MODEL"
echo ""
echo "Selected:"
echo "$RESULT" | tr ',' '\n' | sed 's/^/  - /'
echo "=============================="
