<script lang="ts">
  import DropZone from './DropZone.svelte';
  import FileList from './FileList.svelte';
  import ProgressBar from './ProgressBar.svelte';
  import { formatSize } from '../utils/format';
  import { t } from '../stores/i18n';
  import {
    mergeFiles,
    mergeOutputPath,
    mergeBufferLabel,
    mergeRunning,
    mergeProgress,
    mergeFileCount,
    mergeTotalSize,
    addFilesForMerge,
    removeFile,
    clearMergeFiles,
    startMerge,
    cancelMerge,
  } from '../stores/merge';
  import { OpenFilesDialog, BufferLabels, CheckDiskSpace } from '../../../wailsjs/go/main/App';

  let bufferOptions: string[] = [];
  let diskWarning = '';
  let diskCheckTimer: ReturnType<typeof setTimeout>;

  BufferLabels().then((labels) => { bufferOptions = labels; });

  function scheduleDiskCheck() {
    clearTimeout(diskCheckTimer);
    diskCheckTimer = setTimeout(async () => {
      if (!$mergeOutputPath || $mergeTotalSize === 0) { diskWarning = ''; return; }
      try {
        const sep = $mergeOutputPath.includes('\\') ? '\\' : '/';
        const dir = $mergeOutputPath.substring(0, $mergeOutputPath.lastIndexOf(sep)) || '/';
        const info = await CheckDiskSpace(dir);
        if (info.available > 0 && info.available < $mergeTotalSize) {
          diskWarning = `${$t('diskspace.warning')}: ${$t('diskspace.available')} ${formatSize(info.available)}`;
        } else {
          diskWarning = '';
        }
      } catch { diskWarning = ''; }
    }, 500);
  }

  $: if ($mergeOutputPath && $mergeTotalSize > 0) scheduleDiskCheck();

  async function handleBrowse() {
    const paths = await OpenFilesDialog();
    if (paths && paths.length > 0) await addFilesForMerge(paths);
  }

  async function handleDrop(e: CustomEvent<string[]>) {
    await addFilesForMerge(e.detail);
  }
</script>

<div class="page">
  <div class="section">
    <div class="section-header">
      <h3>{$t('merge.source')}</h3>
      {#if $mergeFileCount > 0}
        <button class="btn-ghost" on:click={clearMergeFiles}>{$t('merge.clearAll')}</button>
      {/if}
    </div>

    {#if $mergeFileCount === 0}
      <DropZone
        label={$t('merge.drop')}
        subtitle={$t('merge.dropSub')}
        icon="📦"
        multiple
        accept=".pkg,.pkgpart"
        disabled={$mergeRunning}
        on:files={handleDrop}
      />
      <div class="alt-action">
        <button class="btn-secondary" on:click={handleBrowse} disabled={$mergeRunning}>
          {$t('merge.browse')}
        </button>
      </div>
    {:else}
      <FileList files={$mergeFiles} on:remove={(e) => removeFile(e.detail)} />
      <div class="stats-row">
        <span class="stat">{$mergeFileCount} {$t('files')}</span>
        <span class="stat-sep">·</span>
        <span class="stat">{formatSize($mergeTotalSize)} {$t('total')}</span>
      </div>
    {/if}
  </div>

  <div class="section">
    <h3>{$t('merge.config')}</h3>
    <div class="form-grid">
      <label class="form-label" for="merge-output">{$t('merge.output')}</label>
      <input
        id="merge-output"
        class="form-input"
        type="text"
        bind:value={$mergeOutputPath}
        placeholder="Output file path"
        disabled={$mergeRunning}
      />

      <label class="form-label" for="merge-buffer">{$t('merge.buffer')}</label>
      <select id="merge-buffer" class="form-select" bind:value={$mergeBufferLabel} disabled={$mergeRunning}>
        {#each bufferOptions as opt}
          <option value={opt}>{opt}</option>
        {/each}
      </select>
    </div>

    {#if diskWarning}
      <div class="disk-warning">⚠️ {diskWarning}</div>
    {/if}
  </div>

  {#if $mergeProgress}
    <div class="section">
      <ProgressBar
        percentage={$mergeProgress.bytesProcessed / $mergeProgress.totalBytes * 100}
        speedBPS={$mergeProgress.speedBPS}
        etaSeconds={$mergeProgress.etaSeconds}
        bytesProcessed={$mergeProgress.bytesProcessed}
        totalBytes={$mergeProgress.totalBytes}
        label="{$t('merge.merging')}: {$mergeProgress.currentFileName} ({$mergeProgress.currentFileIndex + 1}/{$mergeProgress.totalFiles})"
      />
    </div>
  {/if}

  <div class="actions">
    {#if $mergeRunning}
      <button class="btn-danger" on:click={cancelMerge}>{$t('cancel')}</button>
    {:else}
      <button
        class="btn-primary"
        on:click={startMerge}
        disabled={$mergeFileCount < 2 || !$mergeOutputPath}
      >
        {$t('merge.btn')} {$mergeFileCount} {$t('files')}
      </button>
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

  .stats-row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 0;
  }

  .stat {
    font-size: 12px;
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }

  .stat-sep {
    color: var(--text-muted);
    font-size: 12px;
  }

  .form-grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 8px 12px;
    align-items: center;
  }

  .form-label {
    font-size: 13px;
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .form-input,
  .form-select {
    font-size: 13px;
    padding: 7px 10px;
    background: var(--bg-input);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text-primary);
    outline: none;
    transition: border-color 0.15s;
    font-family: inherit;
  }

  .form-input:focus,
  .form-select:focus {
    border-color: var(--accent);
  }

  .form-input:disabled,
  .form-select:disabled {
    opacity: 0.5;
  }

  .disk-warning {
    padding: 8px 12px;
    font-size: 12px;
    color: var(--warning);
    background: rgba(245, 158, 11, 0.08);
    border: 1px solid rgba(245, 158, 11, 0.2);
    border-radius: var(--radius-sm);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding-top: 4px;
  }
</style>
