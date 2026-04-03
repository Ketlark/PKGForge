import { writable, derived, get } from 'svelte/store';
import { DetectParts, SuggestOutputPath, MergeFiles, CancelOperation, GetFileInfo } from '../../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime';
import { addLog } from './activity';
import type { FileEntry, MergeProgress } from '../types';

export const mergeFiles = writable<FileEntry[]>([]);
export const mergeOutputPath = writable('');
export const mergeBufferLabel = writable('64 MB');
export const mergeRunning = writable(false);
export const mergeProgress = writable<MergeProgress | null>(null);

export const mergeFileCount = derived(mergeFiles, ($f) => $f.length);
export const mergeTotalSize = derived(mergeFiles, ($f) =>
  $f.reduce((sum, f) => sum + f.size, 0)
);

export async function addFilesForMerge(paths: string[]) {
  if (paths.length === 0) return;

  const firstPath = paths[0];
  const detection = await DetectParts(firstPath);

  const entries: FileEntry[] = await Promise.all(
    detection.parts.map(async (p: string) => {
      const info = await GetFileInfo(p);
      return { path: p, name: info.name, size: info.size };
    })
  );

  mergeFiles.set(entries);

  const output = await SuggestOutputPath(detection.parts, detection.outputName);
  mergeOutputPath.set(output);

  addLog('info', `Detected ${entries.length} parts for ${detection.outputName}`);
}

export function removeFile(path: string) {
  mergeFiles.update((files) => files.filter((f) => f.path !== path));
}

export function clearMergeFiles() {
  mergeFiles.set([]);
  mergeOutputPath.set('');
  mergeProgress.set(null);
}

export async function startMerge() {
  const files = get(mergeFiles);
  const output = get(mergeOutputPath);
  const buffer = get(mergeBufferLabel);

  if (files.length < 2) {
    addLog('warning', 'Need at least 2 files to merge');
    return;
  }
  if (!output) {
    addLog('warning', 'No output path set');
    return;
  }

  mergeRunning.set(true);
  mergeProgress.set(null);

  EventsOn('merge-progress', (data: MergeProgress) => {
    mergeProgress.set(data);
  });

  try {
    const parts = files.map((f) => f.path);
    await MergeFiles(parts, output, buffer);
    addLog('success', `Merged ${files.length} files → ${output.split('/').pop()}`);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg !== 'cancelled') {
      addLog('error', `Merge failed: ${msg}`);
    } else {
      addLog('warning', 'Merge cancelled');
    }
  } finally {
    EventsOff('merge-progress');
    mergeRunning.set(false);
  }
}

export async function cancelMerge() {
  await CancelOperation();
}
