<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Account } from '../lib/api';

  let { onAuthed }: { onAuthed: (u: Account) => void } = $props();

  let mode = $state<'loading' | 'setup' | 'login' | 'error'>('loading');
  let username = $state('');
  let password = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);

  async function checkSetup() {
    mode = 'loading';
    error = '';
    try {
      const s = await api.setupStatus();
      mode = s.needs_setup ? 'setup' : 'login';
    } catch (err) {
      // Don't silently fall back to login — that locks a fresh install out.
      // Surface the real problem (gateway down / not rebuilt with auth).
      error = readError(err);
      mode = 'error';
    }
  }

  onMount(checkSetup);

  function readError(err: any): string {
    const data = err?.response?.data;
    if (typeof data === 'string' && data) return data;
    return err?.message ?? 'Something went wrong';
  }

  async function submit(e: Event) {
    e.preventDefault();
    error = '';
    if (mode === 'setup') {
      if (password.length < 8) { error = 'Password must be at least 8 characters'; return; }
      if (password !== confirm) { error = 'Passwords do not match'; return; }
    }
    busy = true;
    try {
      const user = mode === 'setup'
        ? await api.setup(username.trim(), password)
        : await api.login(username.trim(), password);
      onAuthed(user);
    } catch (err) {
      error = readError(err);
    } finally {
      busy = false;
    }
  }
</script>

<div class="auth-wrap">
  <div class="auth-card">
    <div class="auth-brand">
      <div class="auth-mark">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none"><path d="M3 16L12 20L21 16M3 12L12 16L21 12M12 3L21 8L12 13L3 8L12 3Z" stroke="white" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </div>
      <div class="auth-brand-name">Beacon</div>
    </div>

    {#if mode === 'loading'}
      <div class="auth-loading">Loading…</div>
    {:else if mode === 'error'}
      <h1 class="auth-title">Can't reach the gateway</h1>
      <p class="auth-sub">
        The dashboard couldn't confirm whether an account exists. Make sure the
        gateway is running and has been rebuilt with the auth endpoints.
      </p>
      <div class="auth-error">{error}</div>
      <button class="auth-btn" onclick={checkSetup}>Retry</button>
    {:else}
      <h1 class="auth-title">{mode === 'setup' ? 'Create your admin account' : 'Sign in'}</h1>
      <p class="auth-sub">
        {mode === 'setup'
          ? 'This is the first account and will be the administrator.'
          : 'Enter your credentials to access the console.'}
      </p>

      <form onsubmit={submit}>
        <label class="auth-label" for="u">Username</label>
        <input id="u" class="auth-input" type="text" bind:value={username} autocomplete="username" required />

        <label class="auth-label" for="p">Password</label>
        <input id="p" class="auth-input" type="password" bind:value={password}
          autocomplete={mode === 'setup' ? 'new-password' : 'current-password'} required />

        {#if mode === 'setup'}
          <label class="auth-label" for="c">Confirm password</label>
          <input id="c" class="auth-input" type="password" bind:value={confirm} autocomplete="new-password" required />
        {/if}

        {#if error}<div class="auth-error">{error}</div>{/if}

        <button class="auth-btn" type="submit" disabled={busy}>
          {busy ? 'Please wait…' : mode === 'setup' ? 'Create account' : 'Sign in'}
        </button>
      </form>
    {/if}
  </div>
</div>

<style>
  .auth-wrap {
    position: fixed; inset: 0; display: flex; align-items: center; justify-content: center;
    background: var(--canvas); padding: 20px;
  }
  .auth-card {
    width: 380px; max-width: 100%; background: var(--surface);
    border: 1px solid var(--border); border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md); padding: 32px 30px;
  }
  .auth-brand { display: flex; align-items: center; gap: 10px; margin-bottom: 22px; }
  .auth-mark {
    width: 34px; height: 34px; border-radius: 10px;
    background: linear-gradient(155deg, var(--accent), #7B8CF0);
    display: flex; align-items: center; justify-content: center; box-shadow: var(--shadow-sm);
  }
  .auth-brand-name { font-family: 'Manrope', sans-serif; font-weight: 800; font-size: 18px; letter-spacing: -.02em; }
  .auth-title { font-family: 'Manrope', sans-serif; font-size: 20px; font-weight: 800; margin: 0 0 4px; }
  .auth-sub { color: var(--mist); font-size: 13px; margin: 0 0 22px; }
  .auth-loading { color: var(--mist); font-size: 14px; padding: 30px 0; text-align: center; }
  .auth-label { display: block; font-size: 12px; font-weight: 650; color: var(--mist); margin: 12px 0 5px; }
  .auth-input {
    width: 100%; padding: 10px 12px; border: 1px solid var(--border); border-radius: var(--radius-sm);
    background: var(--surface-2); color: var(--ink); font-size: 14px; font-family: inherit; outline: none;
    transition: border-color .15s;
  }
  .auth-input:focus { border-color: var(--accent); }
  .auth-error {
    margin-top: 14px; background: var(--red-tint); color: var(--red);
    border-radius: var(--radius-sm); padding: 9px 12px; font-size: 12.5px; font-weight: 600;
  }
  .auth-btn {
    width: 100%; margin-top: 20px; padding: 11px; border: none; border-radius: var(--radius-sm);
    background: var(--accent); color: #fff; font-size: 14px; font-weight: 700; font-family: inherit;
    transition: background .15s, opacity .15s;
  }
  .auth-btn:hover:not(:disabled) { background: var(--accent-ink); }
  .auth-btn:disabled { opacity: .6; cursor: default; }
</style>
