#!/bin/bash

DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$DIR/printer-installer-darwin"
DRIVERS_DIR="$DIR/../Resources/drivers"
LOG="/tmp/printer-installer-result.log"
STATUS_FILE="/tmp/printer-installer-status.txt"
MSG_FILE="/tmp/printer-installer-dialog-msg.txt"
DRVARG="--drivers '$DRIVERS_DIR'"

# --- Load all UI strings from binary (language detection in Go) ---
eval "$("$BINARY" $DRVARG --ui-env 2>/dev/null)"

# --- Dialog helpers ---
write_msg() {
	echo -e "$1" > "$MSG_FILE"
}

osascript_dialog() {
	local buttons="$1"
	local default="$2"
	local extra="${3:-}"
	osascript 2>/dev/null <<ENDOSA
set msg to do shell script "cat '$MSG_FILE'"
display dialog msg buttons {$buttons} default button "$default" $extra
return button returned of result
ENDOSA
}

show_dialog() {
	local msg=$(echo "$1" | tr -d '"')
	local icon="${2:-note}"
	local giving_up="${3:-}"
	write_msg "$msg"
	if [ -n "$giving_up" ]; then
		osascript_dialog "\"$OK_LABEL\"" "$OK_LABEL" "with icon $icon giving up after $giving_up"
	else
		osascript_dialog "\"$OK_LABEL\"" "$OK_LABEL" "with icon $icon"
	fi
}

show_delete_dialog() {
	local OTHER_NAMES="$1"
	local INSTALLED_NAME="$2"

	local ITEMS=""
	local FIRST=true
	while IFS= read -r name; do
		name=$(echo "$name" | sed 's/^ *//;s/ *$//')
		if [ -z "$name" ] || [ "$name" = "$INSTALLED_NAME" ]; then
			continue
		fi
		LOCATION=$("$BINARY" $DRVARG --printer-location "$name" 2>/dev/null)
		if [ -n "$LOCATION" ]; then
			DISPLAY_NAME="$LOCATION: $name"
		else
			DISPLAY_NAME="$name"
		fi
		ESCAPED=$(echo "$DISPLAY_NAME" | sed 's/"/\\"/g')
		if [ "$FIRST" = true ]; then
			ITEMS="\"$ESCAPED\""
			FIRST=false
		else
			ITEMS="$ITEMS, \"$ESCAPED\""
		fi
	done <<< "$(echo "$OTHER_NAMES" | tr ',' '\n')"

	[ -z "$ITEMS" ] && return

	local SELECT=false
	local result=$(osascript -e "display dialog \"$DEL_MSG\" buttons {\"$DEL_BTN\", \"$SKIP_BTN\"} default button \"$SKIP_BTN\" cancel button \"$SKIP_BTN\" giving up after 10" -e "return button returned of result" 2>/dev/null)
	[ "$result" = "$DEL_BTN" ] && SELECT=true

	if [ "$SELECT" = true ]; then
		local SCRIPT="set selected to choose from list {$ITEMS} with prompt \"$CHOOSE_PROMPT\" with multiple selections allowed
if selected is false then return \"\"
set AppleScript's text item delimiters to linefeed
return selected as string"
		local RESULT=$(osascript -e "$SCRIPT" 2>/dev/null)
		[ -z "$RESULT" ] && return

		local RESULT_CLEAN=$(echo "$RESULT" | tr -d '"')
		# Strip "Location: " prefix to get actual printer names
		echo "$RESULT_CLEAN" | while IFS= read -r line; do
			case "$line" in
				*": "*) echo "${line#*: }" ;;
				*) echo "$line" ;;
			esac
		done > /tmp/printer-installer-delete.txt
		osascript -e "do shell script \"'$BINARY' --delete-printers-file /tmp/printer-installer-delete.txt > '$LOG' 2>&1\" with administrator privileges with prompt \"$ADMIN_DELETE_PROMPT\"" >/dev/null 2>&1
		[ $? -ne 0 ] && rm -f /tmp/printer-installer-delete.txt && return
		rm -f /tmp/printer-installer-delete.txt
		local DELETED=$(echo "$RESULT_CLEAN" | tr '\n' ', ' | sed 's/[, ]*$//')
		show_dialog "$DELETED_PREFIX\n$DELETED"
	fi
}

