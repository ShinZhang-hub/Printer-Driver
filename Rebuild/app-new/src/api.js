const invoke = window.__TAURI__?.core?.invoke;

export async function getInitialState() {
  return invoke("get_initial_state");
}

export async function getStrings(lang) {
  return invoke("get_strings", { lang });
}

export async function refreshConfig() {
  return invoke("refresh_config");
}

export async function quit() {
  return invoke("quit");
}

export async function confirm(req) {
  return invoke("confirm", { req });
}

export async function getInstalledPrinters() {
  return invoke("get_installed_printers");
}

export async function checkServerHealth() {
  return invoke("check_server_health");
}
