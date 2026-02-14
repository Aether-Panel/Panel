<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { page } from '$app/stores'
  import { api } from '$lib/api'
  import { t } from '$lib/i18n'
  import Icon from '$lib/components/Icon.svelte'
  
  // Estado reactivo - ¡Así de simple!
  let servers = []
  let loading = true
  let error = null
  
  // Computed values - automático con $:
  $: totalServers = servers.length
  $: onlineServers = servers.filter(s => s.status === 'online').length
  
  // Lifecycle
  onMount(async () => {
    try {
      servers = await api.servers.list()
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  })
  
  // Funciones
  async function startServer(id: string) {
    await api.servers.start(id)
    // Recargar lista
    servers = await api.servers.list()
  }
  
  function navigateToServer(id: string) {
    goto(`/servers/${id}`)
  }
</script>

<!-- Template - Muy similar a Vue pero más simple -->
<div class="container mx-auto p-6">
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-3xl font-bold">{$t('servers.title')}</h1>
    <button 
      class="btn btn-primary"
      on:click={() => goto('/servers/create')}
    >
      <Icon name="plus" />
      {$t('servers.create')}
    </button>
  </div>
  
  <!-- Stats -->
  <div class="grid grid-cols-3 gap-4 mb-6">
    <div class="stat-card">
      <h3>{$t('servers.total')}</h3>
      <p class="text-4xl font-bold">{totalServers}</p>
    </div>
    <div class="stat-card">
      <h3>{$t('servers.online')}</h3>
      <p class="text-4xl font-bold text-green-500">{onlineServers}</p>
    </div>
    <div class="stat-card">
      <h3>{$t('servers.offline')}</h3>
      <p class="text-4xl font-bold text-red-500">{totalServers - onlineServers}</p>
    </div>
  </div>
  
  <!-- Loading state -->
  {#if loading}
    <div class="flex justify-center py-12">
      <div class="spinner"></div>
    </div>
  
  <!-- Error state -->
  {:else if error}
    <div class="alert alert-error">
      <Icon name="alert-circle" />
      <p>{error}</p>
    </div>
  
  <!-- Server list -->
  {:else if servers.length > 0}
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each servers as server (server.id)}
        <div 
          class="server-card"
          on:click={() => navigateToServer(server.id)}
        >
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-xl font-semibold">{server.name}</h3>
            <span 
              class="status-badge"
              class:online={server.status === 'online'}
              class:offline={server.status === 'offline'}
            >
              {server.status}
            </span>
          </div>
          
          <div class="server-info">
            <p><Icon name="cpu" /> {server.cpu}%</p>
            <p><Icon name="memory" /> {server.memory}MB</p>
            <p><Icon name="users" /> {server.players}/{server.maxPlayers}</p>
          </div>
          
          <div class="flex gap-2 mt-4">
            <button 
              class="btn btn-sm btn-success"
              on:click|stopPropagation={() => startServer(server.id)}
              disabled={server.status === 'online'}
            >
              <Icon name="play" />
              {$t('servers.start')}
            </button>
            <button 
              class="btn btn-sm btn-error"
              on:click|stopPropagation={() => api.servers.stop(server.id)}
              disabled={server.status === 'offline'}
            >
              <Icon name="stop" />
              {$t('servers.stop')}
            </button>
          </div>
        </div>
      {/each}
    </div>
  
  <!-- Empty state -->
  {:else}
    <div class="empty-state">
      <Icon name="server" size="64" />
      <h3>{$t('servers.noServers')}</h3>
      <p>{$t('servers.createFirst')}</p>
      <button 
        class="btn btn-primary mt-4"
        on:click={() => goto('/servers/create')}
      >
        {$t('servers.create')}
      </button>
    </div>
  {/if}
</div>

<style>
  /* CSS con scope automático - no necesitas módulos CSS */
  .container {
    max-width: 1200px;
  }
  
  .stat-card {
    @apply bg-card rounded-lg p-6 shadow-md;
  }
  
  .server-card {
    @apply bg-card rounded-lg p-6 shadow-md cursor-pointer;
    @apply transition-all duration-200;
    @apply hover:shadow-xl hover:scale-105;
  }
  
  .status-badge {
    @apply px-3 py-1 rounded-full text-sm font-semibold;
  }
  
  .status-badge.online {
    @apply bg-green-500/20 text-green-500;
  }
  
  .status-badge.offline {
    @apply bg-red-500/20 text-red-500;
  }
  
  .server-info {
    @apply flex gap-4 text-sm text-muted-foreground;
  }
  
  .empty-state {
    @apply flex flex-col items-center justify-center py-12 text-center;
  }
</style>
