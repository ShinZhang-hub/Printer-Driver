// Thin wrapper over the injected Tauri global (withGlobalTauri: true).
const invoke = window.__TAURI__?.core?.invoke;

export async function getInitialState() {
  return invoke("get_initial_state");
}

/** All UI strings for a language, e.g. "en" | "ja" | "ko" | "zh". */
export async function getStrings(lang) {
  return invoke("get_strings", { lang });
}

/** Terminate the process (same as clicking the window close button). */
export async function quit() {
  return invoke("quit");
}

/**
 * Run the install/delete plan.
 * @param {object} req { location, overwrite, delete }
 */
export async function confirm(req) {
  return invoke("confirm", { req });
}
