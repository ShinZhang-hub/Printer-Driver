use printer_core::i18n;
use printer_core::printer;

fn main() {
    println!("lang: {}", i18n::detect());
    let state = printer_core::initial_state();
    println!("{}", serde_json::to_string_pretty(&state).unwrap());
    println!("--- existing printers ---");
    for (name, ip) in printer::list_printers_with_ips() {
        println!("  {} -> {}", name, ip);
    }
}
