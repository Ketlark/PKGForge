<script lang="ts">
  import { onMount } from 'svelte';
  import MergePage from './lib/components/MergePage.svelte';
  import SplitPage from './lib/components/SplitPage.svelte';
  import InspectPage from './lib/components/InspectPage.svelte';
  import PS1Page from './lib/components/PS1Page.svelte';
  import PS2Page from './lib/components/PS2Page.svelte';
  import ActivityLog from './lib/components/ActivityLog.svelte';
  import SettingsPage from './lib/components/SettingsPage.svelte';
  import AboutPage from './lib/components/AboutPage.svelte';
  import appIcon from './assets/pkg-forge-icon.png';
  import { logCount } from './lib/stores/activity';
  import { t, locale } from './lib/stores/i18n';
  import type { Locale } from './lib/stores/i18n';
  import { updateBadge } from './lib/stores/update';
  import { initUpdateOnStartup } from './lib/stores/update';
  import { LoadConfig } from '../wailsjs/go/main/App';

  type Tab = 'merge' | 'split' | 'inspect' | 'ps1' | 'ps2' | 'activity' | 'settings' | 'about';
  let activeTab: Tab = 'merge';

  $: tabs = [
    { id: 'merge' as Tab, label: $t('tab.merge'), icon: '📦', shortcut: '1' },
    { id: 'split' as Tab, label: $t('tab.split'), icon: '✂️', shortcut: '2' },
    { id: 'inspect' as Tab, label: $t('tab.inspect'), icon: '🔍', shortcut: '3' },
    { id: 'ps1' as Tab, label: $t('tab.ps1'), icon: '💿', shortcut: '4' },
    { id: 'ps2' as Tab, label: $t('tab.ps2'), icon: '📀', shortcut: '5' },
    { id: 'activity' as Tab, label: $t('tab.activity'), icon: '📋', shortcut: '6' },
    { id: 'settings' as Tab, label: $t('tab.settings'), icon: '⚙️', shortcut: '7' },
  ];

  onMount(() => {
    void (async () => {
      try {
        const cfg = await LoadConfig();
        if (cfg.language === 'fr' || cfg.language === 'en') {
          locale.set(cfg.language as Locale);
        }
        const checkOnStartup = cfg.checkUpdatesOnStartup ?? true;
        await initUpdateOnStartup(checkOnStartup);
      } catch {
        await initUpdateOnStartup(true);
      }
    })();

    function handleKeydown(e: KeyboardEvent) {
      if (e.metaKey || e.ctrlKey) {
        const idx = parseInt(e.key) - 1;
        if (idx >= 0 && idx < tabs.length) {
          e.preventDefault();
          activeTab = tabs[idx].id;
        } else if (e.key === '8') {
          e.preventDefault();
          activeTab = 'about';
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
      <img class="header-icon" src={appIcon} alt="" aria-hidden="true" />
      <span class="header-text">{$t('app.title')}</span>
    </div>
    <nav class="tab-bar">
      {#each tabs as tab (tab.id)}
        <button
          class="tab"
          class:active={activeTab === tab.id}
          on:click={() => (activeTab = tab.id)}
          title="⌘/Ctrl+{tab.shortcut}"
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
    {:else if activeTab === 'ps1'}
      <PS1Page />
    {:else if activeTab === 'ps2'}
      <PS2Page />
    {:else if activeTab === 'activity'}
      <ActivityLog />
    {:else if activeTab === 'settings'}
      <SettingsPage on:openAbout={() => (activeTab = 'about')} />
    {:else}
      <AboutPage />
    {/if}
  </main>

  <footer class="status-bar">
    <div class="status-left">
      <span>{$t('app.subtitle')}</span>
      <button
        class="status-link"
        class:active={activeTab === 'about'}
        on:click={() => (activeTab = 'about')}
        title="⌘/Ctrl+8"
      >
        {$t('tab.about')}
        {#if $updateBadge}
          <span class="update-dot" title={$t('about.updateBadge')}></span>
        {/if}
      </button>
    </div>
    <span class="shortcuts-hint">⌘/Ctrl+1-7 tabs · 8 {$t('tab.about')}</span>
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
    width: 22px;
    height: 22px;
    object-fit: contain;
    filter: drop-shadow(0 0 8px rgba(74, 125, 255, 0.22));
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
    min-width: 0;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 6px 10px;
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
    align-items: center;
    justify-content: space-between;
    padding: 4px 16px;
    font-size: 11px;
    color: var(--text-muted);
    background: var(--bg-surface);
    border-top: 1px solid var(--border);
    flex-shrink: 0;
  }

  .status-left {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }

  .status-link {
    padding: 0;
    color: var(--text-secondary);
    background: none;
    border: none;
    font: inherit;
    font-size: 11px;
    cursor: pointer;
    opacity: 0.75;
    -webkit-app-region: no-drag;
  }

  .status-link:hover,
  .status-link.active {
    color: var(--accent-light);
    opacity: 1;
  }

  .update-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    margin-left: 5px;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 8px var(--accent-glow);
    vertical-align: middle;
  }

  .shortcuts-hint {
    font-size: 10px;
    color: var(--text-muted);
    opacity: 0.6;
  }
</style>
