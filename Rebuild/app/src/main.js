import { getInitialState, getStrings, confirm, quit } from "./api.js";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { LogicalSize } from "@tauri-apps/api/dpi";

const $ = (id) => document.getElementById(id);
const screens = ["loading", "confirm", "progress", "result"];

// Languages offered in the top-right globe menu. Codes mirror printer-core i18n.
const LANGS = [
  { code: "en", name: "English" },
  { code: "ja", name: "日本語" },
  { code: "ko", name: "한국어" },
  { code: "zh", name: "简体中文" },
  { code: "zh-Hant", name: "繁體中文" },
];

function show(name) {
  screens.forEach((s) => ($(s).hidden = s !== name));
  requestAnimationFrame(fitWindow);
}

// Tight-fit ONLY the height; width is fixed (never re-set). Measuring #app
// directly avoids viewport-clamping and retina physical/logical mismatches
// that caused the window to flash wide.
const WIN_W = 492;
async function fitWindow() {
  try {
    await document.fonts.ready;
    const win = getCurrentWindow();
    const app = document.getElementById("app");
    const rect = app.getBoundingClientRect();
    const h = Math.ceil(rect.height) + 24; // 6px top/bottom margin + breathing room
    const cur = await win.innerSize();
    if (Math.abs(cur.height - h) < 4) return; // already tight, skip resize
    await win.setSize(new LogicalSize(WIN_W, h));
  } catch (_) {
    /* ignore */
  }
}

// Re-measure a few times: CJK fonts swap late and would otherwise grow the
// card after we sized the window, clipping the bottom buttons.
function scheduleFit() {
  for (const ms of [0, 120, 350, 700]) {
    setTimeout(fitWindow, ms);
  }
}

let S = null; // initial state
let chosenLoc = "";
let lang = "en";

function langName(code) {
  return LANGS.find((l) => l.code === code)?.name ?? "";
}

function buildLangMenu() {
  const drop = $("lang-drop");
  drop.innerHTML = "";
  for (const l of LANGS) {
    const b = document.createElement("button");
    b.type = "button";
    b.dataset.lang = l.code;
    b.textContent = l.name;
    b.addEventListener("click", () => switchLang(l.code));
    drop.appendChild(b);
  }
}

function refreshLangMenu() {
  $("lang-btn").textContent = "🌐";
  for (const b of $("lang-drop").querySelectorAll("button")) {
    b.classList.toggle("active", b.dataset.lang === lang);
  }
}

async function switchLang(code) {
  if (!S) return;
  lang = code;
  refreshLangMenu();
  $("lang-drop").hidden = true;
  try {
    S.strings = await getStrings(code);
  } catch (_) {
    /* keep current strings on failure */
  }
  renderConfirm();
  requestAnimationFrame(scheduleFit);
}

function t(key, ...args) {
  let s = S.strings[key] ?? "";
  for (const a of args) {
    s = s.replace("%s", a);
    s = s.replace("%d", a);
  }
  return s;
}

function locIPs(loc) {
  return S.loc_ips[loc] ?? [];
}

function renderConfirm() {
  // title + summary
  $("title").textContent = t("TITLE");
  $("summary").textContent = [
    S.detected_location ?? t("NO_LOCATION"),
    S.detected_name,
    S.detected_ip ? "IP: " + S.detected_ip : "",
  ]
    .filter(Boolean)
    .join("  |  ");

  // confirm checkbox
  $("confirm-row").hidden = !S.detected_location;
  $("confirm-label").textContent = t("CONFIRM_FMT", S.detected_location ?? "");
  $("chk-confirm").checked = !!S.detected_location;
  updatePickerVisibility();

  // picker
  $("picker-label").textContent = t("PICKER_PROMPT");
  $("picker").innerHTML = "";
  const others = S.locations.filter((l) => l !== S.detected_location);
  for (const l of others) {
    const opt = document.createElement("option");
    opt.value = l;
    opt.textContent = l;
    $("picker").appendChild(opt);
  }

  // conflict
  $("conflict-label").textContent = t("CONFLICT_LABEL");
  $("conflict").innerHTML = "";
  for (const v of [t("SKIP_BTN"), t("OVERWRITE_LABEL")]) {
    const opt = document.createElement("option");
    opt.value = v;
    opt.textContent = v;
    $("conflict").appendChild(opt);
  }

  // delete list
  $("existing-label").textContent = t("EXISTING_PRINTERS", S.existing.length);
  $("delete-list").innerHTML = "";
  for (const p of S.existing) {
    const label = document.createElement("label");
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.dataset.name = p.name;
    cb.dataset.ip = p.ip;
    const span = document.createElement("span");
    span.textContent = `${p.name} (${p.ip || "?"})`;
    label.append(cb, span);
    $("delete-list").appendChild(label);
  }

  $("btn-ok").textContent = t("OK_LABEL");
  $("btn-cancel").textContent = t("CANCEL_LABEL");
  $("btn-close").textContent = t("OK_LABEL");

  chosenLoc = S.detected_location ?? $("picker").options[0]?.value ?? "";
  updateChosenState();
  requestAnimationFrame(scheduleFit);
}

