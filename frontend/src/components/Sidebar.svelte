<script lang="ts">
  import { PULSE_VERSION_LABEL } from '../lib/version';
  let { active, onchange }: { active: string; onchange: (view: string) => void } = $props();

  const nav = [
    { id: 'containers', label: 'Containers', icon: '⬡' },
    { id: 'events',     label: 'Events',     icon: '⚡' },
    { id: 'kubernetes', label: 'Kubernetes',  icon: '✦' },
  ];
</script>

<aside>
  <div class="brand">
    <span class="logo">◈</span>
    <span class="name">Pulse</span>
  </div>

  <nav>
    {#each nav as item}
      <button
        class="nav-item"
        class:active={active === item.id}
        onclick={() => onchange(item.id)}
      >
        <span class="icon">{item.icon}</span>
        {item.label}
      </button>
    {/each}
  </nav>

  <div class="version">{PULSE_VERSION_LABEL}</div>
</aside>

<style>
  aside {
    width: 220px;
    min-height: 100vh;
    background: #0f172a;
    display: flex;
    flex-direction: column;
    padding: 24px 12px;
    flex-shrink: 0;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 12px 32px;
    color: #fff;
  }
  .logo { font-size: 1.4rem; color: #818cf8; }
  .name { font-size: 1.1rem; font-weight: 700; letter-spacing: -0.02em; }
  nav { display: flex; flex-direction: column; gap: 2px; flex: 1; }
  .nav-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 9px 12px;
    border-radius: 8px;
    border: none;
    background: none;
    color: #94a3b8;
    font-size: 0.875rem;
    font-weight: 500;
    cursor: pointer;
    text-align: left;
    transition: background 0.15s, color 0.15s;
  }
  .nav-item:hover { background: #1e293b; color: #e2e8f0; }
  .nav-item.active { background: #1e293b; color: #fff; }
  .nav-item.active .icon { color: #818cf8; }
  .icon { font-size: 0.9rem; width: 18px; text-align: center; }
  .version { font-size: 0.7rem; color: #334155; padding: 0 12px; }
</style>
