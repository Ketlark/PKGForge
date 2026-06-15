<script lang="ts">
  import DropZone from './DropZone.svelte';
  import ProgressBar from './ProgressBar.svelte';
  import { t } from '../stores/i18n';
  import {
    ps1CuePath,
    ps1Title,
    ps1TitleID,
    ps1Icon0,
    ps1Pic1,
    ps1Emulator,
    ps1AnalogSticks,
    ps1SkipBootLogo,
    ps1Force60Hz,
    ps1EnableCDDATOC,
    ps1DetectedGameID,
    ps1DetectedTitle,
    ps1DetectedRegion,
    ps1DetectedTrackNum,
    ps1DetectedHasCDDA,
    ps1DetectedIsMultiBin,
    ps1DetectedCoverPath,
    ps1OutputPath,
    ps1ExtraDiscs,
    ps1Running,
    ps1Progress,
    ps1Error,
    ps1CanCreate,
    browseCUE,
    browseIcon,
    browsePic1,
    browseOutput,
    addExtraDisc,
    removeExtraDisc,
    startPS1FPKG,
    clearPS1DiscSelection,
    pickPS1DiscPath,
    selectPS1Disc,
  } from '../stores/ps1';

  function handleDropCUE(e: CustomEvent<string[]>) {
    const path = pickPS1DiscPath(e.detail);
    if (path) void selectPS1Disc(path);
  }

  function basename(path: string) {
    return path.split('/').pop() || path.split('\\').pop() || path;
  }
</script>

