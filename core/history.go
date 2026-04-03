package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// HistoryEntry records a completed operation.
type HistoryEntry struct {
	ID        string  `json:"id"`
	Timestamp string  `json:"timestamp"`
	Type      string  `json:"type"`
	Input     string  `json:"input"`
	Output    string  `json:"output"`
	Status    string  `json:"status"`
	Duration  float64 `json:"duration"`
	Details   string  `json:"details"`
}

const maxHistoryEntries = 100

func historyPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.json"), nil
}

// LoadHistory reads operation history from disk.
func LoadHistory() []HistoryEntry {
	path, err := historyPath()
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

// AddHistory appends an entry and persists (capped at maxHistoryEntries).
func AddHistory(entry HistoryEntry) error {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().Format(time.RFC3339)
	}

	entries := LoadHistory()
	entries = append([]HistoryEntry{entry}, entries...)
	if len(entries) > maxHistoryEntries {
		entries = entries[:maxHistoryEntries]
	}
	return saveHistory(entries)
}

// ClearHistory removes all entries.
func ClearHistory() error {
	return saveHistory([]HistoryEntry{})
}

func saveHistory(entries []HistoryEntry) error {
	path, err := historyPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
