<script lang="ts">
  import { formatSize, formatTime, formatSpeed } from '../utils/format';

  export let percentage = 0;
  export let speedBPS = 0;
  export let etaSeconds = 0;
  export let bytesProcessed = 0;
  export let totalBytes = 0;
  export let label = '';
</script>

<div class="progress-wrapper">
  {#if label}
    <div class="progress-label">{label}</div>
  {/if}

  <div class="progress-track">
    <div class="progress-fill" style="width: {Math.min(percentage, 100)}%"></div>
  </div>

  <div class="progress-stats">
    <span>{percentage.toFixed(1)}%</span>
    <span>{formatSize(bytesProcessed)} / {formatSize(totalBytes)}</span>
    <span>{formatSpeed(speedBPS)}</span>
    <span>ETA {formatTime(etaSeconds)}</span>
  </div>
</div>

<style>
  .progress-wrapper {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .progress-label {
    font-size: 12px;
    color: var(--text-secondary);
    font-weight: 500;
  }

  .progress-track {
    height: 8px;
    background: var(--bg-input);
    border-radius: 4px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--accent), var(--accent-light));
    border-radius: 4px;
    transition: width 0.3s ease;
    box-shadow: 0 0 8px var(--accent-glow);
  }

  .progress-stats {
    display: flex;
    justify-content: space-between;
    font-size: 11px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
</style>
