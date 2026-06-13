import { writable, derived, get } from 'svelte/store';
import {
  DetectPS2Disc,
  CreatePS2FPKG,
  OpenISOFileDialog,
  OpenImageFileDialog,
  SavePKGFileDialog,
} from '../../../wailsjs/go/main/App';
import { EventsOn } from '../../../wailsjs/runtime/runtime';

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

export const ps2ISOPaths = writable<string[]>([]);
export const ps2OutputPath = writable('');
export const ps2Title = writable('');
export const ps2TitleID = writable('');
export const ps2Icon0 = writable('');
export const ps2Pic1 = writable('');
export const ps2Emulator = writable('jakv2');
export const ps2ConfigTXT = writable('');
export const ps2ConfigLUA = writable('');
export const ps2MemoryCardPath = writable('');
export const ps2WidescreenPatch = writable('');
export const ps2Uprender = writable('off');
export const ps2DisplayMode = writable('4:3');

// Detection state
export const ps2DetectedGameID = writable('');
export const ps2DetectedTitle = writable('');
export const ps2DetectedRegion = writable('');
export const ps2DetectedSystemCNF = writable<Record<string, string>>({});

// Progress
export const ps2Running = writable(false);
export const ps2Progress = writable(0);
export const ps2Error = writable('');

// ---------------------------------------------------------------------------
// Derived
// ---------------------------------------------------------------------------

export const ps2CanCreate = derived(
  [ps2ISOPaths, ps2OutputPath, ps2Running],
  ([$isos, $out, $running]) => $isos.length > 0 && $out !== '' && !$running
);

export const ps2DiscCount = derived(ps2ISOPaths, ($isos) => $isos.length);

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

export async function browseISOs() {
  const paths = await OpenISOFileDialog();
  if (paths && paths.length > 0) {
    ps2ISOPaths.set(paths);
    // Auto-detect from the first ISO
    await detectPS2Disc(paths[0]);
  }
}

export async function addExtraDisc() {
  const paths = await OpenISOFileDialog();
  if (paths && paths.length > 0) {
    const current = get(ps2ISOPaths);
    const combined = [...current, ...paths].slice(0, 5); // max 5 discs
    ps2ISOPaths.set(combined);
  }
}

export function removeISO(index: number) {
  const isos = get(ps2ISOPaths);
  isos.splice(index, 1);
  ps2ISOPaths.set([...isos]);
}

export async function detectPS2Disc(isoPath: string) {
  try {
    ps2Error.set('');
    const result = await DetectPS2Disc(isoPath);
    if (result) {
      ps2DetectedGameID.set(result.gameID);
      ps2DetectedTitle.set(result.title);
      ps2DetectedRegion.set(result.region);
      ps2DetectedSystemCNF.set(result.systemCNF || {});

      if (result.gameID) ps2TitleID.set(result.gameID);
      if (result.title) ps2Title.set(result.title);
    }
  } catch (e: any) {
    ps2Error.set(e?.toString() || 'Detection failed');
  }
}

export async function browseIcon() {
  const path = await OpenImageFileDialog('Select icon (512x512 PNG)');
  if (path) ps2Icon0.set(path);
}

export async function browsePic1() {
  const path = await OpenImageFileDialog('Select background (1920x1080 PNG)');
  if (path) ps2Pic1.set(path);
}

export async function browseOutput() {
  const gameID = get(ps2TitleID) || 'PS2_GAME';
  const path = await SavePKGFileDialog(gameID);
  if (path) ps2OutputPath.set(path);
}

export async function startPS2FPKG() {
  ps2Running.set(true);
  ps2Progress.set(0);
  ps2Error.set('');

  try {
    await CreatePS2FPKG({
      isoPaths: get(ps2ISOPaths),
      outputPath: get(ps2OutputPath),
      title: get(ps2Title),
      titleID: get(ps2TitleID),
      icon0: get(ps2Icon0),
      pic1: get(ps2Pic1),
      emulator: get(ps2Emulator),
      configTxt: get(ps2ConfigTXT),
      configLua: get(ps2ConfigLUA),
      memoryCardPath: get(ps2MemoryCardPath),
      widescreenPatch: get(ps2WidescreenPatch),
      uprender: get(ps2Uprender),
      displayMode: get(ps2DisplayMode),
    });
  } catch (e: any) {
    ps2Error.set(e?.toString() || 'Creation failed');
  } finally {
    ps2Running.set(false);
  }
}

// Listen for progress events
EventsOn('fpkg-progress', (pct: number) => {
  ps2Progress.set(pct);
});
