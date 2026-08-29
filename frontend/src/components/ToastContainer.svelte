<script lang="ts">
  import { toast } from '../lib/toast.svelte';
</script>

{#if toast.toasts.length > 0}
  <div class="toast-viewport" aria-live="polite" role="region">
    {#each toast.toasts as t (t.id)}
      <div class="toast-item {t.type}">
        <div class="toast-icon">
          {#if t.type === 'success'}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20 6L9 17l-5-5" />
            </svg>
          {:else if t.type === 'error'}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" />
              <path d="M15 9l-6 6M9 9l6 6" />
            </svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" />
              <path d="M12 16v-4M12 8h.01" />
            </svg>
          {/if}
        </div>

        <span class="toast-msg">{t.message}</span>

        <button class="toast-close" onclick={() => toast.remove(t.id)} aria-label="Close">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .toast-viewport {
    position: fixed;
    bottom: 24px;
    right: 24px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 999999;
    pointer-events: none;
    max-width: 420px;
    width: calc(100vw - 32px);
  }

  .toast-item {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    border-radius: var(--radius-md, 8px);
    background: var(--surface, #1e202e);
    color: var(--ink, #ffffff);
    border: 1px solid var(--border, #2d3148);
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.35);
    animation: toast-in 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .toast-icon {
    width: 18px;
    height: 18px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .toast-icon svg {
    width: 18px;
    height: 18px;
  }

  .toast-item.success {
    border-left: 4px solid var(--green, #10b981);
  }
  .toast-item.success .toast-icon {
    color: var(--green, #10b981);
  }

  .toast-item.error {
    border-left: 4px solid var(--red, #ef4444);
  }
  .toast-item.error .toast-icon {
    color: var(--red, #ef4444);
  }

  .toast-item.info {
    border-left: 4px solid var(--accent, #6366f1);
  }
  .toast-item.info .toast-icon {
    color: var(--accent, #6366f1);
  }

  .toast-msg {
    flex: 1;
    font-size: 13.5px;
    line-height: 1.4;
    word-break: break-word;
  }

  .toast-close {
    width: 22px;
    height: 22px;
    border: none;
    background: transparent;
    color: var(--mist, #94a3b8);
    cursor: pointer;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    flex-shrink: 0;
    transition: color 0.12s;
  }
  .toast-close svg {
    width: 14px;
    height: 14px;
  }
  .toast-close:hover {
    color: var(--ink, #ffffff);
  }

  @keyframes toast-in {
    from {
      transform: translateY(12px) scale(0.96);
      opacity: 0;
    }
    to {
      transform: translateY(0) scale(1);
      opacity: 1;
    }
  }
</style>
