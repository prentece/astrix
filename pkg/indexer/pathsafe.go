package indexer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathOutsideRoot é retornado quando um caminho relativo tenta escapar da raiz do projeto.
var ErrPathOutsideRoot = errors.New("caminho fora da raiz do projeto")

// SafeJoin junta root e rel garantindo que o resultado permaneça dentro de root.
// Rejeita traversal lexical ("../") e symlinks que apontem para fora da raiz.
// Caminhos inexistentes são aceitos (validados apenas lexicalmente) para que o chamador
// reporte o erro de I/O normal.
func SafeJoin(root, rel string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("raiz inválida: %w", err)
	}

	target := filepath.Join(rootAbs, rel)
	if !isWithin(rootAbs, target) {
		return "", fmt.Errorf("%w: %q", ErrPathOutsideRoot, rel)
	}

	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("raiz inacessível: %w", err)
	}
	realTarget, err := evalExistingPrefix(target)
	if err != nil {
		return "", err
	}
	if !isWithin(realRoot, realTarget) {
		return "", fmt.Errorf("%w: %q", ErrPathOutsideRoot, rel)
	}
	return target, nil
}

// evalExistingPrefix resolve symlinks do ancestral existente mais profundo de p
// e reanexa o restante (inexistente) lexicalmente.
func evalExistingPrefix(p string) (string, error) {
	rest := ""
	cur := p
	for {
		realPath, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(realPath, rest), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", err
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// isWithin informa se target é igual a root ou está contido nele.
func isWithin(root, target string) bool {
	r, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}
