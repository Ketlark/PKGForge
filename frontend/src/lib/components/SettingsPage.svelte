<script lang="ts">
  import { onMount } from 'svelte';
  import { t, locale } from '../stores/i18n';
  import type { Locale } from '../stores/i18n';
  import { addLog } from '../stores/activity';
  import { LoadConfig, SaveConfig, BufferLabels, ChunkLabels, SplitFormatLabels } from '../../../wailsjs/go/main/App';

  let bufferLabel = '64 MB';
  let chunkLabel = '4 GB';
  let splitFormat = '_NNN.pkgpart';
  let language: Locale = 'en';

  let bufferOptions: string[] = [];
  let chunkOptions: string[] = [];
  let formatOptions: string[] = [];
  let loaded = false;

  onMount(async () => {
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
    locale.set(language);
    loaded = true;
  });

  async function handleSave() {
    locale.set(language);
    await SaveConfig({
      defaultBufferLabel: bufferLabel,
      defaultChunkLabel: chunkLabel,
      defaultSplitFormat: splitFormat,
      defaultOutputDir: '',
      language,
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
</style>
