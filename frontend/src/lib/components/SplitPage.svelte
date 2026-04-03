<script lang="ts">
  import DropZone from './DropZone.svelte';
  import ProgressBar from './ProgressBar.svelte';
  import { formatSize } from '../utils/format';
  import { t } from '../stores/i18n';
  import {
    splitSource,
    splitOutputDir,
    splitChunkLabel,
    splitFormatLabel,
    splitBufferLabel,
    splitRunning,
    splitProgress,
    setSourceFile,
    clearSplitSource,
    startSplit,
    cancelSplit,
  } from '../stores/split';
  import {
    OpenFileDialog,
    OpenDirectoryDialog,
    BufferLabels,
    ChunkLabels,
    SplitFormatLabels,
    CheckDiskSpace,
  } from '../../../wailsjs/go/main/App';

  let bufferOptions: string[] = [];
  let chunkOptions: string[] = [];
  let formatOptions: string[] = [];
  let diskWarning = '';
  let diskCheckTimer: ReturnType<typeof setTimeout>;

  Promise.all([BufferLabels(), ChunkLabels(), SplitFormatLabels()]).then(
    ([bufs, chunks, fmts]) => {
      bufferOptions = bufs;
      chunkOptions = chunks;
      formatOptions = fmts;
    }
  );

  function scheduleDiskCheck() {
    clearTimeout(diskCheckTimer);
    diskCheckTimer = setTimeout(async () => {
      if (!$splitOutputDir || !$splitSource) { diskWarning = ''; return; }
      try {
        const info = await CheckDiskSpace($splitOutputDir);
        if (info.available > 0 && info.available < $splitSource.size) {
          diskWarning = `${$t('diskspace.warning')}: ${$t('diskspace.available')} ${formatSize(info.available)}`;
        } else {
          diskWarning = '';
        }
      } catch { diskWarning = ''; }
    }, 500);
  }

  $: if ($splitOutputDir && $splitSource) scheduleDiskCheck();

  async function handleBrowse() {
    const path = await OpenFileDialog();
    if (path) await setSourceFile(path);
  }

  async function handleDrop(e: CustomEvent<string[]>) {
    if (e.detail.length > 0) await setSourceFile(e.detail[0]);
  }

  async function handleBrowseOutputDir() {
    const dir = await OpenDirectoryDialog();
    if (dir) splitOutputDir.set(dir);
  }
</script>

<div class="page">
  <div class="section">
    <div class="section-header">
      <h3>{$t('split.source')}</h3>
      {#if $splitSource}
        <button class="btn-ghost" on:click={clearSplitSource}>{$t('split.clear')}</button>
      {/if}
    </div>

    {#if !$splitSource}
      <DropZone
        label={$t('split.drop')}
        subtitle={$t('split.dropSub')}
        icon="✂️"
        accept=".pkg"
        disabled={$splitRunning}
        on:files={handleDrop}
      />
      <div class="alt-action">
        <button class="btn-secondary" on:click={handleBrowse} disabled={$splitRunning}>
          {$t('split.browse')}
        </button>
      </div>
    {:else}
      <div class="source-info">
        <span class="source-name">{$splitSource.name}</span>
        <span class="source-size">{formatSize($splitSource.size)}</span>
      </div>
    {/if}
  </div>

  <div class="section">
    <h3>{$t('split.config')}</h3>
    <div class="form-grid">
      <label class="form-label" for="split-chunk">{$t('split.chunk')}</label>
      <select id="split-chunk" class="form-select" bind:value={$splitChunkLabel} disabled={$splitRunning}>
        {#each chunkOptions as opt}
          <option value={opt}>{opt}</option>
        {/each}
      </select>

      <label class="form-label" for="split-format">{$t('split.format')}</label>
      <select id="split-format" class="form-select" bind:value={$splitFormatLabel} disabled={$splitRunning}>
        {#each formatOptions as opt}
          <option value={opt}>{opt}</option>
        {/each}
      </select>

      <label class="form-label" for="split-buffer">{$t('split.buffer')}</label>
      <select id="split-buffer" class="form-select" bind:value={$splitBufferLabel} disabled={$splitRunning}>
        {#each bufferOptions as opt}
          <option value={opt}>{opt}</option>
        {/each}
      </select>

      <label class="form-label" for="split-outdir">{$t('split.outdir')}</label>
      <div class="input-with-btn">
        <input
          id="split-outdir"
          class="form-input"
          type="text"
          bind:value={$splitOutputDir}
          placeholder="Output directory"
          disabled={$splitRunning}
        />
        <button class="btn-icon" on:click={handleBrowseOutputDir} disabled={$splitRunning} title="Browse">
          📂
        </button>
      </div>
    </div>

    {#if diskWarning}
      <div class="disk-warning">⚠️ {diskWarning}</div>
    {/if}
  </div>

  {#if $splitProgress}
    <div class="section">
      <ProgressBar
        percentage={$splitProgress.bytesWritten / $splitProgress.totalBytes * 100}
        speedBPS={$splitProgress.speedBPS}
        etaSeconds={$splitProgress.etaSeconds}
        bytesProcessed={$splitProgress.bytesWritten}
        totalBytes={$splitProgress.totalBytes}
        label="{$t('split.splitting')}: part {$splitProgress.currentPart + 1}/{$splitProgress.totalParts}"
      />
    </div>
  {/if}

  <div class="actions">
    {#if $splitRunning}
      <button class="btn-danger" on:click={cancelSplit}>{$t('cancel')}</button>
    {:else}
      <button
        class="btn-primary"
        on:click={startSplit}
        disabled={!$splitSource || !$splitOutputDir}
      >
        {$t('split.btn')}
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

  .source-info {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 14px;
    background: var(--bg-input);
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
  }

  .source-name {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary);
  }

  .source-size {
    font-size: 13px;
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
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
    width: 100%;
    box-sizing: border-box;
  }

  .form-input:focus,
  .form-select:focus {
    border-color: var(--accent);
  }

  .form-input:disabled,
  .form-select:disabled {
    opacity: 0.5;
  }

  .input-with-btn {
    display: flex;
    gap: 6px;
  }

  .input-with-btn .form-input {
    flex: 1;
  }

  .btn-icon {
    background: var(--bg-input);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 6px 10px;
    cursor: pointer;
    font-size: 14px;
    transition: all 0.15s;
    line-height: 1;
  }

  .btn-icon:hover:not(:disabled) {
    border-color: var(--accent);
    background: var(--accent-soft);
  }

  .btn-icon:disabled {
    opacity: 0.5;
    cursor: not-allowed;
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
