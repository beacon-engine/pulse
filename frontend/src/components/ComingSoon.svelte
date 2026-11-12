<script lang="ts">
  import type { Snippet } from 'svelte';

  // Placeholder for a settings screen that is planned but not built yet: what it
  // will do, so the menu shows the roadmap instead of hiding it.
  let {
    title,
    summary,
    planned,
    children,
  }: {
    title: string;
    summary: string;
    /** What the screen will let you do. */
    planned: string[];
    /** Optional extra detail, e.g. a sketch of the data model. */
    children?: Snippet;
  } = $props();
</script>

<div class="soon">
  <div class="soon-head">
    <h2>{title}</h2>
    <span class="soon-badge">Coming soon</span>
  </div>
  <p class="summary">{summary}</p>

  <div class="planned">
    <div class="planned-title">Planned</div>
    <ul>
      {#each planned as item (item)}
        <li>{item}</li>
      {/each}
    </ul>
  </div>

  {#if children}
    <div class="extra">{@render children()}</div>
  {/if}
</div>

<style>
  .soon { max-width: 760px; }
  .soon-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
  h2 { margin: 0; font-size: 18px; font-weight: 750; color: var(--ink); }
  .soon-badge {
    font-size: 10.5px; font-weight: 750; letter-spacing: .05em; text-transform: uppercase;
    padding: 2px 9px; border-radius: 20px; background: var(--amber-tint); color: var(--amber);
  }
  .summary { margin: 8px 0 18px; font-size: 13.5px; color: var(--mist); line-height: 1.6; }
  .planned {
    border: 1px dashed var(--border-strong); border-radius: var(--radius-sm);
    padding: 14px 18px; background: var(--surface-2);
  }
  .planned-title {
    font-size: 11.5px; font-weight: 700; color: var(--mist-2); text-transform: uppercase; letter-spacing: .05em;
  }
  ul { margin: 8px 0 0; padding-left: 18px; }
  li { font-size: 13px; color: var(--ink); line-height: 1.6; margin: 3px 0; }
  .extra { margin-top: 18px; font-size: 13px; color: var(--mist); line-height: 1.6; }
  .extra :global(pre) {
    margin: 8px 0 0; padding: 12px 14px; border-radius: var(--radius-sm); overflow-x: auto;
    background: var(--surface-2); border: 1px solid var(--border);
    font-family: 'JetBrains Mono', monospace; font-size: 11.5px; color: var(--ink); line-height: 1.55;
  }
  .extra :global(code) { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
</style>
