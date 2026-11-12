package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/serjaime99/kvEngine/pkg/storage"
)

func TestAOF_PersistenceAndRecovery(t *testing.T) {
	tempDir := t.TempDir()
	aofPath := filepath.Join(tempDir, "test.aof")

	// Phase 1: Initialize AOF and log commands
	writer, err := storage.NewAOF(aofPath)
	if err != nil {
		t.Fatalf("failed to create AOF: %v", err)
	}

	_ = writer.WriteCommand([]string{"SET", "persistedUser", "SerJaime"})
	_ = writer.WriteCommand([]string{"SET", "counter", "100"})
	_ = writer.WriteCommand([]string{"SET", "temp", "discard"})
	_ = writer.WriteCommand([]string{"DEL", "temp"})
	_ = writer.Sync()
	_ = writer.Close()

	// Phase 2: Simulate complete crash and recovery into a blank engine
	recoveredEngine := storage.NewEngine()
	recoveredCount, err := storage.ReplayAOF(aofPath, recoveredEngine)
	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	if recoveredCount != 4 {
		t.Fatalf("expected 4 recovered commands, got %d", recoveredCount)
	}

	// Verify restored state
	val, ok := recoveredEngine.Get("persistedUser")
	if !ok || val != "SerJaime" {
		t.Fatalf("expected SerJaime, got %s", val)
	}

	val, ok = recoveredEngine.Get("counter")
	if !ok || val != "100" {
		t.Fatalf("expected counter 100, got %s", val)
	}

	if recoveredEngine.Exists("temp") {
		t.Fatalf("expected temp key to be deleted during replay")
	}

	_ = os.Remove(aofPath)
}
