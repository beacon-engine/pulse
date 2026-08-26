<script lang="ts">
  import { toast } from '../lib/toast.svelte';
</script>

<div class="toast-container" aria-live="polite">
  {#each toast.toasts as t (t.id)}
    <div class="toast toast-{t.type}">
      <span class="toast-msg">{t.message}</span>
      <button class="toast-close" onclick={() => toast.remove(t.id)}>✕</button>
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    bottom: 20px;
    right: 20px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 9999;
    pointer-events: none;
  }
  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-width: 280px;
    max-width: 420px;
    padding: 10px 14px;
    border-radius: 6px;
    font-size: 13px;
    background: #1e2029;
    color: #e2e4ea;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.35);
    border: 1px solid rgba(255, 255, 255, 0.08);
    animation: toast-in 0.18s ease-out;
  }
  .toast-error {
    border-left: 4px solid var(--red, #ef4444);
    background: #23191d;
  }
  .toast-success {
    border-left: 4px solid var(--green, #10b981);
  }
  .toast-close {
    background: none;
    border: none;
    color: inherit;
    opacity: 0.5;
    cursor: pointer;
    font-size: 12px;
    padding: 2px 4px;
  }
  .toast-close:hover { opacity: 1; }

  @keyframes toast-in {
    from { opacity: 0; transform: translateY(8px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>
