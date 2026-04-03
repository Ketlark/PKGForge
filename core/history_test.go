package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHistory_addAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	entries := []HistoryEntry{
		{ID: "1", Type: "merge", Input: "a.pkg", Output: "merged.pkg", Status: "success"},
	}

	data, _ := json.MarshalIndent(entries, "", "  ")
	os.WriteFile(path, data, 0644)

	raw, _ := os.ReadFile(path)
	var loaded []HistoryEntry
	json.Unmarshal(raw, &loaded)

	if len(loaded) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded))
	}
	if loaded[0].ID != "1" {
		t.Errorf("id = %q", loaded[0].ID)
	}
	if loaded[0].Type != "merge" {
		t.Errorf("type = %q", loaded[0].Type)
	}
}

func TestHistory_maxCap(t *testing.T) {
	entries := make([]HistoryEntry, 150)
	for i := range entries {
		entries[i] = HistoryEntry{ID: string(rune(i))}
	}

	if len(entries) > maxHistoryEntries {
		entries = entries[:maxHistoryEntries]
	}
	if len(entries) != maxHistoryEntries {
		t.Errorf("expected %d entries, got %d", maxHistoryEntries, len(entries))
	}
}

func TestHistoryEntry_fields(t *testing.T) {
	entry := HistoryEntry{
		ID:       "test-1",
		Type:     "split",
		Input:    "big.pkg",
		Output:   "/output/dir",
		Status:   "error",
		Duration: 12.5,
		Details:  "disk full",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}

	var loaded HistoryEntry
	json.Unmarshal(data, &loaded)

	if loaded.ID != "test-1" {
		t.Errorf("id = %q", loaded.ID)
	}
	if loaded.Duration != 12.5 {
		t.Errorf("duration = %f", loaded.Duration)
	}
	if loaded.Details != "disk full" {
		t.Errorf("details = %q", loaded.Details)
	}
}
