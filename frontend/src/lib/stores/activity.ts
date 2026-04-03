import { writable, derived } from 'svelte/store';
import type { LogEntry } from '../types';

let nextId = 1;

export const logs = writable<LogEntry[]>([]);

export const logCount = derived(logs, ($l) => $l.length);

export function addLog(type: LogEntry['type'], message: string) {
  logs.update((entries) => [
    { id: nextId++, timestamp: new Date(), type, message },
    ...entries,
  ]);
}

export function clearLogs() {
  logs.set([]);
}

export function exportLogs(entries: LogEntry[]): string {
  return entries
    .map((e) => `[${e.timestamp.toISOString()}] [${e.type.toUpperCase()}] ${e.message}`)
    .join('\n');
}
