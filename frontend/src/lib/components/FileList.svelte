<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { formatSize } from '../utils/format';
  import type { FileEntry } from '../types';

  export let files: FileEntry[] = [];
  export let removable = true;

  const dispatch = createEventDispatcher<{ remove: string }>();
</script>

{#if files.length > 0}
  <div class="file-list">
    {#each files as file, i (file.path)}
      <div class="file-row">
        <span class="file-index">{i + 1}</span>
        <span class="file-name" title={file.path}>{file.name}</span>
        <span class="file-size">{formatSize(file.size)}</span>
        {#if removable}
          <button class="file-remove" on:click={() => dispatch('remove', file.path)} title="Remove">
            ✕
          </button>
        {/if}
      </div>
    {/each}
  </div>
{:else}
  <div class="file-list-empty">No files added</div>
{/if}

<style>
  .file-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-height: 200px;
    overflow-y: auto;
  }

  .file-row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 10px;
    background: var(--bg-input);
    border-radius: var(--radius-sm);
    font-size: 13px;
    transition: background 0.15s;
  }

  .file-row:hover {
    background: var(--bg-elevated);
  }

  .file-index {
    color: var(--text-muted);
    font-size: 11px;
    min-width: 18px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .file-name {
    flex: 1;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-size {
    color: var(--text-secondary);
    font-size: 12px;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .file-remove {
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    font-size: 12px;
    padding: 2px 4px;
    border-radius: 3px;
    transition: all 0.15s;
    line-height: 1;
  }

  .file-remove:hover {
    color: var(--error);
    background: rgba(239, 68, 68, 0.15);
  }

  .file-list-empty {
    padding: 16px;
    text-align: center;
    color: var(--text-muted);
    font-size: 13px;
  }
</style>
