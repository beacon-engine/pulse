// Set at build time from the VERSION file (see vite.config.ts).
declare const __PULSE_VERSION__: string;

/** The dashboard's version, e.g. "v0.1.0", or "dev" for a local build. */
export const PULSE_VERSION_LABEL = /^\d/.test(__PULSE_VERSION__) ? `v${__PULSE_VERSION__}` : __PULSE_VERSION__;
