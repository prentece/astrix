package indexer

import (
	"astrix/pkg/storage"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// MaxIndexableFileSize define o tamanho máximo padrão (2MB) de arquivos de código a serem indexados.
var MaxIndexableFileSize int64 = 2 * 1024 * 1024

// DefaultIgnoredDirs são pastas comumente ignoradas em repositórios.
var DefaultIgnoredDirs = map[string]bool{
	".git":          true,
	".vscode":       true,
	".idea":         true,
	"node_modules":  true,
	"vendor":        true,
	"__pycache__":   true,
	".venv":         true,
	"venv":          true,
	"env":           true,
	"dist":          true,
	"build":         true,
	"target":        true,
	"bin":           true,
	"obj":           true,
	".next":         true,
	".nuxt":         true,
	".turbo":        true,
	"coverage":      true,
	".cache":        true,
	".pytest_cache": true,
}

// FileInfo representa um arquivo encontrado durante o escaneamento.
type FileInfo struct {
	RelPath string
	AbsPath string
	Size    int64
	Config  LanguageConfig
}

// IsBinaryFile detecta se um arquivo é binário inspecionando os primeiros 512 bytes em busca de bytes NUL (0x00).
func IsBinaryFile(absPath string) bool {
	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	return bytes.IndexByte(buf[:n], 0) != -1
}

// LoadGitIgnore carrega e compila regras de ignore a partir do .gitignore da raiz,
// .git/info/exclude (se existir) e .gitignore aninhados em subdiretórios.
func LoadGitIgnore(rootDir string) *ignore.GitIgnore {
	var lines []string

	// 1. .gitignore na raiz
	rootGitignore := filepath.Join(rootDir, ".gitignore")
	if data, err := os.ReadFile(rootGitignore); err == nil {
		lines = append(lines, strings.Split(string(data), "\n")...)
	}

	// 2. .git/info/exclude
	gitExclude := filepath.Join(rootDir, ".git", "info", "exclude")
	if data, err := os.ReadFile(gitExclude); err == nil {
		lines = append(lines, strings.Split(string(data), "\n")...)
	}

	// 3. Procura .gitignore aninhados em subpastas
	_ = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || DefaultIgnoredDirs[base] {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == ".gitignore" && path != rootGitignore {
			relDir, err := filepath.Rel(rootDir, filepath.Dir(path))
			if err == nil && relDir != "." && relDir != "" {
				if data, err := os.ReadFile(path); err == nil {
					subLines := strings.Split(string(data), "\n")
					for _, l := range subLines {
						l = strings.TrimSpace(l)
						if l == "" || strings.HasPrefix(l, "#") {
							continue
						}
						negated := strings.HasPrefix(l, "!")
						pattern := strings.TrimPrefix(l, "!")
						prefix := filepath.ToSlash(relDir)
						if !strings.HasPrefix(pattern, "/") {
							pattern = prefix + "/" + pattern
						} else {
							pattern = prefix + pattern
						}
						if negated {
							pattern = "!" + pattern
						}
						lines = append(lines, pattern)
					}
				}
			}
		}
		return nil
	})

	if len(lines) == 0 {
		return nil
	}
	return ignore.CompileIgnoreLines(lines...)
}

// ScanRepository percorre o diretório do projeto respeitando .gitignore e pastas ignoradas.
func ScanRepository(rootDir string) ([]FileInfo, error) {
	var files []FileInfo

	gitIgnore := LoadGitIgnore(rootDir)

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Ignora erros de permissão pontuais e continua
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}

		if relPath == "." {
			return nil
		}

		// Verifica se é diretório ignorado por padrão
		if info.IsDir() {
			baseName := filepath.Base(path)
			if DefaultIgnoredDirs[baseName] || baseName == ".git" {
				return filepath.SkipDir
			}
			if gitIgnore != nil && gitIgnore.MatchesPath(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Checa .gitignore para arquivos
		if gitIgnore != nil && gitIgnore.MatchesPath(relPath) {
			return nil
		}

		// Ignora arquivos muito grandes ou vazios
		if info.Size() > MaxIndexableFileSize || info.Size() == 0 {
			return nil
		}

		// Ignora arquivos binários inspecionando byte NUL
		if IsBinaryFile(path) {
			return nil
		}

		// Identifica se possui parser de linguagem configurado
		cfg, ok := GetConfigByFilePath(relPath)
		if ok {
			files = append(files, FileInfo{
				RelPath: relPath,
				AbsPath: path,
				Size:    info.Size(),
				Config:  cfg,
			})
		}

		return nil
	})

	return files, err
}

