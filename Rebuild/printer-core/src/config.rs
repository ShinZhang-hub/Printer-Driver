use serde::{Deserialize, Serialize};
use std::sync::{Mutex, OnceLock};

pub const EMBEDDED_CONFIG: &str = include_str!("../assets/config.json");

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct PrinterInfo {
    pub ip: String,
    pub name: String,
    #[serde(default)]
    pub model: String,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct LocationConfig {
    pub name: String,
    #[serde(default)]
    pub subnets: Vec<String>,
    #[serde(default)]
    pub printer_ip: String,
    #[serde(default)]
    pub printer_name: String,
    #[serde(default)]
    pub printer_model: String,
    #[serde(default)]
    pub port_number: u16,
    #[serde(default)]
    pub protocol: String,
    #[serde(default)]
    pub printers: Vec<PrinterInfo>,
}

impl LocationConfig {
    /// All printers of this location. New `printers[]` wins over the legacy
    /// single `printer_ip`/`printer_name` fields (backward compatible).
    pub fn all_printers(&self) -> Vec<PrinterInfo> {
        if !self.printers.is_empty() {
            self.printers.clone()
        } else if !self.printer_ip.is_empty() {
            vec![PrinterInfo {
                ip: self.printer_ip.clone(),
                name: if self.printer_name.is_empty() {
                    self.printer_model.clone()
                } else {
                    self.printer_name.clone()
                },
                model: self.printer_model.clone(),
            }]
        } else {
            Vec::new()
        }
    }
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Config {
    pub version: u32,
    pub updated_at: String,
    pub config_url: String,
    #[serde(default)]
    pub port_number: u16,
    #[serde(default)]
    pub protocol: String,
    #[serde(default)]
    pub locations: Vec<LocationConfig>,
}

/// Parse embedded / local JSON config.
pub fn parse(data: &str) -> Option<Config> {
    serde_json::from_str(data).ok()
}

/// Global config cache. Seeded from the embedded copy so the UI can render
/// instantly; `refresh_config` swaps in a fresh remote copy off the UI path.
static SHARED: OnceLock<Mutex<Config>> = OnceLock::new();

pub fn shared() -> &'static Mutex<Config> {
    SHARED.get_or_init(|| {
        Mutex::new(parse(EMBEDDED_CONFIG).unwrap_or_default())
    })
}

/// Current snapshot for readers (initial state, install flow).
pub fn shared_snapshot() -> Config {
    shared().lock().map(|g| g.clone()).unwrap_or_default()
}

/// Best-effort remote refresh in the background. Keeps the current cached
/// value on any failure/timeout so startup is never blocked on the network.
/// Returns true when a NEW remote config replaced the cached one.
pub fn refresh_config() -> bool {
    let orig = shared_snapshot();
    if orig.config_url.is_empty() {
        return false;
    }
    let url = format!("{}/api/v1/config", orig.config_url);
    if let Ok(remote) = fetch(&url, 2_000) {
        if let Some(mut c) = parse(&remote) {
            c.config_url = orig.config_url;
            let mut g = shared().lock().unwrap();
            let changed = *g != c;
            *g = c;
            return changed;
        }
    }
    false
}

pub fn fetch(url: &str, timeout_ms: u64) -> Result<String, String> {
    let agent = ureq::AgentBuilder::new()
        .timeout(std::time::Duration::from_millis(timeout_ms))
        .tls_config(std::sync::Arc::new(insecure_tls_config()))
        .build();
    agent
        .get(url)
        .call()
        .map_err(|e| e.to_string())?
        .into_string()
        .map_err(|e| e.to_string())
}

/// TLS config that skips server certificate verification.
/// The config server lives on the corporate LAN with a self-signed cert
/// (Subject == Issuer, no CA will ever validate it), and `fetch` is ONLY
/// used for that internal server — never for public internet hosts.
fn insecure_tls_config() -> rustls::ClientConfig {
    #[derive(Debug)]
    struct NoVerifier;
    impl rustls::client::danger::ServerCertVerifier for NoVerifier {
        fn verify_server_cert(
            &self,
            _end_entity: &rustls::pki_types::CertificateDer<'_>,
            _intermediates: &[rustls::pki_types::CertificateDer<'_>],
            _server_name: &rustls::pki_types::ServerName<'_>,
            _ocsp_response: &[u8],
            _now: rustls::pki_types::UnixTime,
        ) -> Result<rustls::client::danger::ServerCertVerified, rustls::Error> {
            Ok(rustls::client::danger::ServerCertVerified::assertion())
        }
        fn verify_tls12_signature(
            &self,
            _message: &[u8],
            _cert: &rustls::pki_types::CertificateDer<'_>,
            _dss: &rustls::DigitallySignedStruct,
        ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
            Ok(rustls::client::danger::HandshakeSignatureValid::assertion())
        }
        fn verify_tls13_signature(
            &self,
            _message: &[u8],
            _cert: &rustls::pki_types::CertificateDer<'_>,
            _dss: &rustls::DigitallySignedStruct,
        ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
            Ok(rustls::client::danger::HandshakeSignatureValid::assertion())
        }
        fn supported_verify_schemes(&self) -> Vec<rustls::SignatureScheme> {
            use rustls::SignatureScheme;
            vec![
                SignatureScheme::RSA_PKCS1_SHA256,
                SignatureScheme::RSA_PKCS1_SHA384,
                SignatureScheme::RSA_PKCS1_SHA512,
                SignatureScheme::ECDSA_NISTP256_SHA256,
                SignatureScheme::ECDSA_NISTP384_SHA384,
                SignatureScheme::ECDSA_NISTP521_SHA512,
                SignatureScheme::RSA_PSS_SHA256,
                SignatureScheme::RSA_PSS_SHA384,
                SignatureScheme::RSA_PSS_SHA512,
                SignatureScheme::ED25519,
            ]
        }
    }

    // Explicit ring provider: avoids process-default CryptoProvider
    // ambiguity when multiple providers are compiled in.
    let provider = std::sync::Arc::new(rustls::crypto::ring::default_provider());
    let mut cfg = rustls::ClientConfig::builder_with_provider(provider)
        .with_protocol_versions(rustls::DEFAULT_VERSIONS)
        .expect("ring supports default versions")
        .with_root_certificates(rustls::RootCertStore::empty())
        .with_no_client_auth();
    cfg.dangerous()
        .set_certificate_verifier(std::sync::Arc::new(NoVerifier));
    cfg
}

impl Default for Config {
    fn default() -> Self {
        Config {
            version: 1,
            updated_at: String::new(),
            config_url: String::new(),
            port_number: 9100,
            protocol: "raw".into(),
            locations: Vec::new(),
        }
    }
}
