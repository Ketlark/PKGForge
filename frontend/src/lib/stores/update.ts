import { writable, derived, get } from 'svelte/store';
import {
  AppVersion,
  CheckForUpdates,
  ConfigureUpdateOnStartup,
  DownloadAndApplyUpdate,
  UpdateBackend,
} from '../../../wailsjs/go/main/App';
import { EventsOn, EventsOff, Quit } from '../../../wailsjs/runtime/runtime';
import { addLog } from './activity';

export type UpdateInfo = {
  version: string;
  releaseUrl: string;
  releaseNotes: string;
  assetName: string;
  assetUrl: string;
  assetSize: number;
};

export type UpdateBackendKind = 'sparkle' | 'builtin';

export type UpdateStatus =
  | 'idle'
  | 'checking'
  | 'available'
  | 'upToDate'
  | 'downloading'
  | 'ready'
  | 'error';

export const updateBackend = writable<UpdateBackendKind>('builtin');
export const updateStatus = writable<UpdateStatus>('idle');
export const currentVersion = writable('…');
export const availableUpdate = writable<UpdateInfo | null>(null);
export const updateProgress = writable(0);
export const updateError = writable<string | null>(null);

/** True when a startup check found an update (built-in backend only). */
export const updateBadge = writable(false);

export const updateBusy = derived(
  updateStatus,
  ($s) => $s === 'checking' || $s === 'downloading'
);

/** Human-readable version label for UI (e.g. v1.3.0 or devLabel). */
export function formatDisplayVersion(version: string, devLabel: string): string {
  if (!version || version === 'dev' || version.startsWith('dev')) {
    return devLabel;
  }
  return version.startsWith('v') ? version : `v${version}`;
}

let progressUnsub: (() => void) | null = null;

function clearProgressListener() {
  if (progressUnsub) {
    progressUnsub();
    progressUnsub = null;
  }
  EventsOff('update-progress');
}

export async function loadAppVersion() {
  try {
    const v = await AppVersion();
    currentVersion.set(v || 'dev');
  } catch {
    currentVersion.set('dev');
  }
}

async function loadUpdateBackend() {
  try {
    const backend = await UpdateBackend();
    updateBackend.set(backend === 'sparkle' ? 'sparkle' : 'builtin');
  } catch {
    updateBackend.set('builtin');
  }
}

export async function checkForUpdates(options: { silent?: boolean } = {}) {
  const { silent = false } = options;
  if (get(updateBusy)) return;

  if (get(updateBackend) === 'sparkle') {
    if (!silent) {
      updateStatus.set('checking');
      updateError.set(null);
    }
    try {
      await CheckForUpdates();
      if (!silent) {
        updateStatus.set('idle');
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (!silent) {
        updateStatus.set('error');
        updateError.set(msg);
        addLog('error', `Update check failed: ${msg}`);
      }
    }
    return;
  }

  updateStatus.set('checking');
  updateError.set(null);
  if (!silent) {
    availableUpdate.set(null);
    updateBadge.set(false);
  }

  try {
    const info = (await CheckForUpdates()) as UpdateInfo | null;
    if (info?.version) {
      availableUpdate.set(info);
      updateStatus.set('available');
      updateBadge.set(true);
      if (!silent) {
        addLog('info', `Update available: v${info.version}`);
      }
    } else {
      availableUpdate.set(null);
      updateStatus.set('upToDate');
      updateBadge.set(false);
      if (!silent) {
        addLog('success', 'Application is up to date');
      }
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    updateStatus.set('error');
    updateError.set(msg);
    if (!silent) {
      addLog('error', `Update check failed: ${msg}`);
    }
  }
}

export async function downloadAndApplyUpdate() {
  if (get(updateBackend) === 'sparkle') return;

  const info = get(availableUpdate);
  if (!info || get(updateBusy)) return;

  updateStatus.set('downloading');
  updateProgress.set(0);
  updateError.set(null);
  clearProgressListener();

  progressUnsub = EventsOn('update-progress', (p: number) => {
    if (typeof p === 'number' && Number.isFinite(p)) {
      updateProgress.set(Math.min(1, Math.max(0, p)));
    }
  });

  try {
    await DownloadAndApplyUpdate(info);
    updateProgress.set(1);
    updateStatus.set('ready');
    updateBadge.set(false);
    addLog('success', `Update v${info.version} installed — restart to apply`);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    updateStatus.set('error');
    updateError.set(msg);
    addLog('error', `Update failed: ${msg}`);
  } finally {
    clearProgressListener();
  }
}

export function restartApp() {
  Quit();
}

export async function initUpdateOnStartup(checkOnStartup: boolean) {
  await loadAppVersion();
  await loadUpdateBackend();
  try {
    await ConfigureUpdateOnStartup(checkOnStartup);
  } catch {
    /* non-fatal */
  }
  if (get(updateBackend) === 'builtin' && checkOnStartup) {
    void checkForUpdates({ silent: true });
  }
}
