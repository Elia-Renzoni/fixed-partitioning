package options_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fixed-partitioning/internal/options"
)

var file = []byte(`
coordinator: "127.0.0.1:7000"
replication_factor: 3
hash_slots: 128
server_address: "127.0.0.1:6001"
`)

func TestParseConf(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "etc")

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	tempFile, tempErr := os.CreateTemp(dir, "test.yml")
	if tempErr != nil {
		t.Fatal(tempErr)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(file); err != nil {
		t.Fatal(err)
	}

	if err := tempFile.Close(); err != nil {
		t.Fatal(err)
	}

	opt, err := options.ParseConf()
	if err != nil {
		t.Fatal(err)
	}

	if opt.GetCoordinatorAddress() != "127.0.0.1:7000" {
		t.Fatalf("invalid coordinator address, got: %s", opt.GetCoordinatorAddress())
	}

	if opt.GetHashSlots() != 128 {
		t.Fatalf("invalid hash slots, got: %d", opt.GetHashSlots())
	}

	if opt.GetReplicationFactor() != 3 {
		t.Fatalf("invalid replication factor, got: %d", opt.GetReplicationFactor())
	}

	if opt.GetServerAddress() != "127.0.0.1:6001" {
		t.Fatalf("invalid server address, got: %s", opt.GetServerAddress())
	}
}
