<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let label = 'Drop files here';
  export let subtitle = 'or click to browse';
  export let icon = '📁';
  export let accept = '';
  export let multiple = false;
  export let disabled = false;

  const dispatch = createEventDispatcher<{ files: string[] }>();

  let dragging = false;
  let fileInput: HTMLInputElement;

  function handleDragOver(e: DragEvent) {
    if (disabled) return;
    e.preventDefault();
    dragging = true;
  }

  function handleDragLeave() {
    dragging = false;
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    dragging = false;
    if (disabled || !e.dataTransfer) return;

    const paths: string[] = [];
    for (let i = 0; i < e.dataTransfer.files.length; i++) {
      const f = e.dataTransfer.files[i];
      if ('path' in f && typeof (f as any).path === 'string') {
        paths.push((f as any).path);
      }
    }
    if (paths.length > 0) {
      dispatch('files', paths);
    }
  }

  function handleClick() {
    if (disabled) return;
    fileInput?.click();
  }

  function handleFileInput(e: Event) {
    const input = e.target as HTMLInputElement;
    if (!input.files) return;
    const paths: string[] = [];
    for (let i = 0; i < input.files.length; i++) {
      const f = input.files[i];
      if ('path' in f && typeof (f as any).path === 'string') {
        paths.push((f as any).path);
      }
    }
    if (paths.length > 0) {
      dispatch('files', paths);
    }
    input.value = '';
  }
</script>

<!-- svelte-ignore a11y-no-static-element-interactions -->
<div
  class="drop-zone"
  class:dragging
  class:disabled
  on:dragover={handleDragOver}
  on:dragleave={handleDragLeave}
  on:drop={handleDrop}
  on:click={handleClick}
  on:keydown={(e) => e.key === 'Enter' && handleClick()}
  role="button"
  tabindex="0"
>
  <input
    bind:this={fileInput}
    type="file"
    {accept}
    {multiple}
    on:change={handleFileInput}
    class="hidden-input"
  />
  <span class="drop-icon">{icon}</span>
  <span class="drop-label">{label}</span>
  <span class="drop-subtitle">{subtitle}</span>
</div>

<style>
  .drop-zone {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 28px 20px;
    border: 2px dashed var(--border);
    border-radius: var(--radius-lg);
    background: var(--bg-surface);
    cursor: pointer;
    transition: all 0.2s ease;
    user-select: none;
  }

  .drop-zone:hover:not(.disabled) {
    border-color: var(--accent);
    background: var(--accent-soft);
  }

  .drop-zone.dragging {
    border-color: var(--accent);
    background: var(--accent-soft);
    box-shadow: 0 0 20px var(--accent-glow);
  }

  .drop-zone.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .drop-icon {
    font-size: 28px;
    line-height: 1;
  }

  .drop-label {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .drop-subtitle {
    font-size: 12px;
    color: var(--text-muted);
  }

  .hidden-input {
    display: none;
  }
</style>
