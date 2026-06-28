<script lang="ts">
  let {
    label,
    value,
    sub = '',
    accent = '#818cf8',
  }: { label: string; value: number | string; sub?: string; accent?: string } = $props();
</script>

<div class="card" style="--accent: {accent}">
  <div class="top">
    <span class="label">{label}</span>
    <span class="dot"></span>
  </div>
  <div class="value">{value}</div>
  {#if sub}<div class="sub">{sub}</div>{/if}
</div>

<style>
  .card {
    background: #fff;
    border-radius: 12px;
    padding: 20px 24px;
    border: 1px solid #e2e8f0;
    border-top: 3px solid var(--accent);
    box-shadow: 0 1px 3px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.04);
    flex: 1;
    min-width: 140px;
    transition: box-shadow 0.2s;
  }
  .card:hover { box-shadow: 0 4px 16px rgba(0,0,0,0.08); }
  .top { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
  .label { font-size: 0.78rem; font-weight: 600; color: #64748b; text-transform: uppercase; letter-spacing: 0.06em; }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--accent); }
  .value { font-size: 2rem; font-weight: 700; color: #0f172a; letter-spacing: -0.03em; line-height: 1; }
  .sub { font-size: 0.78rem; color: #94a3b8; margin-top: 6px; }
</style>
