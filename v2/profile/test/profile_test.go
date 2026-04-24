package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reddts/edgegate-core/v2/profile"
)

func TestAddByContent(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "basic_profile.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	const profileName = "local-fixture-profile"
	entity, err := profile.AddByContent(string(content), profileName, false)
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}
	t.Cleanup(func() {
		_ = profile.DeleteById(entity.Id)
	})

	if entity.Id == "" {
		t.Fatal("expected generated profile id")
	}
	if entity.Name != profileName {
		t.Fatalf("expected profile name %q, got %q", profileName, entity.Name)
	}

	stored, err := profile.GetById(entity.Id)
	if err != nil {
		t.Fatalf("expected stored profile, got: %v", err)
	}
	if stored.Name != profileName {
		t.Fatalf("expected stored profile name %q, got %q", profileName, stored.Name)
	}

	infoPath := filepath.Join("data", "profiles", entity.Id+".info")
	storedContent, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatalf("expected stored profile content, got: %v", err)
	}
	if string(storedContent) != string(content) {
		t.Fatalf("expected stored content to match fixture")
	}
}
