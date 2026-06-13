<script lang="ts">
  import DropZone from './DropZone.svelte';
  import ProgressBar from './ProgressBar.svelte';
  import { t } from '../stores/i18n';
  import {
    ps2ISOPaths,
    ps2Title,
    ps2TitleID,
    ps2Icon0,
    ps2Pic1,
    ps2Emulator,
    ps2ConfigTXT,
    ps2ConfigLUA,
    ps2Uprender,
    ps2DisplayMode,
    ps2OutputPath,
    ps2Running,
    ps2Progress,
    ps2Error,
    ps2CanCreate,
    ps2DiscCount,
    ps2DetectedGameID,
    ps2DetectedRegion,
    browseISOs,
    addExtraDisc,
    removeISO,
    browseIcon,
    browsePic1,
    browseOutput,
    startPS2FPKG,
  } from '../stores/ps2';

  function handleDropISO(e: CustomEvent<string[]>) {
    if (e.detail.length > 0) {
      ps2ISOPaths.set(e.detail);
    }
  }
</script>

<div class="page">
  <!-- Source ISOs -->
  <div class="section">
    <div class="section-header">
      <h3>{$t('ps2.source')}</h3>
      {#if $ps2DetectedGameID}
        <span class="badge region-{$ps2DetectedRegion}">{$ps2DetectedRegion}</span>
      {/if}
    </div>

    {#if $ps2ISOPaths.length === 0}
      <DropZone
        label={$t('ps2.dropIso')}
        subtitle={$t('ps2.dropIsoSub')}
        icon="📀"
        multiple
        accept=".iso,.bin,.cue"
        disabled={$ps2Running}
        on:files={handleDropISO}
      />
      <div class="alt-action">
        <button class="btn-secondary" on:click={browseISOs} disabled={$ps2Running}>
          {$t('ps2.browseIso')}
        </button>
      </div>
    {:else}
      <div class="iso-list">
        {#each $ps2ISOPaths as iso, i}
          <div class="iso-item">
            <span class="iso-label">Disc {i + 1}</span>
            <span class="iso-icon">📀</span>
            <span class="iso-path">{iso.split('/').pop() || iso.split('\\').pop()}</span>
            {#if i > 0}
              <button class="btn-ghost-sm" on:click={() => removeISO(i)}>✕</button>
            {/if}
          </div>
        {/each}
      </div>

      {#if $ps2DiscCount < 5}
        <button class="btn-ghost" on:click={addExtraDisc} disabled={$ps2Running}>
          + {$t('ps2.addDisc')}
        </button>
      {/if}

      <div class="stats-row">
        <span class="stat">{$ps2DiscCount} {$ps2DiscCount > 1 ? 'discs' : 'disc'}</span>
      </div>
    {/if}
  </div>

  {#if $ps2ISOPaths.length > 0}
    <!-- Game info -->
    <div class="section">
      <h3>{$t('ps2.gameInfo')}</h3>
      <div class="form-grid">
        <label class="form-label" for="ps2-title">{$t('ps2.gameTitle')}</label>
        <input id="ps2-title" class="form-input" type="text" bind:value={$ps2Title} disabled={$ps2Running} />

        <label class="form-label" for="ps2-titleid">{$t('ps2.titleID')}</label>
        <input id="ps2-titleid" class="form-input" type="text" bind:value={$ps2TitleID} disabled={$ps2Running} />

        <label class="form-label">{$t('ps2.icon')}</label>
        <div class="input-with-btn">
          <input class="form-input" type="text" bind:value={$ps2Icon0} placeholder="512×512 PNG" disabled={$ps2Running} />
          <button class="btn-sm" on:click={browseIcon} disabled={$ps2Running}>…</button>
        </div>

        <label class="form-label">{$t('ps2.background')}</label>
        <div class="input-with-btn">
          <input class="form-input" type="text" bind:value={$ps2Pic1} placeholder="1920×1080 PNG" disabled={$ps2Running} />
          <button class="btn-sm" on:click={browsePic1} disabled={$ps2Running}>…</button>
        </div>
      </div>
    </div>

    <!-- Emulator options -->
    <div class="section">
      <h3>{$t('ps2.emulator')}</h3>
      <div class="form-grid">
        <label class="form-label" for="ps2-emu">{$t('ps2.emulatorType')}</label>
        <select id="ps2-emu" class="form-select" bind:value={$ps2Emulator} disabled={$ps2Running}>
          <option value="jakv2">Jak v2 (recommended)</option>
          <option value="rogue">Rogue Galaxy</option>
          <option value="codeveronica">Code: Veronica</option>
        </select>

        <label class="form-label" for="ps2-uprender">{$t('ps2.uprender')}</label>
        <select id="ps2-uprender" class="form-select" bind:value={$ps2Uprender} disabled={$ps2Running}>
          <option value="off">{$t('ps2.uprenderOff')}</option>
          <option value="2x2">2×2</option>
          <option value="4x">4×</option>
        </select>

        <label class="form-label" for="ps2-display">{$t('ps2.displayMode')}</label>
        <select id="ps2-display" class="form-select" bind:value={$ps2DisplayMode} disabled={$ps2Running}>
          <option value="4:3">4:3</option>
          <option value="16:9">16:9</option>
          <option value="auto">Auto</option>
        </select>
      </div>
    </div>

    <!-- Advanced config -->
    <details class="section collapsible">
      <summary class="collapsible-header">{$t('ps2.advanced')}</summary>
      <div class="collapsible-body">
        <div class="form-grid">
          <label class="form-label" for="ps2-config-txt">config-emu-ps4.txt</label>
          <textarea
            id="ps2-config-txt"
            class="form-textarea"
            bind:value={$ps2ConfigTXT}
            placeholder="--uprender=2x2&#10;--display-mode=4:3"
            disabled={$ps2Running}
            rows="3"
          ></textarea>

          <label class="form-label" for="ps2-config-lua">LUA patch</label>
          <textarea
            id="ps2-config-lua"
            class="form-textarea"
            bind:value={$ps2ConfigLUA}
            placeholder="-- Lua script"
            disabled={$ps2Running}
            rows="3"
          ></textarea>
        </div>
      </div>
    </details>

    <!-- Output -->
    <div class="section">
      <h3>{$t('ps2.output')}</h3>
      <div class="form-grid">
        <label class="form-label" for="ps2-output">{$t('ps2.outputPath')}</label>
        <div class="input-with-btn">
          <input id="ps2-output" class="form-input" type="text" bind:value={$ps2OutputPath} disabled={$ps2Running} />
          <button class="btn-sm" on:click={browseOutput} disabled={$ps2Running}>…</button>
        </div>
      </div>
    </div>

    <!-- Error -->
    {#if $ps2Error}
      <div class="error-box">{$ps2Error}</div>
    {/if}

    <!-- Progress -->
    {#if $ps2Running}
      <div class="section">
        <ProgressBar
          percentage={$ps2Progress * 100}
          label={$t('ps2.creating')}
        />
      </div>
    {/if}

    <!-- Actions -->
    <div class="actions">
      {#if $ps2Running}
        <button class="btn-primary" disabled>{$t('ps2.creating')}…</button>
      {:else}
        <button
          class="btn-primary"
          on:click={startPS2FPKG}
          disabled={!$ps2CanCreate}
        >
          {$t('ps2.create')}
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

  .iso-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .iso-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
  }

  .iso-label {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-muted);
    min-width: 50px;
  }

  .iso-icon {
    font-size: 16px;
  }

  .iso-path {
    flex: 1;
    font-size: 13px;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
  }

  .form-grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 8px 12px;
    align-items: start;
  }

  .form-label {
    font-size: 13px;
    color: var(--text-secondary);
    white-space: nowrap;
    padding-top: 7px;
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

  .form-textarea {
    font-size: 12px;
    font-family: 'SF Mono', 'Fira Code', monospace;
    padding: 7px 10px;
    background: var(--bg-input);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text-primary);
    outline: none;
    resize: vertical;
    min-height: 60px;
  }

  .form-textarea:focus,
  .form-input:focus,
  .form-select:focus {
    border-color: var(--accent);
  }

  .form-input:disabled,
  .form-select:disabled,
  .form-textarea:disabled {
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

  .btn-ghost-sm:hover {
    color: var(--accent);
  }

  .collapsible {
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }

  .collapsible-header {
    padding: 8px 12px;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-secondary);
    background: var(--bg-surface);
    cursor: pointer;
    list-style: none;
  }

  .collapsible-header::-webkit-details-marker {
    display: none;
  }

  .collapsible-header::before {
    content: '▸ ';
    transition: transform 0.15s;
  }

  .collapsible[open] .collapsible-header::before {
    content: '▾ ';
  }

  .collapsible-body {
    padding: 12px;
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