# --- Shift key → admin panel ---
SHIFT=$(osascript -l JavaScript -e "ObjC.import('Cocoa'); ($.NSEvent.modifierFlags & 131072) != 0 ? '1' : '0'" 2>/dev/null)
if [ "$SHIFT" = "1" ]; then
	osascript -e "do shell script \"'$BINARY' $DRVARG --admin > '$LOG' 2>&1\" with administrator privileges with prompt \"$ADMIN_INSTALL_PROMPT\""
	exit 0
fi

# --- Rosetta installation for Apple Silicon (silent, forced) ---
if [ "$(uname -m)" = "arm64" ] && ! /usr/bin/arch -x86_64 /bin/true 2>/dev/null; then
	osascript -e "do shell script \"softwareupdate --install-rosetta --agree-to-license\" with administrator privileges" 2>/dev/null
fi

# --- Step 1: Discover printer (no admin) ---
DISCOVERED=$("$BINARY" $DRVARG --discover 2>/dev/null)
DETECTED_IP=$(echo "$DISCOVERED" | grep "^IP=" | head -1 | cut -d= -f2)
DETECTED_MODEL=$(echo "$DISCOVERED" | grep "^Model=" | head -1 | cut -d= -f2)
DETECTED_LOCATION=$(echo "$DISCOVERED" | grep "^Location=" | head -1 | cut -d= -f2)

# --- Step 2: Location confirmation UI ---
LOCATION_ARG=""

if [ -n "$DETECTED_LOCATION" ]; then
	confirm_msg=$(printf "$CONFIRM_FMT" "$DETECTED_LOCATION")
	write_msg "$confirm_msg"

	RESULT=$(osascript_dialog "\"$YES_LABEL\", \"$NO_LABEL\"" "$YES_LABEL")
	if [ "$RESULT" = "$YES_LABEL" ]; then
		LOCATION_ARG="--location '$DETECTED_LOCATION'"
		RESOLVED=$("$BINARY" $DRVARG --resolve-location "$DETECTED_LOCATION" 2>/dev/null)
		INSTALL_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
		INSTALL_IP=$(echo "$RESOLVED" | grep "^IP=" | head -1 | cut -d= -f2)
	fi
fi

# --- Step 3: Location picker (if not confirmed or no location detected) ---
if [ -z "$LOCATION_ARG" ]; then
	LOCATIONS=$("$BINARY" $DRVARG --list-locations 2>/dev/null)
	if [ -n "$LOCATIONS" ]; then
		ITEMS=""
		FIRST=true
		while IFS=',' read -ra NAMES; do
			for name in "${NAMES[@]}"; do
				name=$(echo "$name" | sed 's/^ *//;s/ *$//')
				[ -z "$name" ] && continue
				if [ "$FIRST" = true ]; then
					ITEMS="\"$name\""
					FIRST=false
				else
					ITEMS="$ITEMS, \"$name\""
				fi
			done
		done <<< "$LOCATIONS"

		PICKED=$(osascript -e "set selected to choose from list {$ITEMS} with prompt \"$PICKER_PROMPT\"" -e "if selected is false then return \"\"" -e "return selected as string" 2>/dev/null)
		if [ -n "$PICKED" ]; then
			LOCATION_ARG="--location '$PICKED'"
			RESOLVED=$("$BINARY" $DRVARG --resolve-location "$PICKED" 2>/dev/null)
			INSTALL_NAME=$(echo "$RESOLVED" | grep "^Name=" | head -1 | cut -d= -f2)
			INSTALL_IP=$(echo "$RESOLVED" | grep "^IP=" | head -1 | cut -d= -f2)
		fi
	else
		APT=$(osascript -e "display dialog \"$NAME_PROMPT\" default answer \"$DETECTED_MODEL\" buttons {\"OK\", \"Cancel\"} default button \"OK\"" -e "return text returned of result" 2>/dev/null)
		if [ -n "$APT" ]; then
			LOCATION_ARG="--name '$APT'"
			INSTALL_NAME="$APT"
			INSTALL_IP="$DETECTED_IP"
			[ -n "$DETECTED_IP" ] && LOCATION_ARG="$LOCATION_ARG --ip '$DETECTED_IP'"
		fi
	fi
