//! Shared WebSocket (WSS) transport for the gateway ↔ agent link.
//!
//! Both roles speak WebSocket over `ws://` or `wss://`, so the traffic rides
//! HTTP/443 and traverses firewalls and proxies that block arbitrary TCP ports.
//! This module provides the **outbound client** half — used by the agent in
//! `dial` (push) mode and by the gateway when it connects to a listening agent
//! (pull). The server half lives in each binary (the gateway upgrades via axum;
//! the agent accepts with tungstenite).
//!
//! ## TLS
//!
//! TLS is rustls with the **ring** crypto provider — no `native-tls`/OpenSSL, so
//! musl and slim/distroless container images link without a system `libssl`.
//! Trust anchors are the bundled webpki roots (no dependency on an OS cert
//! store). Verification can be disabled with `PULSE_INSECURE_SKIP_TLS_VERIFY` for
//! trusted networks or self-signed gateways.

use std::sync::Arc;

use tokio::net::TcpStream;
use tokio_tungstenite::tungstenite::Error as WsError;
use tokio_tungstenite::tungstenite::client::IntoClientRequest;
use tokio_tungstenite::tungstenite::http::header::AUTHORIZATION;
use tokio_tungstenite::{
    Connector, MaybeTlsStream, WebSocketStream, connect_async_tls_with_config,
};

/// A connected client WebSocket stream (plain for `ws://`, rustls for `wss://`).
pub type WsStream = WebSocketStream<MaybeTlsStream<TcpStream>>;

/// Re-export so both sides frame messages against one WebSocket message type.
pub use tokio_tungstenite::tungstenite::Message;

/// Server-certificate trust policy for outbound `wss://` connections.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TlsMode {
    /// Verify the server certificate against the bundled webpki roots (default).
    Verify,
    /// Skip certificate verification entirely. For trusted networks or
    /// self-signed gateways only — this disables MITM protection.
    InsecureSkipVerify,
}

impl TlsMode {
    /// Parses a trust-policy flag (e.g. the value of `PULSE_INSECURE_SKIP_TLS_VERIFY`,
    /// read by each binary's config): truthy (`1`/`true`/`yes`/`on`,
    /// case-insensitive) selects [`TlsMode::InsecureSkipVerify`]; anything else
    /// (including unset) is [`TlsMode::Verify`]. Env access stays in `config.rs`.
    pub fn from_flag(value: Option<&str>) -> Self {
        match value.map(|s| s.trim().to_ascii_lowercase()).as_deref() {
            Some("1" | "true" | "yes" | "on") => TlsMode::InsecureSkipVerify,
            _ => TlsMode::Verify,
        }
    }
}

/// Builds the rustls client config for the given trust policy, pinned to the
/// ring provider (matching the rest of the tree, and avoiding aws-lc-rs's C
/// build).
pub fn rustls_client_config(mode: TlsMode) -> Arc<rustls::ClientConfig> {
    let provider = Arc::new(rustls::crypto::ring::default_provider());
    let builder = rustls::ClientConfig::builder_with_provider(provider)
        .with_safe_default_protocol_versions()
        .expect("ring provider supports the default TLS versions");

    let config = match mode {
        TlsMode::Verify => {
            let mut roots = rustls::RootCertStore::empty();
            roots.extend(webpki_roots::TLS_SERVER_ROOTS.iter().cloned());
            builder.with_root_certificates(roots).with_no_client_auth()
        }
        TlsMode::InsecureSkipVerify => builder
            .dangerous()
            .with_custom_certificate_verifier(Arc::new(danger::NoVerification::new()))
            .with_no_client_auth(),
    };

    Arc::new(config)
}

/// Opens a client WebSocket to `url` (`ws://` or `wss://`), optionally sending a
/// `Authorization: Bearer <token>` header on the upgrade so the server can
/// authenticate the agent before accepting the session.
pub async fn connect(url: &str, mode: TlsMode, bearer: Option<&str>) -> Result<WsStream, WsError> {
    let mut request = url.into_client_request()?;
    if let Some(token) = bearer {
        let value = format!("Bearer {token}").parse().map_err(|_| {
            WsError::Url(tokio_tungstenite::tungstenite::error::UrlError::EmptyHostName)
        })?;
        request.headers_mut().insert(AUTHORIZATION, value);
    }

    // `Connector::Rustls` is only consulted for `wss://`; plain `ws://` ignores it.
    let connector = Connector::Rustls(rustls_client_config(mode));
    let (stream, _response) =
        connect_async_tls_with_config(request, None, false, Some(connector)).await?;
    Ok(stream)
}

/// A rustls verifier that accepts any server certificate. Signature checks still
/// run (via the ring provider) so the handshake stays well-formed; only the
/// certificate *trust chain* is skipped. Gated behind [`TlsMode::InsecureSkipVerify`].
mod danger {
    use rustls::client::danger::{HandshakeSignatureValid, ServerCertVerified, ServerCertVerifier};
    use rustls::crypto::{CryptoProvider, verify_tls12_signature, verify_tls13_signature};
    use rustls::pki_types::{CertificateDer, ServerName, UnixTime};
    use rustls::{DigitallySignedStruct, Error, SignatureScheme};

    #[derive(Debug)]
    pub struct NoVerification(CryptoProvider);

    impl NoVerification {
        pub fn new() -> Self {
            Self(rustls::crypto::ring::default_provider())
        }
    }

    impl ServerCertVerifier for NoVerification {
        fn verify_server_cert(
            &self,
            _end_entity: &CertificateDer<'_>,
            _intermediates: &[CertificateDer<'_>],
            _server_name: &ServerName<'_>,
            _ocsp_response: &[u8],
            _now: UnixTime,
        ) -> Result<ServerCertVerified, Error> {
            Ok(ServerCertVerified::assertion())
        }

        fn verify_tls12_signature(
            &self,
            message: &[u8],
            cert: &CertificateDer<'_>,
            dss: &DigitallySignedStruct,
        ) -> Result<HandshakeSignatureValid, Error> {
            verify_tls12_signature(
                message,
                cert,
                dss,
                &self.0.signature_verification_algorithms,
            )
        }

        fn verify_tls13_signature(
            &self,
            message: &[u8],
            cert: &CertificateDer<'_>,
            dss: &DigitallySignedStruct,
        ) -> Result<HandshakeSignatureValid, Error> {
            verify_tls13_signature(
                message,
                cert,
                dss,
                &self.0.signature_verification_algorithms,
            )
        }

        fn supported_verify_schemes(&self) -> Vec<SignatureScheme> {
            self.0.signature_verification_algorithms.supported_schemes()
        }
    }
}
