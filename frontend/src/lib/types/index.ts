export interface MergeProgress {
  currentFileIndex: number;
  totalFiles: number;
  bytesProcessed: number;
  totalBytes: number;
  currentFileName: string;
  speedBPS: number;
  etaSeconds: number;
}

export interface SplitProgress {
  currentPart: number;
  totalParts: number;
  bytesWritten: number;
  totalBytes: number;
  speedBPS: number;
  etaSeconds: number;
}

export interface FileEntry {
  path: string;
  name: string;
  size: number;
}

/** Mirrors core.PKGInfo JSON from Wails (do not import gitignored wailsjs/go/models.ts in CI). */
export interface PKGInfo {
  contentId: string;
  titleId: string;
  region: string;
  contentType: string;
  drmType: string;
  fileSize: number;
  pkgSize: number;
  valid: boolean;
  error?: string;
}

/** Mirrors core.ChecksumResult JSON from Wails. */
export interface ChecksumResult {
  sha256: string;
  size: number;
  duration: number;
}

export interface LogEntry {
  id: number;
  timestamp: Date;
  type: 'info' | 'success' | 'error' | 'warning';
  message: string;
}
