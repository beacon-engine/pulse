<script lang="ts">
  import { isLocalOrigin } from "../lib/webhooks";
  import { toast } from "../lib/toast.svelte";

  let { url }: { url: string } = $props();

  async function copy() {
    try {
      await navigator.clipboard.writeText(url);
      toast.success("Webhook URL copied");
    } catch {
      toast.error("Couldn't copy: select the URL and copy it instead");
    }
  }
</script>

<div class="wh">
  <div class="wh-row">
    <span class="wh-method">POST</span>
    <code class="wh-url">{url}</code>
    <button class="wh-copy" type="button" title="Copy URL" aria-label="Copy webhook URL" onclick={copy}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="9" y="9" width="13" height="13" rx="2" />
        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
      </svg>
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
    width: 24px; height: 24px; flex-shrink: 0;
    display: flex; align-items: center; justify-content: center;
    border: 1px solid var(--border); border-radius: 5px;
    background: var(--surface); color: var(--mist-2); cursor: pointer;
  }
  .wh-copy:hover { color: var(--ink); border-color: var(--border-strong); }
  .wh-copy svg { width: 12px; height: 12px; }
  .wh-note { font-size: 11px; color: var(--mist); line-height: 1.4; }
</style>
