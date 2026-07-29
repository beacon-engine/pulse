<script lang="ts">
  let {
    page,
    perPage,
    total,
    onpage,
  }: {
    page: number;
    perPage: number;
    total: number;
    onpage: (p: number) => void;
  } = $props();

  let pages = $derived(Math.max(1, Math.ceil(total / perPage)));
  let from = $derived(total === 0 ? 0 : (page - 1) * perPage + 1);
  let to = $derived(Math.min(page * perPage, total));
</script>

{#if total > perPage}
  <div class="pager">
    <span class="pager-info">{from}–{to} of {total}</span>
    <div class="pager-btns">
      <button class="pager-btn" disabled={page <= 1} onclick={() => onpage(page - 1)}>‹ Prev</button>
      <span class="pager-cur">{page} / {pages}</span>
      <button class="pager-btn" disabled={page >= pages} onclick={() => onpage(page + 1)}>Next ›</button>
    </div>
  </div>
{/if}

<style>
  .pager {
    display: flex; align-items: center; justify-content: space-between;
    gap: 12px; padding: 12px 2px; flex-wrap: wrap;
  }
  .pager-info { font-size: 12px; color: var(--mist-2); }
  .pager-btns { display: flex; align-items: center; gap: 8px; }
  .pager-cur { font-size: 12.5px; color: var(--mist); font-variant-numeric: tabular-nums; min-width: 54px; text-align: center; }
  .pager-btn {
    padding: 5px 11px; border: 1px solid var(--border); border-radius: var(--radius-sm);
    background: var(--surface); color: var(--ink); font-size: 12.5px; font-weight: 600;
    cursor: pointer; transition: background .12s, border-color .12s;
  }
  .pager-btn:hover:not(:disabled) { background: var(--surface-2); border-color: var(--border-strong); }
  .pager-btn:disabled { opacity: .4; cursor: not-allowed; }
</style>
