import { writable, get } from 'svelte/store';
import { SplitFile, CancelOperation, GetFileInfo } from '../../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime';
import { addLog } from './activity';
import type { FileEntry, SplitProgress } from '../types';

export const splitSource = writable<FileEntry | null>(null);
export const splitOutputDir = writable('');
export const splitChunkLabel = writable('4 GB');
export const splitFormatLabel = writable('_NNN.pkgpart');
export const splitBufferLabel = writable('64 MB');
export const splitRunning = writable(false);
export const splitProgress = writable<SplitProgress | null>(null);

export async function setSourceFile(path: string) {
  const info = await GetFileInfo(path);
  splitSource.set({ path, name: info.name, size: info.size });
  splitOutputDir.set(info.dir);
  addLog('info', `Selected for split: ${info.name}`);
}

export function clearSplitSource() {
  splitSource.set(null);
  splitOutputDir.set('');
  splitProgress.set(null);
}

export async function startSplit() {
  const source = get(splitSource);
  const outputDir = get(splitOutputDir);
  const chunk = get(splitChunkLabel);
  const format = get(splitFormatLabel);
  const buffer = get(splitBufferLabel);

  if (!source) {
    addLog('warning', 'No source file selected');
    return;
  }
  if (!outputDir) {
    addLog('warning', 'No output directory set');
    return;
  }

  splitRunning.set(true);
  splitProgress.set(null);

  EventsOn('split-progress', (data: SplitProgress) => {
    splitProgress.set(data);
  });

  try {
    await SplitFile(source.path, outputDir, chunk, format, buffer);
    addLog('success', `Split ${source.name} → ${outputDir}`);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg !== 'cancelled') {
      addLog('error', `Split failed: ${msg}`);
    } else {
      addLog('warning', 'Split cancelled');
    }
  } finally {
    EventsOff('split-progress');
    splitRunning.set(false);
  }
}

export async function cancelSplit() {
  await CancelOperation();
}