<div class="page">
  <!-- Source disc -->
  <div class="section">
    <div class="section-header">
      <h3>{$t('ps1.source')}</h3>
      {#if $ps1DetectedGameID}
        <span class="badge region-{$ps1DetectedRegion}">{$ps1DetectedRegion}</span>
      {/if}
    </div>

    {#if !$ps1CuePath}
      <DropZone
        label={$t('ps1.dropCue')}
        subtitle={$t('ps1.dropCueSub')}
        icon="💿"
        accept=".cue,.bin"
        disabled={$ps1Running}
        on:files={handleDropCUE}
      />
      <p class="source-note">{$t('ps1.cueBinHelp')}</p>
      <div class="alt-action">
        <button class="btn-secondary" on:click={browseCUE} disabled={$ps1Running}>
          {$t('ps1.browseCue')}
        </button>
      </div>
    {:else}
      <div class="file-info">
        <span class="disc-label primary">{$t('ps1.disc1')}</span>
        <span class="file-icon">💿</span>
        <span class="file-name">{basename($ps1CuePath)}</span>
        {#if $ps1DetectedGameID}
          <span class="file-detail">{$ps1DetectedGameID}</span>
        {/if}
        {#if $ps1DetectedTrackNum > 0}
          <span class="file-detail">{$ps1DetectedTrackNum} tracks</span>
        {/if}
        {#if $ps1DetectedIsMultiBin}
          <span class="file-detail warning">multi-bin</span>
        {/if}
        {#if $ps1DetectedHasCDDA}
          <span class="file-detail cdda">CDDA</span>
        {/if}
        {#if $ps1DetectedCoverPath}
          <span class="file-detail cover">{$t('ps1.coverFound')}</span>
        {/if}
        <button class="btn-ghost-sm" on:click={clearPS1DiscSelection} disabled={$ps1Running}>
          {$t('ps1.changeDisc')}
        </button>
      </div>
      <p class="source-note">{$t('ps1.extraDiscHelp')}</p>

      <!-- Extra discs -->
      {#if $ps1ExtraDiscs.length > 0}
        <div class="extra-discs">
          {#each $ps1ExtraDiscs as disc, i}
            <div class="disc-item">
              <span class="disc-label">Disc {i + 2}</span>
              <span class="disc-path">{basename(disc)}</span>
              <button class="btn-ghost-sm" on:click={() => removeExtraDisc(i)}>✕</button>
            </div>
          {/each}
        </div>
      {/if}
      {#if $ps1ExtraDiscs.length < 3}
        <button class="btn-ghost" on:click={addExtraDisc} disabled={$ps1Running}>
          + {$t('ps1.addDisc')}
        </button>
      {/if}
    {/if}
  </div>

  <!-- Game info -->
  {#if $ps1CuePath}
    <div class="section">
      <h3>{$t('ps1.gameInfo')}</h3>
      <div class="form-grid">
        <label class="form-label" for="ps1-title">{$t('ps1.gameTitle')}</label>
        <input id="ps1-title" class="form-input" type="text" bind:value={$ps1Title} disabled={$ps1Running} />

        <label class="form-label" for="ps1-titleid">{$t('ps1.titleID')}</label>
        <input id="ps1-titleid" class="form-input" type="text" bind:value={$ps1TitleID} disabled={$ps1Running} />

        <label class="form-label" for="ps1-icon">{$t('ps1.icon')}</label>
        <div class="input-with-btn">
          <input id="ps1-icon" class="form-input" type="text" bind:value={$ps1Icon0} placeholder="512×512 PNG" disabled={$ps1Running} />
          <button class="btn-sm" on:click={browseIcon} disabled={$ps1Running}>…</button>
        </div>

        <label class="form-label" for="ps1-background">{$t('ps1.background')}</label>
        <div class="input-with-btn">
          <input id="ps1-background" class="form-input" type="text" bind:value={$ps1Pic1} placeholder="1920×1080 PNG" disabled={$ps1Running} />
          <button class="btn-sm" on:click={browsePic1} disabled={$ps1Running}>…</button>
        </div>
      </div>
    </div>

    <!-- Emulator options -->
    <div class="section">
      <h3>{$t('ps1.emulator')}</h3>
      <div class="form-grid">
        <label class="form-label" for="ps1-emu">{$t('ps1.emulatorType')}</label>
        <select id="ps1-emu" class="form-select" bind:value={$ps1Emulator} disabled={$ps1Running}>
          <option value="ps1_emu">PS1 Emu (default)</option>
          <option value="ps1_netemu">PS1 Net Emu (PS Plus)</option>
        </select>
      </div>

      <div class="checkbox-grid">
        <label class="checkbox-label">
          <input type="checkbox" bind:checked={$ps1AnalogSticks} disabled={$ps1Running} />
          {$t('ps1.analogSticks')}
        </label>
        <label class="checkbox-label">
          <input type="checkbox" bind:checked={$ps1SkipBootLogo} disabled={$ps1Running} />
          {$t('ps1.skipBootLogo')}
        </label>
        <label class="checkbox-label">
          <input type="checkbox" bind:checked={$ps1Force60Hz} disabled={$ps1Running} />
          {$t('ps1.force60hz')}
        </label>
        {#if $ps1DetectedHasCDDA}
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={$ps1EnableCDDATOC} disabled={$ps1Running} />
            {$t('ps1.cddaToc')}
          </label>
        {/if}
      </div>
    </div>

    <!-- Output -->
    <div class="section">
      <h3>{$t('ps1.output')}</h3>
      <div class="form-grid">
        <label class="form-label" for="ps1-output">{$t('ps1.outputPath')}</label>
        <div class="input-with-btn">
          <input id="ps1-output" class="form-input" type="text" bind:value={$ps1OutputPath} disabled={$ps1Running} />
          <button class="btn-sm" on:click={browseOutput} disabled={$ps1Running}>…</button>
        </div>
      </div>
    </div>

    <!-- Error -->
    {#if $ps1Error}
      <div class="error-box">{$ps1Error}</div>
    {/if}

    <!-- Progress -->
    {#if $ps1Running}
      <div class="section">
        <ProgressBar
          percentage={$ps1Progress.percentage}
          speedBPS={$ps1Progress.speedBPS}
          etaSeconds={$ps1Progress.etaSeconds}
          bytesProcessed={$ps1Progress.bytesProcessed}
          totalBytes={$ps1Progress.totalBytes}
          label={$ps1Progress.phase || $t('ps1.creating')}
        />
      </div>
    {/if}

    <!-- Actions -->
    <div class="actions">
      {#if $ps1Running}
        <button class="btn-primary" disabled>{$t('ps1.creating')}…</button>
      {:else}
        <button
          class="btn-primary"
          on:click={startPS1FPKG}
          disabled={!$ps1CanCreate}
        >
          {$t('ps1.create')}
        </button>
      {/if}
    </div>
  {/if}
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

  .badge {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    padding: 2px 8px;
    border-radius: 10px;
    color: #fff;
  }

  .badge.region-america { background: #3b82f6; }
  .badge.region-europe { background: #22c55e; }
  .badge.region-japan { background: #ef4444; }
  .badge.region-asia { background: #a855f7; }

  .alt-action {
    display: flex;
    justify-content: center;
    margin-top: 2px;
  }

  .file-info {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
  }

  .source-note {
    margin: -2px 0 0;
    font-size: 12px;
    line-height: 1.4;
    color: var(--text-muted);
  }

  .file-icon {
    font-size: 18px;
  }

  .file-name {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-primary);
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-detail {
    font-size: 11px;
    color: var(--text-muted);
    padding: 2px 6px;
    background: var(--bg-elevated);
    border-radius: 4px;
  }

  .file-detail.warning {
    color: #f59e0b;
    background: rgba(245, 158, 11, 0.1);
  }

  .file-detail.cdda {
    color: #8b5cf6;
    background: rgba(139, 92, 246, 0.1);
  }

  .file-detail.cover {
    color: #16a34a;
    background: rgba(22, 163, 74, 0.1);
  }

  .extra-discs {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .disc-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 12px;
  }

  .disc-label {
    font-weight: 600;
    color: var(--text-secondary);
    min-width: 50px;
  }

  .disc-label.primary {
    min-width: auto;
    padding: 2px 6px;
    border-radius: 4px;
    color: var(--accent);
    background: var(--accent-soft);
  }

  .disc-path {
    flex: 1;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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

  .input-with-btn {
    display: flex;
    gap: 4px;
  }

  .input-with-btn .form-input {
    flex: 1;
  }

  .btn-sm {
    font-size: 12px;
    padding: 6px 10px;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text-secondary);
    cursor: pointer;
    font-family: inherit;
  }

  .btn-sm:hover {
    background: var(--accent-soft);
    color: var(--accent);
  }

  .btn-ghost {
    font-size: 12px;
    padding: 4px 8px;
    background: none;
    border: 1px dashed var(--border);
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    cursor: pointer;
    font-family: inherit;
  }

  .btn-ghost:hover {
    border-color: var(--accent);
    color: var(--accent);
  }

  .btn-ghost-sm {
    font-size: 11px;
    padding: 2px 6px;
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    font-family: inherit;
  }

  .btn-ghost-sm:hover:not(:disabled) {
    color: var(--accent);
  }

  .btn-ghost-sm:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .checkbox-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 12px 20px;
    padding-top: 4px;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--text-secondary);
    cursor: pointer;
  }

  .checkbox-label input[type="checkbox"] {
    accent-color: var(--accent);
  }

  .error-box {
    padding: 8px 12px;
    font-size: 12px;
    color: var(--danger, #ef4444);
    background: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.2);
    border-radius: var(--radius-sm);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding-top: 4px;
  }
</style>
