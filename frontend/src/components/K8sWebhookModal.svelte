<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "../lib/api";
  import { toast } from "../lib/toast.svelte";
  import { webhookUrl } from "../lib/webhooks";
  import type { K8sWorkload } from "../types/containers";
  import WebhookUrl from "./WebhookUrl.svelte";

  let { hostId, workload, onclose }: { hostId: string; workload: K8sWorkload; onclose: () => void } = $props();

  let token = $state<string | null>(null);
  let loading = $state(true);
  let busy = $state(false);

  // Matches the gateway's k8sWebhookKey.
  const key = $derived(`${workload.namespace}:${workload.kind}:${workload.name}`);
  const url = $derived(
    token
      ? webhookUrl(
          `${hostId}/k8s/${encodeURIComponent(workload.namespace)}/${workload.kind}/${encodeURIComponent(workload.name)}/restart`,
          { token },
        )
      : "",
  );

  onMount(async () => {
    try {
      token = (await api.getWebhookToken(hostId, "k8s", key)).token;
    } catch (e) {
      toast.error(`Couldn't load the webhook: ${e}`);
    } finally {
      loading = false;
    }
  });

  async function generate(confirmText?: string) {
    if (confirmText && !confirm(confirmText)) return;
    busy = true;
    try {
      token = (await api.generateWebhookToken(hostId, "k8s", key)).token;
    } catch (e) {
      toast.error(String(e));
    } finally {
      busy = false;
    }
  }

  async function disable() {
    if (!confirm("Disable this webhook? Its URL stops working.")) return;
    busy = true;
    try {
      await api.deleteWebhookToken(hostId, "k8s", key);
      token = null;
    } catch (e) {
      toast.error(String(e));
    } finally {
      busy = false;
    }
  }
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && onclose()} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="overlay" role="presentation" onclick={onclose}>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal" role="dialog" tabindex="-1" aria-modal="true" aria-labelledby="k8s-wh-title" onclick={(e) => e.stopPropagation()}>
    <div class="head">
      <div>
        <h2 id="k8s-wh-title">Restart webhook</h2>
        <div class="sub">{workload.kind} <b>{workload.name}</b> in {workload.namespace}</div>
      </div>
      <button class="close" type="button" aria-label="Close" onclick={onclose}>✕</button>
    </div>

    <p class="desc">
      A POST to this URL does a rolling restart, like the restart button. Call it from CI after pushing
      an image: pods using a moving tag (such as <code>latest</code>) with
      <code>imagePullPolicy: Always</code> come back on the new image.
    </p>

    {#if loading}
      <div class="muted">Loading…</div>
    {:else if token}
      <WebhookUrl {url} />
      <pre class="example">curl -fsS -X POST '{url}'</pre>
      <div class="actions">
        <button class="btn" type="button" disabled={busy}
          onclick={() => generate("Rotate the token? The current URL stops working.")}>Rotate token</button>
        <button class="btn danger" type="button" disabled={busy} onclick={disable}>Disable</button>
      </div>
    {:else}
      <div class="actions">
        <button class="btn primary" type="button" disabled={busy || workload.protected}
          title={workload.protected ? `Protected: ${workload.protected_reason}` : ""}
          onclick={() => generate()}>Enable webhook</button>
      </div>
    {/if}
  </div>
</div>

<style>
  .overlay {
    position: fixed; inset: 0; z-index: 200;
    background: rgba(15, 23, 42, 0.55); backdrop-filter: blur(2px);
    display: flex; align-items: center; justify-content: center; padding: 16px;
  }
  .modal {
    width: 560px; max-width: 100%;
    background: var(--surface); color: var(--ink);
    border: 1px solid var(--border); border-radius: 14px;
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.2);
    padding: 22px 24px; display: flex; flex-direction: column; gap: 14px;
  }
  .head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
  h2 { margin: 0; font-size: 1.05rem; font-weight: 700; }
  .sub { margin-top: 3px; font-size: 12.5px; color: var(--mist); overflow-wrap: anywhere; }
  .close {
    background: none; border: none; color: var(--mist-2); cursor: pointer;
    font-size: 0.9rem; padding: 4px 8px; border-radius: 6px;
  }
  .close:hover { color: var(--ink); }
  .desc { margin: 0; font-size: 12.5px; line-height: 1.5; color: var(--mist); }
  .desc code { font-family: "JetBrains Mono", monospace; font-size: 11.5px; }
  .muted { font-size: 12.5px; color: var(--mist); }
  .example {
    margin: 0; padding: 8px 10px; border-radius: 6px;
    background: var(--surface-2); color: var(--ink);
    font-family: "JetBrains Mono", monospace; font-size: 10.5px; line-height: 1.5;
    white-space: pre-wrap; word-break: break-all;
  }
  .actions { display: flex; gap: 8px; flex-wrap: wrap; }
  .btn {
    padding: 6px 12px; border-radius: var(--radius-sm);
    border: 1px solid var(--border); background: var(--surface); color: var(--ink);
    font-size: 12px; font-weight: 600; cursor: pointer;
  }
  .btn:hover:not(:disabled) { background: var(--surface-2); border-color: var(--border-strong); }
  .btn:disabled { opacity: 0.45; cursor: not-allowed; }
  .btn.primary { color: var(--accent-ink); border-color: var(--accent); background: var(--accent-tint); }
  .btn.primary:hover:not(:disabled) { background: var(--accent); color: #fff; }
  .btn.danger:hover:not(:disabled) { background: var(--red-tint); color: var(--red); border-color: var(--red); }
</style>
