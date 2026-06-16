<script lang="ts">
  import { onMount } from 'svelte';
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
  import appIcon from '../../assets/pkg-forge-icon.png';
  import { t } from '../stores/i18n';
  import {
    currentVersion,
    availableUpdate,
    updateStatus,
    updateProgress,
    updateError,
    updateBusy,
    checkForUpdates,
    downloadAndApplyUpdate,
    restartApp,
    loadAppVersion,
    updateBadge,
    updateBackend,
    formatDisplayVersion,
  } from '../stores/update';

  const links = [
    {
      labelKey: 'about.link.repo',
      descriptionKey: 'about.link.repoDesc',
      url: 'https://github.com/Ketlark/PKGForge',
    },
    {
      labelKey: 'about.link.release',
      descriptionKey: 'about.link.releaseDesc',
      url: 'https://github.com/Ketlark/PKGForge/releases/latest',
    },
    {
      labelKey: 'about.link.issue',
      descriptionKey: 'about.link.issueDesc',
      url: 'https://github.com/Ketlark/PKGForge/issues',
    },
  ];

  onMount(() => {
    void loadAppVersion();
    updateBadge.set(false);
  });

  function openExternal(url: string) {
    if (typeof window !== 'undefined' && (window as any).runtime?.BrowserOpenURL) {
      BrowserOpenURL(url);
      return;
    }
    window.open(url, '_blank', 'noopener,noreferrer');
  }

  function formatVersion(version: string) {
    return formatDisplayVersion(version, $t('about.versionDev'));
  }

  function tWithVersion(key: string, version: string) {
    return $t(key).replace('{version}', version.startsWith('v') ? version : `v${version}`);
  }

  async function handleCheckUpdates() {
    updateBadge.set(false);
    await checkForUpdates();
  }
</script>

