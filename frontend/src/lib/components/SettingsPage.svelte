<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { onMount } from 'svelte';
  import { t, locale } from '../stores/i18n';
  import type { Locale } from '../stores/i18n';
  import { addLog } from '../stores/activity';
  import { LoadConfig, SaveConfig, BufferLabels, ChunkLabels, SplitFormatLabels, OpenEmulatorDirDialog } from '../../../wailsjs/go/main/App';

  const dispatch = createEventDispatcher<{ openAbout: void }>();

  let bufferLabel = '64 MB';
  let chunkLabel = '4 GB';
  let splitFormat = '_NNN.pkgpart';
  let language: Locale = 'en';
  let emulatorFilesDir = '';
  let checkUpdatesOnStartup = true;

  let bufferOptions: string[] = [];
  let chunkOptions: string[] = [];
  let formatOptions: string[] = [];
  let loaded = false;

  onMount(() => {
    void (async () => {
      const [bufs, chunks, fmts, cfg] = await Promise.all([
        BufferLabels(),
        ChunkLabels(),
        SplitFormatLabels(),
        LoadConfig(),
      ]);
      bufferOptions = bufs;
      chunkOptions = chunks;
      formatOptions = fmts;

      bufferLabel = cfg.defaultBufferLabel || '64 MB';
      chunkLabel = cfg.defaultChunkLabel || '4 GB';
      splitFormat = cfg.defaultSplitFormat || '_NNN.pkgpart';
      language = (cfg.language as Locale) || 'en';
      emulatorFilesDir = cfg.emulatorFilesDir || '';
      checkUpdatesOnStartup = cfg.checkUpdatesOnStartup ?? true;
      locale.set(language);
      loaded = true;
    })();
  });

  async function browseEmulatorDir() {
    const path = await OpenEmulatorDirDialog();
    if (path) emulatorFilesDir = path;
  }

  function clearEmulatorDir() {
    emulatorFilesDir = '';
  }

  async function handleSave() {
    locale.set(language);
    await SaveConfig({
      defaultBufferLabel: bufferLabel,
      defaultChunkLabel: chunkLabel,
      defaultSplitFormat: splitFormat,
      defaultOutputDir: '',
      language,
      emulatorFilesDir,
      checkUpdatesOnStartup,
    });
    addLog('success', $t('settings.saved'));
  }
</script>

<div class="page">
  {#if loaded}
    <div class="section">
      <h3>{$t('settings.title')}</h3>

      <div class="form-grid">
        <label class="form-label" for="settings-lang">{$t('settings.language')}</label>
        <select id="settings-lang" class="form-select" bind:value={language}>
          <option value="en">English</option>
          <option value="fr">Français</option>
        </select>
      </div>
    </div>

    <div class="section">
      <h3>{$t('settings.emulator')}</h3>

      {#if emulatorFilesDir}
        <p class="hint">{$t('settings.emulatorDirOverride')}</p>
        <div class="form-grid">
          <label class="form-label" for="settings-emudir">{$t('settings.emulatorDir')}</label>
          <div class="path-field">
            <input
              id="settings-emudir"
              type="text"
              class="form-select"
              bind:value={emulatorFilesDir}
              placeholder={$t('settings.emulatorDirPlaceholder')}
              readonly
            />
            <button class="btn-browse" on:click={browseEmulatorDir}>{$t('settings.browse')}</button>
          </div>
        </div>
      {:else}
        <p class="hint">{$t('settings.emulatorAutoDownload')}</p>
      {/if}

      <button class="btn-browse" on:click={clearEmulatorDir}>{$t('settings.emulatorReset')}</button>
    </div>

    <div class="section">
      <h3>{$t('settings.updates')}</h3>
      <label class="toggle-row">
        <input type="checkbox" bind:checked={checkUpdatesOnStartup} />
        <span>
          <strong>{$t('settings.checkUpdatesOnStartup')}</strong>
          <small>{$t('settings.checkUpdatesOnStartupHint')}</small>
        </span>
      </label>
    </div>

    <div class="section">
      <h3>{$t('settings.defaults')}</h3>

      <div class="form-grid">
        <label class="form-label" for="settings-buffer">{$t('settings.buffer')}</label>
        <select id="settings-buffer" class="form-select" bind:value={bufferLabel}>
          {#each bufferOptions as opt}
            <option value={opt}>{opt}</option>
          {/each}
        </select>

        <label class="form-label" for="settings-chunk">{$t('settings.chunk')}</label>
        <select id="settings-chunk" class="form-select" bind:value={chunkLabel}>
          {#each chunkOptions as opt}
            <option value={opt}>{opt}</option>
          {/each}
        </select>

        <label class="form-label" for="settings-format">{$t('settings.format')}</label>
        <select id="settings-format" class="form-select" bind:value={splitFormat}>
          {#each formatOptions as opt}
            <option value={opt}>{opt}</option>
          {/each}
        </select>
      </div>
    </div>

    <div class="section">
      <h3>{$t('settings.about')}</h3>
      <p class="hint">{$t('settings.aboutHint')}</p>
      <button class="btn-browse about-button" on:click={() => dispatch('openAbout')}>
        {$t('settings.aboutOpen')}
      </button>
    </div>

    <div class="actions">
      <button class="btn-primary" on:click={handleSave}>{$t('settings.save')}</button>
    </div>
  {/if}
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: 24px;
    padding: 20px;
  }

  .section {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  h3 {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .form-grid {
    display: grid;
    grid-template-columns: 140px 1fr;
    gap: 8px 12px;
    align-items: center;
  }

  .form-label {
    font-size: 13px;
    color: var(--text-secondary);
    white-space: nowrap;
  }

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

  .form-select:focus {
    border-color: var(--accent);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
  }

  .path-field {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .path-field input {
    flex: 1;
    min-width: 0;
    cursor: pointer;
  }

  .btn-browse {
    font-size: 12px;
    padding: 7px 14px;
    background: var(--bg-input);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text-primary);
    cursor: pointer;
    white-space: nowrap;
    transition: border-color 0.15s;
  }

  .btn-browse:hover {
    border-color: var(--accent);
  }

  .about-button {
    align-self: flex-start;
  }

  .hint {
    margin: 4px 0 0;
    font-size: 11px;
    color: var(--text-secondary);
    opacity: 0.7;
  }

  .toggle-row {
    display: flex;
    gap: 10px;
    align-items: flex-start;
    cursor: pointer;
    font-size: 13px;
    color: var(--text-primary);
  }

  .toggle-row input {
    margin-top: 3px;
    accent-color: var(--accent);
  }

  .toggle-row strong {
    display: block;
    margin-bottom: 3px;
    font-weight: 600;
  }

  .toggle-row small {
    display: block;
    color: var(--text-secondary);
    font-size: 11px;
    line-height: 1.4;
  }

</style>
