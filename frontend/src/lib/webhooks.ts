/** The full URL of a webhook action, for a CI job to POST to. It uses the
 *  address this page was opened with. */
export function webhookUrl(path: string, params: Record<string, string> = {}): string {
  const url = new URL(`/api/webhooks/${path}`, window.location.origin);
  for (const [k, v] of Object.entries(params)) url.searchParams.set(k, v);
  return url.toString();
}

/** Whether the page was opened on this machine, so a webhook URL built from
 *  its address won't be reachable from a CI runner. */
export function isLocalOrigin(): boolean {
  return ["localhost", "127.0.0.1", "[::1]"].includes(window.location.hostname);
}