function currentLoc() {
  if ($("chk-confirm").checked && S.detected_location) {
    return S.detected_location;
  }
  return $("picker").value || "";
}

function updatePickerVisibility() {
  $("picker-wrap").hidden = $("chk-confirm").checked;
}

function updateChosenState() {
  chosenLoc = currentLoc();
  const ips = locIPs(chosenLoc);

  // conflict enabled only when a printer exists at the chosen location IPs
  $("conflict").disabled = !(S.conflict[chosenLoc] ?? false);

  // disable delete checkboxes whose IP belongs to the chosen location
  for (const label of $("delete-list").querySelectorAll("label")) {
    const cb = label.querySelector("input");
    const disabled = ips.includes(cb.dataset.ip) || (cb.dataset.ip === "" && false);
    cb.disabled = disabled;
    if (disabled) cb.checked = false;
  }
}

function updateSummary() {
  const loc = currentLoc();
  const names = S.loc_names[loc] ?? [];
  const ips = locIPs(loc);
  $("summary").textContent = [
    loc || t("NO_LOCATION"),
    names.length ? names.join(", ") : "",
    ips.length ? "IP: " + ips.join(", ") : "",
  ]
    .filter(Boolean)
    .join("  |  ");
}

async function doConfirm() {
  const checked = [];
  for (const cb of $("delete-list").querySelectorAll("input")) {
    if (cb.checked) checked.push(cb.dataset.name);
  }
  const req = {
    location: currentLoc(),
    overwrite: $("conflict").value === t("OVERWRITE_LABEL"),
    delete: checked,
    lang,
  };
  if (!req.location) {
    showResult([t("FAIL_PREFIX") + " no location"]);
    return;
  }
  $("progress-text").textContent = t("INSTALLING");
  show("progress");

  try {
    const res = await confirm(req);
    if (res.cancelled) {
      // User cancelled the admin password prompt → back to the dialog.
      show("confirm");
      return;
    }
    showResult(res.messages.length ? res.messages : ["ok"]);
  } catch (e) {
    showResult([t("FAIL_PREFIX") + " " + e]);
  }
}

function showResult(lines) {
  const el = $("result-body");
  el.innerHTML = "";
  for (const line of lines) {
    const p = document.createElement("p");
    p.textContent = line;
    if (line.includes("❌")) p.className = "fail";
    el.appendChild(p);
  }
  show("result");
}

function wire() {
  $("lang-btn").addEventListener("click", (e) => {
    e.stopPropagation();
    $("lang-drop").hidden = !$("lang-drop").hidden;
  });
  document.addEventListener("click", (e) => {
    if (!e.target.closest("#lang-menu")) $("lang-drop").hidden = true;
  });

  $("chk-confirm").addEventListener("change", () => {
    updatePickerVisibility();
    updateChosenState();
    updateSummary();
    requestAnimationFrame(scheduleFit);
  });
  $("picker").addEventListener("change", () => {
    updateChosenState();
    updateSummary();
  });
  $("btn-cancel").addEventListener("click", () => quit());
  $("btn-ok").addEventListener("click", doConfirm);
  $("btn-close").addEventListener("click", () => quit());
}

(async () => {
  wire();
  buildLangMenu();
  try {
    S = await getInitialState();
  } catch (e) {
    showResult(["❌ failed to load state: " + e]);
    return;
  }
  lang = S.lang || "en";
  refreshLangMenu();
  renderConfirm();
  show("confirm");
})();
