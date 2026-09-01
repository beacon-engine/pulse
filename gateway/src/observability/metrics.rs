//! Prometheus metrics for the gateway process.
#![allow(dead_code)]

use prometheus::{
    CounterVec, Gauge, IntGauge, TextEncoder, register_counter_vec, register_gauge,
    register_int_gauge,
};
use std::sync::OnceLock;
use std::time::Instant;

static START_TIME: OnceLock<Instant> = OnceLock::new();

pub struct Metrics {
    pub http_requests_total: CounterVec,
    pub containers_total: IntGauge,
    pub uptime_seconds: Gauge,
}

static METRICS: OnceLock<Metrics> = OnceLock::new();

pub fn init() {
    START_TIME.get_or_init(Instant::now);
    METRICS.get_or_init(|| Metrics {
        http_requests_total: register_counter_vec!(
            "pulse_http_requests_total",
            "Total number of HTTP requests",
            &["method", "endpoint", "status"]
        )
        .expect("Failed to register http_requests_total"),
        containers_total: register_int_gauge!(
            "pulse_containers_total",
            "Number of containers known to Docker"
        )
        .expect("Failed to register containers_total"),
        uptime_seconds: register_gauge!("pulse_uptime_seconds", "Seconds since Pulse started")
            .expect("Failed to register uptime_seconds"),
    });
}

pub fn record_request(method: &str, endpoint: &str, status: &str) {
    METRICS
        .get()
        .expect("metrics not initialized")
        .http_requests_total
        .with_label_values(&[method, endpoint, status])
        .inc();
}

pub fn set_container_count(n: i64) {
    METRICS
        .get()
        .expect("metrics not initialized")
        .containers_total
        .set(n);
}

static EVENTS: OnceLock<prometheus::CounterVec> = OnceLock::new();

pub fn init_events_counter() {
    EVENTS.get_or_init(|| {
        register_counter_vec!(
            "pulse_container_events_total",
            "Container lifecycle events recorded by Pulse",
            &["event"]
        )
        .expect("Failed to register pulse_container_events_total")
    });
}

pub fn record_event(event: &str) {
    if let Some(c) = EVENTS.get() {
        c.with_label_values(&[event]).inc();
    }
}

pub fn render() -> String {
    if let Some(m) = METRICS.get()
        && let Some(t) = START_TIME.get()
    {
        m.uptime_seconds.set(t.elapsed().as_secs_f64());
    }
    TextEncoder::new()
        .encode_to_string(&prometheus::gather())
        .unwrap_or_default()
}
