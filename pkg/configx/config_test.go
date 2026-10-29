package configx

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMergesLocalOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	write(t, path, "db:\n  dsn: dev\nstorage:\n  key_id: \"\"\n  secret: \"\"\n  endpoint: https://s3.example.com\n")
	write(t, localPath(path), "storage:\n  key_id: k1\n  secret: s3cret\n")

	var cfg structpb.Struct
	if err := Load(path, &cfg); err != nil {
		t.Fatal(err)
	}
	st := cfg.Fields["storage"].GetStructValue().Fields
	if got := st["secret"].GetStringValue(); got != "s3cret" {
		t.Fatalf("storage.secret = %q, want override", got)
	}
	if got := st["endpoint"].GetStringValue(); got != "https://s3.example.com" {
		t.Fatalf("storage.endpoint = %q, want base value kept", got)
	}
	if got := cfg.Fields["db"].GetStructValue().Fields["dsn"].GetStringValue(); got != "dev" {
		t.Fatalf("db.dsn = %q, want base value kept", got)
	}
}

func TestLoadWithoutLocalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	write(t, path, "storage:\n  key_id: base\n")

	var cfg structpb.Struct
	if err := Load(path, &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Fields["storage"].GetStructValue().Fields["key_id"].GetStringValue(); got != "base" {
		t.Fatalf("storage.key_id = %q", got)
	}
}

func TestLocalPath(t *testing.T) {
	if got := localPath("configs/config.yaml"); got != "configs/config.local.yaml" {
		t.Fatalf("localPath = %q", got)
	}
}
