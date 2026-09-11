//! Tests for the shared WebSocket transport: TLS-mode parsing, rustls config
//! construction (both trust policies), and the correlation envelope round-trip.

use protocol::transport::{TlsMode, rustls_client_config};
use protocol::wire::{EVENT_STREAM_ID, Request, RequestFrame, Response, ResponseFrame};

#[test]
fn tls_mode_defaults_to_verify() {
    assert_eq!(TlsMode::from_flag(None), TlsMode::Verify);
    assert_eq!(TlsMode::from_flag(Some("")), TlsMode::Verify);
    assert_eq!(TlsMode::from_flag(Some("0")), TlsMode::Verify);
    assert_eq!(TlsMode::from_flag(Some("false")), TlsMode::Verify);
}

#[test]
fn tls_mode_truthy_flags_skip_verification() {
    for v in ["1", "true", "TRUE", "Yes", " on "] {
        assert_eq!(
            TlsMode::from_flag(Some(v)),
            TlsMode::InsecureSkipVerify,
            "{v:?} should skip verification"
        );
    }
}

#[test]
fn rustls_config_builds_for_both_modes() {
    // Exercises the ring provider + webpki roots (Verify) and the custom
    // no-verification verifier (InsecureSkipVerify). Construction must not panic.
    let verify = rustls_client_config(TlsMode::Verify);
    assert!(
        !verify
            .crypto_provider()
            .signature_verification_algorithms
            .supported_schemes()
            .is_empty()
    );

    let insecure = rustls_client_config(TlsMode::InsecureSkipVerify);
    assert!(
        !insecure
            .crypto_provider()
            .signature_verification_algorithms
            .supported_schemes()
            .is_empty()
    );
}

#[test]
fn envelope_round_trips_over_json() {
    let req = RequestFrame {
        id: 42,
        req: Request::K8sRollout {
            namespace: "prod".into(),
            deployment: "api".into(),
        },
    };
    let bytes = serde_json::to_vec(&req).unwrap();
    let back: RequestFrame = serde_json::from_slice(&bytes).unwrap();
    assert_eq!(back.id, 42);
    assert!(matches!(back.req, Request::K8sRollout { .. }));

    let resp = ResponseFrame {
        id: 42,
        resp: Response::Ok,
    };
    let bytes = serde_json::to_vec(&resp).unwrap();
    let back: ResponseFrame = serde_json::from_slice(&bytes).unwrap();
    assert_eq!(back.id, 42);
    assert!(matches!(back.resp, Response::Ok));
}

#[test]
fn events_use_reserved_stream_id() {
    // Real request ids start at 1, so the reserved event id never collides.
    assert_eq!(EVENT_STREAM_ID, 0);
}
