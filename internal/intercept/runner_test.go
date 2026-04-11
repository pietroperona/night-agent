package intercept_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pietroperona/night-agent/internal/intercept"
)

func TestBuildEnv_SetsDYLD(t *testing.T) {
	dylibPath := "/tmp/guardian-intercept.dylib"
	socketPath := "/tmp/night-agent.sock"

	env := intercept.BuildEnv(os.Environ(), dylibPath, socketPath)

	found := false
	for _, e := range env {
		if e == "DYLD_INSERT_LIBRARIES="+dylibPath {
			found = true
		}
	}
	if !found {
		t.Errorf("DYLD_INSERT_LIBRARIES non trovato nell'env")
	}
}

func TestBuildEnv_SetsSocketPath(t *testing.T) {
	socketPath := "/tmp/night-agent.sock"
	env := intercept.BuildEnv(os.Environ(), "/tmp/test.dylib", socketPath)

	found := false
	for _, e := range env {
		if e == "GUARDIAN_SOCKET="+socketPath {
			found = true
		}
	}
	if !found {
		t.Errorf("GUARDIAN_SOCKET non trovato nell'env")
	}
}

func TestBuildEnv_PreservesExistingEnv(t *testing.T) {
	base := []string{"HOME=/home/user", "PATH=/usr/bin:/bin"}
	env := intercept.BuildEnv(base, "/tmp/test.dylib", "/tmp/night-agent.sock")

	foundHome := false
	for _, e := range env {
		if e == "HOME=/home/user" {
			foundHome = true
		}
	}
	if !foundHome {
		t.Error("variabili d'ambiente originali non preservate")
	}
}

func TestBuildEnv_OverridesDYLDIfAlreadySet(t *testing.T) {
	base := []string{"DYLD_INSERT_LIBRARIES=/old/lib.dylib"}
	newDylib := "/new/guardian-intercept.dylib"
	env := intercept.BuildEnv(base, newDylib, "/tmp/night-agent.sock")

	count := 0
	for _, e := range env {
		if len(e) > 22 && e[:22] == "DYLD_INSERT_LIBRARIES=" {
			count++
			if e != "DYLD_INSERT_LIBRARIES="+newDylib {
				t.Errorf("atteso nuovo valore DYLD_INSERT_LIBRARIES, ottenuto %s", e)
			}
		}
	}
	if count != 1 {
		t.Errorf("attesa esattamente 1 occorrenza di DYLD_INSERT_LIBRARIES, trovate %d", count)
	}
}

func TestFindDylib_FindsInSameDir(t *testing.T) {
	dir := t.TempDir()
	dylibPath := filepath.Join(dir, "guardian-intercept.dylib")
	_ = os.WriteFile(dylibPath, []byte("fake dylib"), 0755)

	found, err := intercept.FindDylib(dir)
	if err != nil {
		t.Fatalf("atteso nessun errore, ottenuto: %v", err)
	}
	if found != dylibPath {
		t.Errorf("atteso %s, ottenuto %s", dylibPath, found)
	}
}

func TestFindDylib_ErrorIfNotFound(t *testing.T) {
	_, err := intercept.FindDylib("/nonexistent/dir")
	if err == nil {
		t.Fatal("atteso errore se dylib non trovata, ottenuto nil")
	}
}
