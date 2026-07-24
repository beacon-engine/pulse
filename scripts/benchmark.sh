#!/usr/bin/env bash
#
# Measure CPU% and resident memory (RSS) of the Beacon agent and gateway, at
# idle and (optionally) under light HTTP load. Prints a Markdown table you can
# paste into the README.
#
# Usage:
#   scripts/benchmark.sh [DURATION_SECONDS]
#
# Env:
#   GATEWAY_PORT   HTTP port of the gateway  (default 8080)
#   AGENT_PORT     TCP port of the agent     (default 9000)
#   GATEWAY_URL    base URL for load probes  (default http://localhost:$GATEWAY_PORT)
#   LOAD           1 = generate request load, 0 = measure idle (default 1)
#   BEACON_COOKIE  optional "beacon_session=..." to hit an authed endpoint
#
# Works on macOS and Linux (uses `ps` and `lsof`).

set -euo pipefail

DURATION="${1:-30}"
GATEWAY_PORT="${GATEWAY_PORT:-8080}"
AGENT_PORT="${AGENT_PORT:-9000}"
GATEWAY_URL="${GATEWAY_URL:-http://localhost:${GATEWAY_PORT}}"
LOAD="${LOAD:-1}"
COOKIE="${BEACON_COOKIE:-}"

pid_on_port() { lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1; }

GW_PID="$(pid_on_port "$GATEWAY_PORT" || true)"
AG_PID="$(pid_on_port "$AGENT_PORT" || true)"

if [ -z "${GW_PID:-}" ] || [ -z "${AG_PID:-}" ]; then
  echo "Could not find both processes."
  echo "  gateway (port $GATEWAY_PORT): ${GW_PID:-not found}"
  echo "  agent   (port $AGENT_PORT):   ${AG_PID:-not found}"
  echo "Start the stack first, or set GATEWAY_PORT / AGENT_PORT."
  exit 1
fi

echo "Benchmarking for ${DURATION}s  (gateway pid $GW_PID, agent pid $AG_PID, load=$LOAD)"

# ── Optional load generator ──────────────────────────────────────────────────
LOAD_PID=""
if [ "$LOAD" = "1" ]; then
  (
    end=$(( $(date +%s) + DURATION ))
    while [ "$(date +%s)" -lt "$end" ]; do
      curl -s -o /dev/null "${GATEWAY_URL}/health" || true
      curl -s -o /dev/null "${GATEWAY_URL}/metrics" || true
      if [ -n "$COOKIE" ]; then
        curl -s -o /dev/null -H "Cookie: ${COOKIE}" "${GATEWAY_URL}/api/hosts" || true
      fi
    done
  ) &
  LOAD_PID=$!
fi

# ── Sample loop ──────────────────────────────────────────────────────────────
# Accumulate CPU% (avg) and RSS in KB (peak) per process.
gw_cpu_sum=0; gw_rss_max=0
ag_cpu_sum=0; ag_rss_max=0
samples=0

read_stat() { # $1 = pid -> "rss_kb cpu_pct"
  ps -o rss=,%cpu= -p "$1" 2>/dev/null | awk '{print $1, $2}'
}

for _ in $(seq 1 "$DURATION"); do
  read gw_rss gw_cpu <<<"$(read_stat "$GW_PID")"
  read ag_rss ag_cpu <<<"$(read_stat "$AG_PID")"
  [ -z "${gw_rss:-}" ] && break
  gw_cpu_sum=$(awk -v a="$gw_cpu_sum" -v b="${gw_cpu:-0}" 'BEGIN{print a+b}')
  ag_cpu_sum=$(awk -v a="$ag_cpu_sum" -v b="${ag_cpu:-0}" 'BEGIN{print a+b}')
  [ "${gw_rss:-0}" -gt "$gw_rss_max" ] && gw_rss_max="$gw_rss"
  [ "${ag_rss:-0}" -gt "$ag_rss_max" ] && ag_rss_max="$ag_rss"
  samples=$((samples + 1))
  sleep 1
done

[ -n "$LOAD_PID" ] && kill "$LOAD_PID" 2>/dev/null || true
[ "$samples" -eq 0 ] && { echo "No samples collected."; exit 1; }

mb()  { awk -v k="$1" 'BEGIN{printf "%.1f", k/1024}'; }
avg() { awk -v s="$1" -v n="$2" 'BEGIN{printf "%.1f", s/n}'; }

state="load"; [ "$LOAD" = "0" ] && state="idle"

echo
echo "| Process | State | Peak RSS (MB) | Avg CPU (%) |"
echo "| ------- | ----- | ------------- | ----------- |"
echo "| agent   | $state | $(mb "$ag_rss_max") | $(avg "$ag_cpu_sum" "$samples") |"
echo "| gateway | $state | $(mb "$gw_rss_max") | $(avg "$gw_cpu_sum" "$samples") |"
echo
echo "(${samples} samples over ${DURATION}s; RSS is peak, CPU is average across samples.)"
