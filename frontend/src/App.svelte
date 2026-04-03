<script lang="ts">
  import { onMount } from 'svelte';
  import MergePage from './lib/components/MergePage.svelte';
  import SplitPage from './lib/components/SplitPage.svelte';
  import InspectPage from './lib/components/InspectPage.svelte';
  import ActivityLog from './lib/components/ActivityLog.svelte';
  import SettingsPage from './lib/components/SettingsPage.svelte';
  import { logCount } from './lib/stores/activity';
  import { t, locale } from './lib/stores/i18n';
  import type { Locale } from './lib/stores/i18n';
  import { LoadConfig } from '../wailsjs/go/main/App';

  type Tab = 'merge' | 'split' | 'inspect' | 'activity' | 'settings';
  let activeTab: Tab = 'merge';

  $: tabs = [
    { id: 'merge' as Tab, label: $t('tab.merge'), icon: '📦', shortcut: '1' },
    { id: 'split' as Tab, label: $t('tab.split'), icon: '✂️', shortcut: '2' },
    { id: 'inspect' as Tab, label: $t('tab.inspect'), icon: '🔍', shortcut: '3' },
    { id: 'activity' as Tab, label: $t('tab.activity'), icon: '📋', shortcut: '4' },
    { id: 'settings' as Tab, label: $t('tab.settings'), icon: '⚙️', shortcut: '5' },
  ];

  onMount(async () => {
    try {
      const cfg = await LoadConfig();
      if (cfg.language === 'fr' || cfg.language === 'en') {
        locale.set(cfg.language as Locale);
      }
    } catch {}

    function handleKeydown(e: KeyboardEvent) {
      if (e.metaKey || e.ctrlKey) {
        const idx = parseInt(e.key) - 1;
        if (idx >= 0 && idx < tabs.length) {
          e.preventDefault();
          activeTab = tabs[idx].id;
        }
      }
    }
    window.addEventListener('keydown', handleKeydown);
    return () => window.removeEventListener('keydown', handleKeydown);
  });
</script>

<div class="shell">
  <header class="header">
    <div class="header-title">
      <span class="header-icon">🎮</span>
      <span class="header-text">{$t('app.title')}</span>
    </div>
    <nav class="tab-bar">
      {#each tabs as tab (tab.id)}
        <button
          class="tab"
          class:active={activeTab === tab.id}
          on:click={() => (activeTab = tab.id)}
          title="⌘{tab.shortcut}"
        >
          <span class="tab-icon">{tab.icon}</span>
          <span class="tab-label">{tab.label}</span>
          {#if tab.id === 'activity' && $logCount > 0}
            <span class="tab-badge">{$logCount}</span>
          {/if}
        </button>
      {/each}
    </nav>
  </header>

  <main class="content">
    {#if activeTab === 'merge'}
      <MergePage />
    {:else if activeTab === 'split'}
      <SplitPage />
    {:else if activeTab === 'inspect'}
      <InspectPage />
    {:else if activeTab === 'activity'}
      <ActivityLog />
    {:else}
      <SettingsPage />
    {/if}
  </main>

  <footer class="status-bar">
    <span>{$t('app.subtitle')}</span>
    <span class="shortcuts-hint">⌘1-5 tabs</span>
  </footer>
</div>

<style>
  .shell {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 16px;
    height: 44px;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    -webkit-app-region: drag;
  }

  .header-title {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .header-icon {
    font-size: 16px;
  }

  .header-text {
    font-size: 14px;
    font-weight: 700;
    color: var(--text-primary);
    letter-spacing: -0.01em;
  }

  .tab-bar {
    display: flex;
    gap: 2px;
    -webkit-app-region: no-drag;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 6px 12px;
    font-size: 12px;
    font-weight: 500;
    font-family: var(--font);
    color: var(--text-muted);
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: all 0.15s;
    position: relative;
  }

  .tab:hover {
    color: var(--text-secondary);
    background: var(--bg-elevated);
  }

  .tab.active {
    color: var(--accent);
    background: var(--accent-soft);
  }

  .tab-icon {
    font-size: 12px;
  }

  .tab-label {
    font-size: 12px;
  }

  .tab-badge {
    font-size: 10px;
    font-weight: 700;
    color: #fff;
    background: var(--accent);
    padding: 0 5px;
    border-radius: 8px;
    min-width: 16px;
    height: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
  }

  .content {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
  }

  .status-bar {
    display: flex;
    justify-content: space-between;
    padding: 4px 16px;
    font-size: 11px;
    color: var(--text-muted);
    background: var(--bg-surface);
    border-top: 1px solid var(--border);
    flex-shrink: 0;
  }

  .shortcuts-hint {
    font-size: 10px;
    color: var(--text-muted);
    opacity: 0.6;
  }
</style>
