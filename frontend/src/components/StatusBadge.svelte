<script lang="ts">
  let { state }: { state: string } = $props();

  const config: Record<string, { color: string; bg: string; glow: string }> = {
    running: { color: '#059669', bg: '#d1fae5', glow: '#6ee7b7' },
    exited:  { color: '#dc2626', bg: '#fee2e2', glow: '#fca5a5' },
    paused:  { color: '#d97706', bg: '#fef3c7', glow: '#fcd34d' },
  };

  let c = $derived(config[state] ?? { color: '#64748b', bg: '#f1f5f9', glow: '#cbd5e1' });
</script>

<span class="badge" style="--color:{c.color}; --bg:{c.bg}; --glow:{c.glow}">
  <span class="dot" class:pulse={state === 'running'}></span>
  {state}
</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 9px;
    border-radius: 9999px;
    background: var(--bg);
    color: var(--color);
    font-size: 0.72rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--color);
    flex-shrink: 0;
  }
  .dot.pulse {
    box-shadow: 0 0 0 0 var(--glow);
    animation: pulse 2s infinite;
  }
  @keyframes pulse {
    0%   { box-shadow: 0 0 0 0 var(--glow); }
    70%  { box-shadow: 0 0 0 6px transparent; }
    100% { box-shadow: 0 0 0 0 transparent; }
  }
</style>
