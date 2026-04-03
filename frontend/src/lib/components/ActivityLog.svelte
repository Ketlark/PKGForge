<script lang="ts">
  import { logs, logCount, clearLogs, exportLogs } from '../stores/activity';
  import { t } from '../stores/i18n';

  function handleExport() {
    const content = exportLogs($logs);
    const blob = new Blob([content], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `pkg-forge-log-${new Date().toISOString().slice(0, 10)}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  }

  function formatTimestamp(d: Date): string {
    return d.toLocaleTimeString('en-GB', { hour12: false });
  }
</script>

<div class="page">
  <div class="section-header">
    <h3>{$t('activity.title')}</h3>
    <div class="header-actions">
      {#if $logCount > 0}
        <button class="btn-ghost" on:click={handleExport}>{$t('activity.export')}</button>
        <button class="btn-ghost" on:click={clearLogs}>{$t('activity.clear')}</button>
      {/if}
    </div>
  </div>

  <div class="log-container">
    {#if $logCount === 0}
      <div class="log-empty">{$t('activity.empty')}</div>
    {:else}
      {#each $logs as entry (entry.id)}
        <div class="log-row log-{entry.type}">
          <span class="log-time">{formatTimestamp(entry.timestamp)}</span>
          <span class="log-badge">{entry.type}</span>
          <span class="log-msg">{entry.message}</span>
        </div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 20px;
    height: 100%;
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

  .header-actions {
    display: flex;
    gap: 6px;
  }

  .log-container {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .log-empty {
    padding: 40px;
    text-align: center;
    color: var(--text-muted);
    font-size: 13px;
  }

  .log-row {
    display: flex;
    align-items: baseline;
    gap: 8px;
    padding: 5px 8px;
    border-radius: var(--radius-sm);
    font-size: 12px;
    transition: background 0.1s;
  }

  .log-row:hover {
    background: var(--bg-elevated);
  }

  .log-time {
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
    font-size: 11px;
    flex-shrink: 0;
  }

  .log-badge {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 1px 5px;
    border-radius: 3px;
    flex-shrink: 0;
  }

  .log-info .log-badge {
    color: var(--accent);
    background: var(--accent-soft);
  }

  .log-success .log-badge {
    color: var(--success);
    background: rgba(45, 212, 160, 0.12);
  }

  .log-error .log-badge {
    color: var(--error);
    background: rgba(239, 68, 68, 0.12);
  }

  .log-warning .log-badge {
    color: var(--warning);
    background: rgba(245, 158, 11, 0.12);
  }

  .log-msg {
    color: var(--text-primary);
    flex: 1;
    word-break: break-word;
  }
</style>
