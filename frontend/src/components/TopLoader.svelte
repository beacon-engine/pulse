<script lang="ts">
    import { loadingStore } from "../stores/loading.svelte";
</script>

{#if loadingStore.isLoading}
  <div class="top-loader" aria-hidden="true"></div>
{/if}

<style>
  .top-loader {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    height: 2.5px;
    background: linear-gradient(
      90deg,
      rgba(79, 70, 229, 0) 0%,
      var(--accent, #4f46e5) 50%,
      rgba(79, 70, 229, 0) 100%
    );
    background-size: 200% 100%;
    animation: top-bar-slide 1s infinite ease-in-out;
    z-index: 99999;
    pointer-events: none;
  }

  @keyframes top-bar-slide {
    0% { background-position: -150% 0; }
    100% { background-position: 150% 0; }
  }
</style>
