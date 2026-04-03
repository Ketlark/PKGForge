<script lang="ts">
  import DropZone from './DropZone.svelte';
  import ProgressBar from './ProgressBar.svelte';
  import { formatSize } from '../utils/format';
  import { t } from '../stores/i18n';
  import { addLog } from '../stores/activity';
  import {
    InspectPKG,
    OpenFileDialog,
    CalculateChecksum,
    CancelOperation,
    SuggestRenamePKG,
    RenamePKG,
  } from '../../../wailsjs/go/main/App';
  import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime';
  import type { core } from '../../../wailsjs/go/models';

  let info: core.PKGInfo | null = null;
  let filePath = '';
  let checksum: core.ChecksumResult | null = null;
  let checksumRunning = false;
  let checksumProgress = 0;
  let suggestedName = '';

  async function inspectFile(path: string) {
    filePath = path;
    checksum = null;
    suggestedName = '';
    checksumProgress = 0;

    info = await InspectPKG(path);
    if (info.valid) {
      addLog('info', `Inspected: ${info.contentId || path.split('/').pop()}`);
      const suggestion = await SuggestRenamePKG(path);
      suggestedName = suggestion.newName;
    } else {
      addLog('warning', `Invalid PKG: ${info.error}`);
    }
  }

  async function handleBrowse() {
    const path = await OpenFileDialog();
    if (path) await inspectFile(path);
  }

  async function handleDrop(e: CustomEvent<string[]>) {
    if (e.detail.length > 0) await inspectFile(e.detail[0]);
  }

  async function handleChecksum() {
    if (!filePath) return;
    checksumRunning = true;
    checksumProgress = 0;

    EventsOn('checksum-progress', (pct: number) => {
      checksumProgress = pct;
    });

    try {
      checksum = await CalculateChecksum(filePath);
      addLog('success', `SHA-256: ${checksum.sha256.substring(0, 16)}...`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg !== 'cancelled') addLog('error', `Checksum failed: ${msg}`);
    } finally {
      EventsOff('checksum-progress');
      checksumRunning = false;
    }
  }

  async function handleRename() {
    if (!filePath) return;
    try {
      const newPath = await RenamePKG(filePath);
      addLog('success', `Renamed → ${newPath.split('/').pop()}`);
      filePath = newPath;
    } catch (err) {
      addLog('error', `Rename failed: ${err instanceof Error ? err.message : err}`);
    }
  }

  function clear() {
    info = null;
    filePath = '';
    checksum = null;
    suggestedName = '';
  }

  type InfoRow = { label: string; value: string };

  $: rows = (info?.valid ? [
    { label: $t('inspect.contentId'), value: info.contentId },
    { label: $t('inspect.titleId'), value: info.titleId },
    { label: $t('inspect.region'), value: info.region },
    { label: $t('inspect.type'), value: info.contentType },
    { label: $t('inspect.drm'), value: info.drmType },
    { label: $t('inspect.fileSize'), value: formatSize(info.fileSize) },
  ] : []) as InfoRow[];
</script>

<div class="page">
  <div class="section">
    <div class="section-header">
      <h3>{$t('inspect.title')}</h3>
      {#if info}
        <button class="btn-ghost" on:click={clear}>{$t('split.clear')}</button>
      {/if}
    </div>

    {#if !info}
      <DropZone
        label={$t('inspect.drop')}
        subtitle={$t('inspect.dropSub')}
        icon="🔍"
        accept=".pkg,.pkgpart"
        on:files={handleDrop}
      />
      <div class="alt-action">
        <button class="btn-secondary" on:click={handleBrowse}>{$t('inspect.browse')}</button>
      </div>
    {:else if !info.valid}
      <div class="error-card">
        <span class="error-icon">⚠️</span>
        <span>{info.error}</span>
      </div>
    {:else}
      <div class="info-grid">
        {#each rows as row}
          <span class="info-label">{row.label}</span>
          <span class="info-value">{row.value}</span>
        {/each}
      </div>

      <div class="action-group">
        {#if suggestedName && suggestedName !== filePath.split('/').pop()}
          <button class="btn-secondary" on:click={handleRename} title="{$t('inspect.renameTo')} {suggestedName}">
            ✏️ {$t('inspect.renameTo')} {suggestedName}
          </button>
        {/if}

        {#if checksumRunning}
          <button class="btn-danger" on:click={() => CancelOperation()}>{$t('cancel')}</button>
        {:else if !checksum}
          <button class="btn-secondary" on:click={handleChecksum}>🔐 {$t('inspect.calcSha')}</button>
        {/if}
      </div>

      {#if checksumRunning}
        <ProgressBar
          percentage={checksumProgress}
          speedBPS={0}
          etaSeconds={0}
          bytesProcessed={0}
          totalBytes={0}
          label="SHA-256"
        />
      {/if}

      {#if checksum}
        <div class="checksum-result">
          <span class="checksum-label">SHA-256</span>
          <code class="checksum-hash">{checksum.sha256}</code>
          <span class="checksum-meta">{formatSize(checksum.size)} in {checksum.duration.toFixed(1)}s</span>
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: 20px;
    padding: 20px;
  }

  .section {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  h3 {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .alt-action {
    display: flex;
    justify-content: center;
    margin-top: 2px;
  }

  .info-grid {
    display: grid;
    grid-template-columns: 140px 1fr;
    gap: 6px 12px;
    padding: 14px;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
  }

  .info-label {
    font-size: 12px;
    color: var(--text-secondary);
    font-weight: 500;
  }

  .info-value {
    font-size: 13px;
    color: var(--text-primary);
    font-family: 'SF Mono', 'Fira Code', monospace;
    word-break: break-all;
  }

  .error-card {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 14px;
    background: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.2);
    border-radius: var(--radius-sm);
    color: var(--error);
    font-size: 13px;
  }

  .error-icon {
    font-size: 16px;
  }

  .action-group {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .checksum-result {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px 14px;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
  }

  .checksum-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    font-weight: 600;
    letter-spacing: 0.04em;
  }

  .checksum-hash {
    font-size: 12px;
    color: var(--success);
    font-family: 'SF Mono', 'Fira Code', monospace;
    word-break: break-all;
    background: none;
    padding: 0;
    user-select: text;
  }

  .checksum-meta {
    font-size: 11px;
    color: var(--text-muted);
  }
</style>
