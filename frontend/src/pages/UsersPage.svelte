<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Account, type Role } from '../lib/api';
  import { toast } from '../lib/toast.svelte';

  let { me }: { me: Account } = $props();

  let users = $state<Account[]>([]);
  let loading = $state(true);
  let error = $state('');

  // Create form
  let nu = $state('');
  let np = $state('');
  let nr = $state<Role>('viewer');
  let creating = $state(false);

  function readError(err: any): string {
    const data = err?.response?.data;
    if (typeof data === 'string' && data) return data;
    return err?.message ?? 'Request failed';
  }

  async function load() {
    loading = true;
    error = '';
    try {
      users = await api.listUsers();
    } catch (err) {
      error = readError(err);
    } finally {
      loading = false;
    }
  }

  onMount(load);

  async function createUser(e: Event) {
    e.preventDefault();
    creating = true;
    try {
      await api.createUser(nu.trim(), np, nr);
      nu = '';
      np = '';
      nr = 'viewer';
      toast.success('User created');
      await load();
    } catch (err) {
      toast.error(readError(err));
    } finally {
      creating = false;
    }
  }

  async function changeRole(u: Account, role: Role) {
    if (u.role === role) return;
    try {
      await api.updateUser(u.id, { role });
      toast.success(`${u.username} is now ${role}`);
      await load();
    } catch (err) {
      toast.error(readError(err));
      await load();
    }
  }

  async function resetPassword(u: Account) {
    const pw = prompt(`New password for ${u.username} (min 8 chars):`);
    if (!pw) return;
    try {
      await api.updateUser(u.id, { password: pw });
      toast.success(`Password updated for ${u.username}`);
    } catch (err) {
      toast.error(readError(err));
    }
  }

  async function removeUser(u: Account) {
    if (!confirm(`Delete user "${u.username}"? This cannot be undone.`)) return;
    try {
      await api.deleteUser(u.id);
      toast.info(`User "${u.username}" removed`);
      await load();
    } catch (err) {
      toast.error(readError(err));
    }
  }
</script>

<div class="users">
  <form class="create-card" onsubmit={createUser}>
    <div class="create-title">Add a user</div>
    <div class="create-row">
      <input class="ipt" placeholder="Username" bind:value={nu} required />
      <input class="ipt" type="password" placeholder="Password (min 8)" bind:value={np} required />
      <select class="ipt sel" bind:value={nr}>
        <option value="viewer">Viewer</option>
        <option value="admin">Admin</option>
      </select>
      <button class="btn-primary" type="submit" disabled={creating}>{creating ? 'Adding…' : 'Add user'}</button>
    </div>
    <div class="create-hint">Viewers have read-only access. Admins can perform every action and manage users.</div>
  </form>

  {#if loading}
    <div class="muted">Loading users…</div>
  {:else if error}
    <div class="err-box">{error}</div>
  {:else}
    <div class="table">
      <div class="thead">
        <div>User</div><div>Role</div><div>Created</div><div class="ta-r">Actions</div>
      </div>
      {#each users as u (u.id)}
        <div class="trow">
          <div class="cell-user">
            <span class="avatar">{u.username.slice(0, 1).toUpperCase()}</span>
            <span class="uname">{u.username}{#if u.id === me.id}<span class="you">you</span>{/if}</span>
          </div>
          <div>
            <select class="role-sel" value={u.role} onchange={(e) => changeRole(u, (e.currentTarget as HTMLSelectElement).value as Role)}>
              <option value="viewer">Viewer</option>
              <option value="admin">Admin</option>
            </select>
          </div>
          <div class="muted sm">{u.created_at ? new Date(u.created_at).toLocaleString() : '—'}</div>
          <div class="ta-r">
            <button class="btn-ghost" onclick={() => resetPassword(u)}>Reset password</button>
            <button class="btn-danger" onclick={() => removeUser(u)} disabled={u.id === me.id}>Delete</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .users { display: flex; flex-direction: column; gap: 20px; }
  .create-card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); padding: 18px 20px; }
  .create-title { font-weight: 700; font-size: 14px; margin-bottom: 12px; }
  .create-row { display: flex; gap: 10px; flex-wrap: wrap; }
  .ipt { padding: 9px 11px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface-2); color: var(--ink); font-size: 13.5px; font-family: inherit; outline: none; }
  .ipt:focus { border-color: var(--accent); }
  .create-row .ipt:not(.sel) { flex: 1; min-width: 160px; }
  .sel { min-width: 120px; }
  .create-hint { color: var(--mist-2); font-size: 12px; margin-top: 10px; }
  .btn-primary { padding: 9px 16px; border: none; border-radius: var(--radius-sm); background: var(--accent); color: #fff; font-weight: 700; font-size: 13.5px; }
  .btn-primary:hover:not(:disabled) { background: var(--accent-ink); }
  .btn-primary:disabled { opacity: .6; }

  .table { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); overflow: hidden; }
  .thead, .trow { display: grid; grid-template-columns: 2fr 1fr 1.5fr 1.5fr; align-items: center; gap: 12px; padding: 12px 18px; }
  .thead { font-size: 11px; text-transform: uppercase; letter-spacing: .06em; color: var(--mist-2); font-weight: 700; border-bottom: 1px solid var(--border); }
  .trow { border-top: 1px solid var(--border); }
  .cell-user { display: flex; align-items: center; gap: 10px; }
  .avatar { width: 28px; height: 28px; border-radius: 50%; background: var(--accent-tint); color: var(--accent-ink); display: flex; align-items: center; justify-content: center; font-weight: 700; font-size: 12px; }
  .uname { font-weight: 650; }
  .you { margin-left: 8px; font-size: 10.5px; font-weight: 700; color: var(--accent-ink); background: var(--accent-tint); padding: 1px 6px; border-radius: 20px; }
  .role-sel { padding: 5px 8px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface-2); color: var(--ink); font-size: 13px; font-family: inherit; }
  .muted { color: var(--mist); } .sm { font-size: 12.5px; }
  .ta-r { text-align: right; }
  .btn-ghost { background: none; border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 5px 10px; font-size: 12.5px; color: var(--mist); margin-right: 6px; }
  .btn-ghost:hover { color: var(--ink); border-color: var(--border-strong); }
  .btn-danger { background: none; border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 5px 10px; font-size: 12.5px; color: var(--red); }
  .btn-danger:hover:not(:disabled) { background: var(--red-tint); }
  .btn-danger:disabled { opacity: .4; cursor: default; }
  .err-box { background: var(--red-tint); color: var(--red); border-radius: var(--radius-sm); padding: 12px 14px; font-size: 13px; }
</style>
