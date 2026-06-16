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

  const projectLinks = [
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

  $: updateMessage = (() => {
    if ($updateStatus === 'checking') return $t('about.checking');
    if ($updateStatus === 'upToDate') return $t('about.upToDate');
    if ($updateStatus === 'available' && $availableUpdate) {
      return tWithVersion('about.updateAvailable', $availableUpdate.version);
    }
    if ($updateStatus === 'downloading') {
      return `${$t('about.downloading')} ${Math.round($updateProgress * 100)}%`;
    }
    if ($updateStatus === 'ready') return $t('about.updateReady');
    if ($updateStatus === 'error' && $updateError) {
      return `${$t('about.updateError')}: ${$updateError}`;
    }
    if ($updateBackend === 'sparkle') return $t('about.sparkleHint');
    return $t('about.updatesIdle');
  })();

  $: updateMessageClass =
    $updateStatus === 'upToDate' || $updateStatus === 'ready'
      ? 'ok'
      : $updateStatus === 'error'
        ? 'err'
        : $updateStatus === 'available'
          ? 'pending'
          : '';
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

  <div class="top-grid">
    <section class="story-card panel">
      <span class="eyebrow">{$t('about.originTitle')}</span>
      <p class="origin-body">{$t('about.originBody')}</p>
      <p class="origin-footer">
        <span class="origin-name">{$t('about.originSignature')}</span>
        <span class="origin-sep" aria-hidden="true">·</span>
        <button class="text-link" on:click={() => openExternal('https://github.com/Ketlark')}>
          {$t('about.originGithub')}
        </button>
      </p>
    </section>

    <section class="update-panel panel" aria-label={$t('about.updatesTitle')}>
      <div class="update-top">
        <span class="eyebrow">{$t('about.updatesTitle')}</span>
        {#if $updateStatus === 'available' && $availableUpdate}
          <span class="update-tag">v{$availableUpdate.version}</span>
        {/if}
      </div>
      <p
        class="update-message"
        class:ok={updateMessageClass === 'ok'}
        class:err={updateMessageClass === 'err'}
        class:pending={updateMessageClass === 'pending'}
      >
        {updateMessage}
      </p>
      {#if $updateStatus === 'downloading'}
        <div class="progress-track" aria-hidden="true">
          <div class="progress-fill" style:width="{$updateProgress * 100}%"></div>
        </div>
      {/if}
      {#if $updateStatus === 'available' && $availableUpdate?.releaseNotes}
        <pre class="release-notes">{$availableUpdate.releaseNotes.trim().slice(0, 320)}{$availableUpdate.releaseNotes.length > 320 ? '…' : ''}</pre>
      {/if}
      <div class="update-actions">
        {#if $updateBackend === 'sparkle'}
          <button class="btn-primary" disabled={$updateBusy} on:click={handleCheckUpdates}>
            {$updateBusy ? $t('about.checking') : $t('about.checkUpdates')}
          </button>
        {:else if $updateStatus === 'ready'}
          <button class="btn-primary" on:click={restartApp}>{$t('about.restartNow')}</button>
        {:else if $updateStatus === 'available' && $availableUpdate}
          <button class="btn-primary" disabled={$updateBusy} on:click={downloadAndApplyUpdate}>
            {$t('about.downloadUpdate')}
          </button>
          <button class="btn-secondary" on:click={() => openExternal($availableUpdate.releaseUrl)}>
            {$t('about.viewRelease')}
          </button>
        {:else if $updateStatus !== 'downloading'}
          <button class="btn-primary" disabled={$updateBusy} on:click={handleCheckUpdates}>
            {$updateBusy ? $t('about.checking') : $t('about.checkUpdates')}
          </button>
        {/if}
      </div>
    </section>
  </div>

  <div class="bottom-grid">
    <section class="links-panel panel">
      <span class="eyebrow">{$t('about.linksTitle')}</span>
      <div class="link-list">
        {#each projectLinks as link}
          <button class="link-row" on:click={() => openExternal(link.url)}>
            <span>
              <strong>{$t(link.labelKey)}</strong>
              <small>{$t(link.descriptionKey)}</small>
            </span>
            <span class="link-action">{$t('about.open')}</span>
          </button>
        {/each}
      </div>
    </section>

    <section class="support-panel panel">
      <span class="eyebrow">{$t('about.supportTitle')}</span>
      <h3>{$t('about.supportHeading')}</h3>
      <p>{$t('about.supportBody')}</p>
      <div class="support-actions">
        <button class="btn-primary support-main" on:click={() => openExternal('https://github.com/sponsors/Ketlark')}>
          {$t('about.supportDonate')}
        </button>
        <button class="btn-secondary" on:click={() => openExternal('https://github.com/Ketlark/PKGForge/stargazers')}>
          {$t('about.supportStar')}
        </button>
      </div>
    </section>
  </div>

  <section class="legal-note panel">
    <strong>{$t('about.legalTitle')}</strong>
    <p>{$t('about.legalBody')}</p>
  </section>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 20px;
  }

  .panel {
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: rgba(24, 24, 31, 0.62);
  }

  .intro {
    display: flex;
    gap: 16px;
    align-items: center;
    padding: 20px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: rgba(24, 24, 31, 0.62);
  }

  .app-mark-frame {
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 0 0 auto;
    width: 72px;
    height: 72px;
    border-radius: 16px;
    background:
      radial-gradient(circle at 50% 48%, rgba(255, 154, 33, 0.18), transparent 52%),
      linear-gradient(145deg, rgba(74, 125, 255, 0.14), rgba(24, 24, 31, 0.92));
    border: 1px solid rgba(255, 255, 255, 0.08);
    overflow: hidden;
  }

  .app-mark {
    width: 64px;
    height: 64px;
    object-fit: contain;
  }

  .intro-copy {
    min-width: 0;
  }

  .eyebrow,
  .legal-note strong {
    font-size: 11px;
    font-weight: 700;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .intro h2 {
    margin: 4px 0 6px;
    font-size: 26px;
    line-height: 1.1;
    color: var(--text-primary);
  }

  .intro p,
  .origin-body,
  .support-panel p,
  .legal-note p,
  .link-row small,
  .update-message {
    margin: 0;
    color: var(--text-secondary);
    line-height: 1.5;
    font-size: 13px;
  }

  .meta-row {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 12px;
  }

  .meta-row span,
  .version-pill {
    padding: 3px 8px;
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

  .top-grid,
  .bottom-grid {
    display: grid;
    gap: 14px;
    align-items: stretch;
  }

  .top-grid {
    grid-template-columns: minmax(0, 1.25fr) minmax(280px, 0.9fr);
  }

  .bottom-grid {
    grid-template-columns: minmax(0, 1.1fr) minmax(0, 0.9fr);
  }

  .story-card,
  .links-panel {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
  }

  .origin-body {
    line-height: 1.55;
  }

  .origin-footer {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    margin: 0;
    font-size: 12px;
  }

  .origin-name {
    color: var(--text-primary);
    font-weight: 600;
  }

  .origin-sep {
    color: var(--text-muted);
  }

  .text-link {
    margin: 0;
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

  .update-panel {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px;
    border-color: rgba(74, 125, 255, 0.22);
  }

  .update-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .update-message.ok {
    color: #86efac;
  }

  .update-message.err {
    color: #fca5a5;
  }

  .update-message.pending {
    color: var(--accent-light);
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

  .update-actions,
  .support-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: auto;
  }

  .release-notes {
    margin: 0;
    padding: 8px 10px;
    max-height: 88px;
    overflow: auto;
    font-family: inherit;
    font-size: 11px;
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
    padding: 10px 12px;
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

  .support-panel {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px;
    border-color: rgba(74, 125, 255, 0.35);
    background:
      linear-gradient(135deg, rgba(74, 125, 255, 0.12), rgba(24, 24, 31, 0.72)),
      var(--bg-elevated);
  }

  .support-panel h3 {
    margin: 0;
    color: var(--text-primary);
    font-size: 17px;
    line-height: 1.2;
  }

  .support-main {
    box-shadow: 0 0 16px var(--accent-glow);
  }

  .legal-note {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 12px;
    align-items: start;
    padding: 12px 14px;
  }

  .legal-note strong {
    color: var(--warning);
  }

  .legal-note p {
    font-size: 12px;
  }

  @media (max-width: 820px) {
    .top-grid,
    .bottom-grid {
      grid-template-columns: 1fr;
    }

    .intro {
      align-items: flex-start;
    }
  }
</style>
