import { writable, derived, get } from 'svelte/store';
import {
  DetectPS1Disc,
  CreatePS1FPKG,
  OpenCUEFileDialog,
  OpenImageFileDialog,
  SavePKGFileDialog,
} from '../../../wailsjs/go/main/App';
import { EventsOn } from '../../../wailsjs/runtime/runtime';

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

export const ps1CuePath = writable('');
export const ps1ExtraDiscs = writable<string[]>([]);
export const ps1OutputPath = writable('');
export const ps1Title = writable('');
export const ps1TitleID = writable('');
export const ps1Icon0 = writable('');
export const ps1Pic1 = writable('');
export const ps1Emulator = writable('ps1_emu');
export const ps1AnalogSticks = writable(false);
export const ps1SkipBootLogo = writable(false);
export const ps1Force60Hz = writable(false);
export const ps1EnableCDDATOC = writable(false);

// Detection state
export const ps1DetectedGameID = writable('');
export const ps1DetectedTitle = writable('');
export const ps1DetectedRegion = writable('');
export const ps1DetectedTrackNum = writable(0);
export const ps1DetectedHasCDDA = writable(false);
export const ps1DetectedIsMultiBin = writable(false);

// Progress
export const ps1Running = writable(false);
export const ps1Progress = writable(0);
export const ps1Error = writable('');

// ---------------------------------------------------------------------------
// Derived
// ---------------------------------------------------------------------------

export const ps1CanCreate = derived(
  [ps1CuePath, ps1OutputPath, ps1Running],
  ([$cue, $out, $running]) => $cue !== '' && $out !== '' && !$running
);

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

export async function browseCUE() {
  const path = await OpenCUEFileDialog();
  if (path) {
    ps1CuePath.set(path);
    await detectPS1Disc(path);
  }
}

export async function detectPS1Disc(cuePath: string) {
  try {
    ps1Error.set('');
    const result = await DetectPS1Disc(cuePath);
    if (result) {
      ps1DetectedGameID.set(result.gameID);
      ps1DetectedTitle.set(result.title);
      ps1DetectedRegion.set(result.region);
      ps1DetectedTrackNum.set(result.trackNum);
      ps1DetectedHasCDDA.set(result.hasCdda);
      ps1DetectedIsMultiBin.set(result.isMultiBin);

      if (result.gameID) ps1TitleID.set(result.gameID);
      if (result.title) ps1Title.set(result.title);
    }
  } catch (e: any) {
    ps1Error.set(e?.toString() || 'Detection failed');
  }
}

export async function browseIcon() {
  const path = await OpenImageFileDialog('Select icon (512x512 PNG)');
  if (path) ps1Icon0.set(path);
}

export async function browsePic1() {
  const path = await OpenImageFileDialog('Select background (1920x1080 PNG)');
  if (path) ps1Pic1.set(path);
}

export async function browseOutput() {
  const gameID = get(ps1TitleID) || 'PS1_GAME';
  const path = await SavePKGFileDialog(gameID);
  if (path) ps1OutputPath.set(path);
}

export async function addExtraDisc() {
  const path = await OpenCUEFileDialog();
  if (path) {
    const discs = get(ps1ExtraDiscs);
    if (discs.length < 3) {
      discs.push(path);
      ps1ExtraDiscs.set([...discs]);
    }
  }
}

export function removeExtraDisc(index: number) {
  const discs = get(ps1ExtraDiscs);
  discs.splice(index, 1);
  ps1ExtraDiscs.set([...discs]);
}

export async function startPS1FPKG() {
  ps1Running.set(true);
  ps1Progress.set(0);
  ps1Error.set('');

  try {
    await CreatePS1FPKG({
      cuePath: get(ps1CuePath),
      extraDiscs: get(ps1ExtraDiscs),
      outputPath: get(ps1OutputPath),
      title: get(ps1Title),
      titleID: get(ps1TitleID),
      icon0: get(ps1Icon0),
      pic1: get(ps1Pic1),
      emulator: get(ps1Emulator),
      analogSticks: get(ps1AnalogSticks),
      skipBootLogo: get(ps1SkipBootLogo),
      force60Hz: get(ps1Force60Hz),
      enableCddaToc: get(ps1EnableCDDATOC),
    });
  } catch (e: any) {
    ps1Error.set(e?.toString() || 'Creation failed');
  } finally {
    ps1Running.set(false);
  }
}

// Listen for progress events
EventsOn('fpkg-progress', (pct: number) => {
  ps1Progress.set(pct);
});
