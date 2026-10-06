package indexer_test

import (
	"astrix/pkg/indexer"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoin(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "proj")
	outside := filepath.Join(base, "secret")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "a.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Irmão com mesmo prefixo textual do root
	sibling := filepath.Join(base, "proj-other")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink indisponível: %v", err)
	}

	ok := []string{"sub/a.go", "", ".", "sub/../sub/a.go", "nao/existe.go"}
	for _, rel := range ok {
		if _, err := indexer.SafeJoin(root, rel); err != nil {
			t.Errorf("SafeJoin(%q) deveria permitir, erro: %v", rel, err)
		}
	}

	bad := []string{"../secret", "../../etc/passwd", "../proj-other/x", "sub/../../secret", "link", "link/file.txt"}
	for _, rel := range bad {
		_, err := indexer.SafeJoin(root, rel)
		if !errors.Is(err, indexer.ErrPathOutsideRoot) {
			t.Errorf("SafeJoin(%q) deveria falhar com ErrPathOutsideRoot, obteve: %v", rel, err)
		}
	}
}