// BuildDirectoryTree gera a árvore estruturada de arquivos a partir de um subcaminho relativo.
func BuildDirectoryTree(rootDir, subPath string, maxDepth int) (*storage.FileNode, error) {
	targetAbs, err := SafeJoin(rootDir, subPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(targetAbs)
	if err != nil {
		return nil, err
	}

	gitIgnore := LoadGitIgnore(rootDir)

	relPath, _ := filepath.Rel(rootDir, targetAbs)
	if relPath == "." {
		relPath = ""
	}

	rootNode := &storage.FileNode{
		Name:  filepath.Base(targetAbs),
		Path:  relPath,
		IsDir: info.IsDir(),
		Size:  info.Size(),
	}

	if !info.IsDir() {
		return rootNode, nil
	}

	var buildChildren func(currentAbs string, depth int) ([]*storage.FileNode, error)
	buildChildren = func(currentAbs string, depth int) ([]*storage.FileNode, error) {
		if depth > maxDepth {
			return nil, nil
		}

		entries, err := os.ReadDir(currentAbs)
		if err != nil {
			return nil, err
		}

		var children []*storage.FileNode
		for _, entry := range entries {
			name := entry.Name()
			if DefaultIgnoredDirs[name] || strings.HasPrefix(name, ".") {
				continue
			}

			entryAbs := filepath.Join(currentAbs, name)
			entryRel, _ := filepath.Rel(rootDir, entryAbs)

			if gitIgnore != nil && gitIgnore.MatchesPath(entryRel) {
				continue
			}

			entryInfo, err := entry.Info()
			if err != nil {
				continue
			}

			node := &storage.FileNode{
				Name:  name,
				Path:  entryRel,
				IsDir: entry.IsDir(),
				Size:  entryInfo.Size(),
			}

			if entry.IsDir() {
				subChildren, _ := buildChildren(entryAbs, depth+1)
				node.Children = subChildren
			}

			children = append(children, node)
		}

		// Ordena diretórios primeiro, depois alfabeticamente
		sort.Slice(children, func(i, j int) bool {
			if children[i].IsDir != children[j].IsDir {
				return children[i].IsDir
			}
			return children[i].Name < children[j].Name
		})

		return children, nil
	}

	children, err := buildChildren(targetAbs, 1)
	if err != nil {
		return nil, err
	}
	rootNode.Children = children

	return rootNode, nil
}

// BuildTreeText gera uma árvore textual formatada (├──, └──, │) respeitando limite de profundidade (depth),
// showHidden e exclusão de diretórios ignorados/.gitignore.
func BuildTreeText(rootDir, subPath string, maxDepth int, showHidden bool) (string, error) {
	if maxDepth <= 0 {
		maxDepth = 2
	}

	targetAbs, err := SafeJoin(rootDir, subPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(targetAbs)
	if err != nil {
		return "", err
	}

	if !info.IsDir() {
		return filepath.Base(targetAbs) + "\n", nil
	}

	gitIgnore := LoadGitIgnore(rootDir)

	var sb strings.Builder

	// Header do nó raiz
	cleanSub := filepath.Clean(subPath)
	if cleanSub == "." || cleanSub == "" || cleanSub == "/" {
		sb.WriteString("root/\n")
	} else {
		sb.WriteString(filepath.ToSlash(cleanSub))
		sb.WriteString("/\n")
	}

	isDirIgnored := func(name, relPath string) bool {
		if !showHidden && strings.HasPrefix(name, ".") {
			return true
		}
		if DefaultIgnoredDirs[name] {
			return true
		}
		if gitIgnore != nil && gitIgnore.MatchesPath(relPath) {
			return true
		}
		return false
	}

	isFileIgnored := func(name, relPath string) bool {
		if !showHidden && strings.HasPrefix(name, ".") {
			return true
		}
		if gitIgnore != nil && gitIgnore.MatchesPath(relPath) {
			return true
		}
		return false
	}

	hasVisibleChildren := func(dirAbs, dirRel string) bool {
		entries, err := os.ReadDir(dirAbs)
		if err != nil {
			return false
		}
		for _, e := range entries {
			eName := e.Name()
			eRel := filepath.Join(dirRel, eName)
			if e.IsDir() {
				if !isDirIgnored(eName, eRel) {
					return true
				}
			} else {
				if !isFileIgnored(eName, eRel) {
					return true
				}
			}
		}
		return false
	}

	type entryItem struct {
		name  string
		isDir bool
	}

	var renderLevel func(currentAbs, currentRel, prefix string, depth int) error
	renderLevel = func(currentAbs, currentRel, prefix string, depth int) error {
		entries, err := os.ReadDir(currentAbs)
		if err != nil {
			return err
		}

		var items []entryItem
		for _, e := range entries {
			name := e.Name()
			entryRel := filepath.Join(currentRel, name)
			if e.IsDir() {
				if isDirIgnored(name, entryRel) {
					continue
				}
				items = append(items, entryItem{name: name, isDir: true})
			} else {
				if isFileIgnored(name, entryRel) {
					continue
				}
				items = append(items, entryItem{name: name, isDir: false})
			}
		}

		// Ordenação determinística: diretórios primeiro (alfabético), depois arquivos (alfabético)
		sort.Slice(items, func(i, j int) bool {
			if items[i].isDir != items[j].isDir {
				return items[i].isDir
			}
			return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
		})

		count := len(items)
		for i, item := range items {
			isLast := (i == count-1)
			branch := "├── "
			nextPrefix := prefix + "│   "
			if isLast {
				branch = "└── "
				nextPrefix = prefix + "    "
			}

			if item.isDir {
				childAbs := filepath.Join(currentAbs, item.name)
				childRel := filepath.Join(currentRel, item.name)
				hasChildren := hasVisibleChildren(childAbs, childRel)

				if depth >= maxDepth {
					if hasChildren {
						sb.WriteString(prefix)
						sb.WriteString(branch)
						sb.WriteString(item.name)
						sb.WriteString("/ (dir)\n")
					} else {
						sb.WriteString(prefix)
						sb.WriteString(branch)
						sb.WriteString(item.name)
						sb.WriteString("/\n")
					}
				} else {
					sb.WriteString(prefix)
					sb.WriteString(branch)
					sb.WriteString(item.name)
					sb.WriteString("/\n")
					if hasChildren {
						_ = renderLevel(childAbs, childRel, nextPrefix, depth+1)
					}
				}
			} else {
				sb.WriteString(prefix)
				sb.WriteString(branch)
				sb.WriteString(item.name)
				sb.WriteString("\n")
			}
		}

		return nil
	}

	targetRel, _ := filepath.Rel(rootDir, targetAbs)
	if targetRel == "." {
		targetRel = ""
	}

	if err := renderLevel(targetAbs, targetRel, "", 1); err != nil {
		return "", err
	}

	return sb.String(), nil
}