fi

# User cancelled → exit
[ -z "$LOCATION_ARG" ] && exit 0

# --- Step 4: Check for existing printer at same IP ---
if [ -n "$INSTALL_IP" ]; then
	EXISTING_NAME=$("$BINARY" $DRVARG --printer-at-ip "$INSTALL_IP" 2>/dev/null)
	if [ -n "$EXISTING_NAME" ]; then
		CONFLICT_MSG=$(printf "$CONFLICT_FMT" "$INSTALL_IP" "$EXISTING_NAME")
		write_msg "$CONFLICT_MSG"
		RESULT=$(osascript_dialog "\"$SKIP_BTN\", \"$OVERWRITE_LABEL\"" "$SKIP_BTN")
		[ "$RESULT" != "$OVERWRITE_LABEL" ] && exit 0
	fi
fi

# --- Step 4: Install with selected location ---
: > "$LOG"
ERR=$(osascript -e "do shell script \"'$BINARY' $DRVARG $LOCATION_ARG > '$LOG' 2>&1\" with administrator privileges with prompt \"$ADMIN_INSTALL_PROMPT\"" 2>&1)
EXIT_CODE=$?

# User cancelled password → exit cleanly
if [ $EXIT_CODE -ne 0 ]; then
	case "$ERR" in
		*[Cc]ancel*|*[Cc]ancelled*|*[Cc]anceled*|*-128*|*not\ authorized*)
			cleanup
			exit 0
			;;
	esac
fi

# Dismiss System Settings printer page if macOS opened it
(sleep 2 && osascript -e 'if application "System Settings" is running then tell application "System Settings" to quit') 2>/dev/null &

# --- Step 5: Post-install dialogs ---
cleanup() {
	rm -f "$STATUS_FILE" "$MSG_FILE"
}

if [ $EXIT_CODE -eq 0 ]; then
	RAW_MSG=""
	if [ -s "$STATUS_FILE" ]; then
		RAW_MSG=$(tr -d '"' < "$STATUS_FILE")
	else
		RAW_MSG="Printer installed successfully"
	fi

	# Translate Go output using i18n values
	DIALOG_MSG=$(echo "$RAW_MSG" | sed "s/ installed$/$INSTALLED_LABEL/;s/^Other printers: /$OTHER_PRINTERS_LABEL/")
	DIALOG_MSG="✅ $DIALOG_MSG$AUTO_CLOSE"
	show_dialog "$DIALOG_MSG" note 5

	OTHER_LINE=$(echo "$RAW_MSG" | grep "Other printers: ")
	if [ -n "$OTHER_LINE" ]; then
		OTHER_VAL=$(echo "$OTHER_LINE" | sed 's/.*Other printers: //')
		FIRST_LINE=$(echo "$RAW_MSG" | head -1 | sed 's/[[:space:]]*$//')
		INSTALLED_NAME=$(echo "$FIRST_LINE" | sed 's/ installed$//')
		show_delete_dialog "$OTHER_VAL" "$INSTALLED_NAME"
	fi
else
	if [ ! -s "$LOG" ]; then
		echo "$ERR" > "$LOG"
	fi
	ERR_MSG=$(head -20 "$LOG" 2>/dev/null | tr -d '"' || echo "Unknown error")
	show_dialog "$FAIL_PREFIX\n$ERR_MSG" stop
fi
cleanup