<div class="page">
  <section class="intro">
    <div class="app-mark-frame" aria-hidden="true">
      <img class="app-mark" src={appIcon} alt="" />
    </div>
    <div class="intro-copy">
      <span class="eyebrow">{$t('about.kicker')}</span>
      <h2>PKG Forge</h2>
      <p>{$t('about.summary')}</p>
      <div class="meta-row">
        <span>{$t('about.meta.license')}</span>
        <span>{$t('about.meta.stack')}</span>
        <span>{$t('about.meta.platforms')}</span>
        <span class="version-pill">{$t('about.version')} {formatVersion($currentVersion)}</span>
      </div>
    </div>
  </section>

  <section class="update-panel">
    <div class="update-header">
      <span class="eyebrow">{$t('about.updatesTitle')}</span>
      {#if $updateStatus === 'available' && $availableUpdate}
        <span class="update-tag">v{$availableUpdate.version}</span>
      {/if}
    </div>

    {#if $updateBackend === 'sparkle'}
      <p class="update-hint">{$t('about.sparkleHint')}</p>
      <div class="update-actions">
        <button class="btn-primary" disabled={$updateBusy} on:click={handleCheckUpdates}>
          {$t('about.checkUpdates')}
        </button>
      </div>
    {:else if $updateStatus === 'idle'}
      <p class="update-hint">{$t('about.checkUpdates')}</p>
    {:else if $updateStatus === 'checking'}
      <p class="update-hint">{$t('about.checking')}</p>
    {:else if $updateStatus === 'upToDate'}
      <p class="update-success">{$t('about.upToDate')}</p>
    {:else if $updateStatus === 'available' && $availableUpdate}
      <p class="update-hint">{tWithVersion('about.updateAvailable', $availableUpdate.version)}</p>
      {#if $availableUpdate.releaseNotes}
        <pre class="release-notes">{$availableUpdate.releaseNotes.trim().slice(0, 400)}{$availableUpdate.releaseNotes.length > 400 ? '…' : ''}</pre>
      {/if}
    {:else if $updateStatus === 'downloading'}
      <p class="update-hint">{$t('about.downloading')} {Math.round($updateProgress * 100)}%</p>
      <div class="progress-track" aria-hidden="true">
        <div class="progress-fill" style:width="{$updateProgress * 100}%"></div>
      </div>
    {:else if $updateStatus === 'ready'}
      <p class="update-success">{$t('about.updateReady')}</p>
    {:else if $updateStatus === 'error' && $updateError}
      <p class="update-error">{$t('about.updateError')}: {$updateError}</p>
    {/if}

    <div class="update-actions">
      {#if $updateBackend !== 'sparkle'}
        {#if $updateStatus === 'ready'}
          <button class="btn-primary" on:click={restartApp}>{$t('about.restartNow')}</button>
        {:else if $updateStatus === 'available' && $availableUpdate}
          <button class="btn-primary" disabled={$updateBusy} on:click={downloadAndApplyUpdate}>
            {$t('about.downloadUpdate')}
          </button>
          <button class="btn-secondary" on:click={() => openExternal($availableUpdate.releaseUrl)}>
            {$t('about.viewRelease')}
          </button>
        {:else}
          <button class="btn-primary" disabled={$updateBusy} on:click={handleCheckUpdates}>
            {#if $updateStatus === 'checking'}
              {$t('about.checking')}
            {:else}
              {$t('about.checkUpdates')}
            {/if}
          </button>
        {/if}
      {/if}
    </div>
  </section>

  <section class="support-panel">
    <div>
      <span class="eyebrow">{$t('about.supportTitle')}</span>
      <h3>{$t('about.supportHeading')}</h3>
      <p>{$t('about.supportBody')}</p>
    </div>
    <div class="support-actions">
      <button class="btn-primary support-main" on:click={() => openExternal('https://github.com/sponsors/Ketlark')}>
        {$t('about.supportDonate')}
      </button>
      <button class="btn-secondary" on:click={() => openExternal('https://github.com/Ketlark/PKGForge/stargazers')}>
        {$t('about.supportStar')}
      </button>
    </div>
  </section>

  <section class="details-grid">
    <div class="section">
      <h3>{$t('about.creatorLabel')}</h3>
      <div class="creator">
        <div class="avatar" aria-hidden="true">KD</div>
        <div>
          <strong>{$t('about.creatorName')}</strong>
          <p>{$t('about.creatorRole')}</p>
          <button class="text-link" on:click={() => openExternal('https://github.com/Ketlark')}>
            {$t('about.link.profile')}
          </button>
        </div>
      </div>
    </div>

    <div class="section">
      <h3>{$t('about.projectTitle')}</h3>
      <div class="link-list">
        {#each links as link}
          <button class="link-row" on:click={() => openExternal(link.url)}>
            <span>
              <strong>{$t(link.labelKey)}</strong>
              <small>{$t(link.descriptionKey)}</small>
            </span>
            <span class="link-action">{$t('about.open')}</span>
          </button>
        {/each}
      </div>
    </div>
  </section>

  <section class="legal-note">
    <strong>{$t('about.legalTitle')}</strong>
    <p>{$t('about.legalBody')}</p>
  </section>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 20px;
  }

  .intro,
  .support-panel,
  .update-panel,
  .section,
  .legal-note {
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: rgba(24, 24, 31, 0.62);
  }

  .intro {
    display: flex;
    gap: 16px;
    align-items: center;
    padding: 22px;
  }

  .app-mark-frame,
  .avatar {
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 0 0 auto;
    color: #fff;
    background: linear-gradient(135deg, var(--accent), #7c3aed);
    font-weight: 800;
    letter-spacing: 0.04em;
  }

  .app-mark-frame {
    width: 78px;
    height: 78px;
    border-radius: 18px;
    background:
      radial-gradient(circle at 50% 48%, rgba(255, 154, 33, 0.18), transparent 52%),
      linear-gradient(145deg, rgba(74, 125, 255, 0.14), rgba(24, 24, 31, 0.92));
    border: 1px solid rgba(255, 255, 255, 0.08);
    box-shadow:
      0 18px 40px rgba(0, 0, 0, 0.25),
      0 0 24px rgba(74, 125, 255, 0.16);
    overflow: hidden;
  }

  .app-mark {
    width: 70px;
    height: 70px;
    object-fit: contain;
    filter: drop-shadow(0 8px 18px rgba(0, 0, 0, 0.28));
  }

  .intro-copy {
    min-width: 0;
  }

  .eyebrow,
  .section h3,
  .legal-note strong {
    font-size: 12px;
    font-weight: 700;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .intro h2 {
    margin: 5px 0 8px;
    font-size: 28px;
    line-height: 1.1;
    color: var(--text-primary);
  }

  .intro p,
  .support-panel p,
  .creator p,
  .legal-note p,
  .link-row small,
  .update-hint,
  .update-success,
  .update-error {
    margin: 0;
    color: var(--text-secondary);
    line-height: 1.5;
  }

  .meta-row {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 14px;
  }

  .meta-row span,
  .version-pill {
    padding: 4px 8px;
    color: var(--text-secondary);
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 999px;
    font-size: 11px;
  }

  .version-pill {
    color: var(--accent-light);
    border-color: rgba(74, 125, 255, 0.35);
  }

  .update-panel {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px 18px;
    border-color: rgba(74, 125, 255, 0.28);
  }

  .update-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .update-tag {
    padding: 3px 8px;
    border-radius: 999px;
    font-size: 11px;
    font-weight: 700;
    color: var(--accent-light);
    background: var(--accent-soft);
    border: 1px solid rgba(74, 125, 255, 0.35);
  }

  .update-success {
    color: #86efac;
  }

  .update-error {
    color: #fca5a5;
  }

  .release-notes {
    margin: 0;
    padding: 10px 12px;
    max-height: 120px;
    overflow: auto;
    font-family: inherit;
    font-size: 12px;
    line-height: 1.45;
    color: var(--text-secondary);
    white-space: pre-wrap;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
  }

  .progress-track {
    height: 6px;
    border-radius: 999px;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--accent), #7c3aed);
    transition: width 0.15s ease;
  }

  .update-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .support-panel {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 18px;
    align-items: center;
    padding: 18px;
    border-color: rgba(74, 125, 255, 0.45);
    background:
      linear-gradient(135deg, rgba(74, 125, 255, 0.16), rgba(24, 24, 31, 0.72)),
      var(--bg-elevated);
  }

  .support-panel h3 {
    margin: 6px 0 8px;
    color: var(--text-primary);
    font-size: 18px;
  }

  .support-actions {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .support-main {
    box-shadow: 0 0 16px var(--accent-glow);
  }

  .details-grid {
    display: grid;
    grid-template-columns: 0.85fr 1.15fr;
    gap: 16px;
  }

  .section {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 16px;
  }

  .section h3 {
    margin: 0;
  }

  .creator {
    display: flex;
    gap: 12px;
    align-items: center;
  }

  .avatar {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    font-size: 14px;
  }

  .creator strong {
    display: block;
    margin-bottom: 3px;
    color: var(--text-primary);
  }

  .text-link {
    margin-top: 7px;
    padding: 0;
    color: var(--accent-light);
    background: none;
    border: none;
    cursor: pointer;
    font: inherit;
    font-size: 12px;
  }

  .text-link:hover {
    color: var(--accent);
  }

  .link-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .link-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 12px;
    align-items: center;
    width: 100%;
    padding: 11px 12px;
    color: inherit;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    font-family: inherit;
    transition:
      border-color 0.15s,
      background 0.15s;
  }

  .link-row:hover {
    border-color: var(--accent);
    background: var(--accent-soft);
  }

  .link-row strong,
  .link-row small {
    display: block;
  }

  .link-row strong {
    margin-bottom: 2px;
    color: var(--text-primary);
    font-size: 13px;
  }

  .link-row small {
    font-size: 12px;
  }

  .link-action {
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
  }

  .legal-note {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 12px;
    align-items: start;
    padding: 13px 15px;
  }

  .legal-note strong {
    color: var(--warning);
  }

  .legal-note p {
    font-size: 12px;
  }

  @media (max-width: 820px) {
    .intro,
    .support-panel,
    .details-grid,
    .legal-note {
      grid-template-columns: 1fr;
    }

    .intro {
      align-items: flex-start;
    }

    .support-actions {
      justify-content: flex-start;
    }
  }
</style>
