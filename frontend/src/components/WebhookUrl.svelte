<script lang="ts">
  import { isLocalOrigin } from "../lib/webhooks";
  import { toast } from "../lib/toast.svelte";

  let { url }: { url: string } = $props();

  let copied = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function copy() {
    if (!(await writeClipboard(url))) {
      toast.error("Couldn't copy: select the URL and copy it instead");
      return;
    }
    copied = true;
    toast.success("Webhook URL copied");
    clearTimeout(timer);
    timer = setTimeout(() => (copied = false), 2000);
  }

  /** The Clipboard API only exists on HTTPS or localhost; a gateway on plain
   *  HTTP falls back to the older copy command. */
  async function writeClipboard(text: string): Promise<boolean> {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      const ok = document.execCommand("copy");
      ta.remove();
      return ok;
    }
  }
</script>

<div class="wh">
  <div class="wh-row">
    <span class="wh-method">POST</span>
    <code class="wh-url">{url}</code>
    <button class="wh-copy" class:copied type="button" title={copied ? "Copied" : "Copy URL"}
      aria-label={copied ? "Copied" : "Copy webhook URL"} onclick={copy}>
      {#if copied}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M20 6 9 17l-5-5" /></svg>
        Copied
      {:else}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="9" y="9" width="13" height="13" rx="2" />
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
        </svg>
        Copy
      {/if}
    </button>
  </div>
  {#if isLocalOrigin()}
    <div class="wh-note">
      This URL uses the address you opened Pulse with. A CI runner on another machine needs your
      gateway's public address in its place.
    </div>
  {/if}
</div>

<style>
  .wh { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
  .wh-row { display: flex; align-items: flex-start; gap: 6px; min-width: 0; }
  .wh-method {
    font-family: "JetBrains Mono", monospace; font-size: 10px; font-weight: 800;
    color: var(--accent-ink); background: var(--accent-tint);
    padding: 3px 6px; border-radius: 4px; flex-shrink: 0;
  }
  .wh-url {
    flex: 1; min-width: 0;
    font-family: "JetBrains Mono", monospace; font-size: 10.5px; line-height: 1.5;
    color: var(--ink); background: var(--surface-2);
    padding: 3px 8px; border-radius: 4px;
    word-break: break-all; user-select: all;
  }
  .wh-copy {
    height: 24px; padding: 0 8px; flex-shrink: 0;
    display: flex; align-items: center; gap: 4px;
    font-size: 11px; font-weight: 600;
    border: 1px solid var(--border); border-radius: 5px;
    background: var(--surface); color: var(--mist-2); cursor: pointer;
  }
  .wh-copy:hover { color: var(--ink); border-color: var(--border-strong); }
  .wh-copy svg { width: 12px; height: 12px; }
  .wh-copy.copied { color: var(--green); border-color: var(--green); background: var(--green-tint); }
  .wh-note { font-size: 11px; color: var(--mist); line-height: 1.4; }
</style>
