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

export interface LogEntry {
  id: number;
  timestamp: Date;
  type: 'info' | 'success' | 'error' | 'warning';
  message: string;
}
