<script lang="ts">
  import { getAppState } from '../lib/appContext';

  const app = getAppState();
</script>

{#if app.events.length === 0}
  <div class="empty-state">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>
    <p>No events recorded yet</p>
  </div>
{:else}
  <div class="table-wrap">
    <table>
      <thead><tr><th>Time</th><th>Container</th><th>Event</th></tr></thead>
      <tbody>
        {#each app.events as ev (ev.id)}
          <tr>
            <td class="cell-mono cell-mist">{new Date(ev.occurred_at).toLocaleString()}</td>
            <td class="cell-mono">{ev.container_name}</td>
            <td><span class="pill pill-{ev.event === 'start' ? 'green' : ev.event === 'stop' ? 'red' : 'violet'}">{ev.event}</span></td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  .empty-state {
    display: flex; flex-direction: column; align-items: center; gap: 10px;
    padding: 80px 0; color: var(--mist);
  }
  .empty-state :global(svg) { width: 38px; height: 38px; color: var(--mist-2); opacity: .6; }
  .empty-state p { font-size: 14px; margin: 0; }
  .table-wrap { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  thead th {
    text-align: left; padding: 10px 18px; font-size: 11.5px; text-transform: uppercase;
    letter-spacing: .05em; color: var(--mist-2); font-weight: 700;
    border-bottom: 1px solid var(--border); background: var(--surface-2); white-space: nowrap;
  }
  :global(tbody td) {
    padding: 11px 18px; border-bottom: 1px solid var(--border);
    font-size: 13.3px; vertical-align: middle;
  }
  :global(tbody tr:last-child td) { border-bottom: none; }
  :global(tbody tr:hover td) { background: var(--surface-2); }
  .cell-mono { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
  .cell-mist { color: var(--mist); }
  .pill { display: inline-block; padding: 2px 8px; border-radius: 20px; font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; }
  .pill-green { background: var(--green-tint); color: var(--green); }
  .pill-red { background: var(--red-tint); color: var(--red); }
  .pill-violet { background: var(--accent-tint); color: var(--accent); }
  .pill-mist { background: var(--surface-2); color: var(--mist); }
</style>
