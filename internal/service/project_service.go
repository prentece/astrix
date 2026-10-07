package service

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// ProjectWatcher define a interface de escuta de arquivos para registro/remoção dinâmica de projetos.
type ProjectWatcher interface {
	WatchProject(proj *storage.Project) error
	UnwatchProject(projectID string)
}

// ProjectService gerencia o ciclo de vida de projetos e navegação no sistema de arquivos do host.
type ProjectService struct {
	projectRepo storage.ProjectRepository
	symbolRepo  storage.SymbolRepository
	engine      *indexer.Engine
	watcher     ProjectWatcher
}

// NewProjectService cria uma nova instância de ProjectService.
func NewProjectService(projectRepo storage.ProjectRepository, symbolRepo storage.SymbolRepository, engine *indexer.Engine) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		symbolRepo:  symbolRepo,
		engine:      engine,
	}
}

// SetWatcher vincula o serviço de monitoramento em tempo real de arquivos.
func (s *ProjectService) SetWatcher(watcher ProjectWatcher) {
	s.watcher = watcher
}

// ProjectRepo retorna o repositório de projetos subjacente.
func (s *ProjectService) ProjectRepo() storage.ProjectRepository {
	return s.projectRepo
}

// ListAll retorna todos os projetos cadastrados.
func (s *ProjectService) ListAll() ([]*storage.Project, error) {
	projects, err := s.projectRepo.ListAll()
	if err != nil {
		return nil, err
	}
	if projects == nil {
		return []*storage.Project{}, nil
	}
	return projects, nil
}

// GetByID busca um projeto pelo ID.
func (s *ProjectService) GetByID(id string) (*storage.Project, error) {
	if id == "" {
		return nil, errors.New("id do projeto não fornecido")
	}
	return s.projectRepo.GetByID(id)
}

// SetAutoSync atualiza o status de sincronização automática de um projeto.
func (s *ProjectService) SetAutoSync(id string, autoSync bool) error {
	if id == "" {
		return errors.New("id do projeto não fornecido")
	}
	if err := s.projectRepo.SetAutoSync(id, autoSync); err != nil {
		return err
	}
	return nil
}

// RegisterProject valida e cadastra um novo projeto, iniciando a indexação assíncrona.
func (s *ProjectService) RegisterProject(name, path, language string) (*storage.Project, error) {
	name = strings.TrimSpace(name)
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("o campo 'path' é obrigatório")
	}
	if language == "" {
		language = "auto"
	}

	cleanPath := filepath.Clean(path)

	// Valida se o diretório existe
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("caminho não encontrado ou inacessível: %s (%w)", cleanPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("o caminho especificado não é um diretório: %s", cleanPath)
	}

	detName, detLang := DetectProjectManifestInfo(cleanPath)
	if name == "" {
		if detName != "" {
			name = detName
		} else {
			name = FormatProjectDisplayName(filepath.Base(cleanPath))
		}
	} else {
		name = FormatProjectDisplayName(name)
	}
	if name == "" {
		return nil, errors.New("o campo 'name' é obrigatório")
	}
	if language == "" || language == "auto" {
		if detLang != "" && detLang != "auto" {
			language = detLang
		} else {
			language = "auto"
		}
	}

	// Verifica se já existe um projeto cadastrado com o mesmo caminho
	all, _ := s.projectRepo.ListAll()
	for _, p := range all {
		if p.Path == cleanPath {
			return nil, fmt.Errorf("este repositório já está cadastrado no Astrix (ID: %s)", p.ID)
		}
	}

	// Gera ID curto com prefixo proj- (ex: proj-5429ad94)
	shortUUID := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	proj := &storage.Project{
		ID:       fmt.Sprintf("proj-%s", shortUUID),
		Name:     name,
		Path:     cleanPath,
		Language: language,
		AutoSync: true,
	}

	if err := s.projectRepo.Create(proj); err != nil {
		return nil, fmt.Errorf("falha ao salvar projeto no banco: %w", err)
	}

	// Registra no FileWatcher se ativo
	if s.watcher != nil {
		_ = s.watcher.WatchProject(proj)
	}

	return proj, nil
}

// ReindexProject força a sincronização incremental ou indexação completa de um projeto de forma síncrona.
func (s *ProjectService) ReindexProject(id string) (*storage.Project, error) {
	proj, err := s.projectRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if proj == nil {
		return nil, errors.New("projeto não encontrado")
	}

	if s.engine != nil {
		if proj.FileCount == 0 || proj.Status == storage.StatusPending {
			if err := s.engine.IndexProject(id); err != nil {
				return nil, err
			}
		} else {
			if _, err := s.engine.ProcessIncrementalDelta(id); err != nil {
				return nil, err
			}
		}
	}

	return s.projectRepo.GetByID(id)
}

// GetIndexingProgress retorna o estado em tempo real da indexação ativa do projeto.
func (s *ProjectService) GetIndexingProgress(projectID string) *storage.IndexingProgress {
	if s.engine != nil {
		return s.engine.GetIndexingProgress(projectID)
	}
	return nil
}

// DeleteProject remove o projeto, seus símbolos e dados associados.
func (s *ProjectService) DeleteProject(id string) error {
	if id == "" {
		return errors.New("id do projeto não fornecido")
	}

	if s.watcher != nil {
		s.watcher.UnwatchProject(id)
	}

	if err := s.projectRepo.Delete(id); err != nil {
		return err
	}

	return nil
}

// AvailableRepo representa um repositório montado e detectado no host.
type AvailableRepo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ListAvailableRepos descobre e lista os diretórios montados no container (/mnt/host ou /mnt/repos).
func (s *ProjectService) ListAvailableRepos() ([]AvailableRepo, error) {
	reposPath := os.Getenv("REPOS_MOUNT_PATH")
	if reposPath == "" {
		reposPath = "/mnt/repos"
	}
	if _, err := os.Stat(reposPath); os.IsNotExist(err) {
		reposPath = "./repos"
	}

	var results []AvailableRepo
	entries, err := os.ReadDir(reposPath)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				fullPath := filepath.Join(reposPath, entry.Name())
				results = append(results, AvailableRepo{
					Name: entry.Name(),
					Path: fullPath,
				})
			}
		}
	}

	if len(results) == 0 {
		if _, err := os.Stat(reposPath); err == nil {
			results = append(results, AvailableRepo{
				Name: filepath.Base(reposPath) + " (Raiz)",
				Path: reposPath,
			})
		}
	}

	return results, nil
}

// FormatProjectDisplayName converte strings em kebab-case, snake_case ou minúsculas em Title Case legível.
// Ex: "fibra-backend-core" -> "Fibra Backend Core", "astrix-engine" -> "Astrix Engine", "customer_care" -> "Customer Care".
func FormatProjectDisplayName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Remove escopos npm como @org/meu-pacote -> meu-pacote
	if strings.HasPrefix(raw, "@") && strings.Contains(raw, "/") {
		parts := strings.SplitN(raw, "/", 2)
		raw = parts[1]
	}

	// Se for path de URL como github.com/user/repo -> repo
	if strings.Contains(raw, "/") {
		parts := strings.Split(raw, "/")
		raw = parts[len(parts)-1]
	}

	// Substitui hífens, sublinhados e pontos por espaços
	cleaned := strings.ReplaceAll(raw, "-", " ")
	cleaned = strings.ReplaceAll(cleaned, "_", " ")
	cleaned = strings.ReplaceAll(cleaned, ".", " ")

	words := strings.Fields(cleaned)
	for i, w := range words {
		lower := strings.ToLower(w)
		if len(lower) > 0 {
			words[i] = strings.ToUpper(lower[:1]) + lower[1:]
		}
	}
	return strings.Join(words, " ")
}

// DetectProjectManifestInfo inspeciona os arquivos de manifesto existentes no diretório
// ("package.json", "go.mod", "pom.xml", "build.gradle", "composer.json", "requirements.txt", "Cargo.toml", "dubbo.properties")
// e extrai o nome e a linguagem principal do projeto.
func DetectProjectManifestInfo(dirPath string) (name, lang string) {
	if dirPath == "" {
		return "", "auto"
	}

	// 1. package.json (Node / TypeScript / JavaScript)
	pkgPath := filepath.Join(dirPath, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Name != "" {
			name = FormatProjectDisplayName(pkg.Name)
		}
		if _, err := os.Stat(filepath.Join(dirPath, "tsconfig.json")); err == nil {
			lang = "typescript"
		} else {
			lang = "javascript"
		}
		if name != "" {
			return name, lang
		}
	}

	// 2. go.mod (Go)
	goModPath := filepath.Join(dirPath, "go.mod")
	if data, err := os.ReadFile(goModPath); err == nil {
		lang = "go"
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				modName := strings.TrimSpace(strings.TrimPrefix(line, "module"))
				name = FormatProjectDisplayName(modName)
				break
			}
		}
		if name != "" {
			return name, lang
		}
	}

	// 3. Cargo.toml (Rust)
	cargoPath := filepath.Join(dirPath, "Cargo.toml")
	if data, err := os.ReadFile(cargoPath); err == nil {
		lang = "rust"
		lines := strings.Split(string(data), "\n")
		inPackage := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "[package]" {
				inPackage = true
				continue
			}
			if inPackage && strings.HasPrefix(trimmed, "name") && strings.Contains(trimmed, "=") {
				parts := strings.SplitN(trimmed, "=", 2)
				val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				name = FormatProjectDisplayName(val)
				break
			}
		}
		if name != "" {
			return name, lang
		}
	}

	// 4. pom.xml (Java Maven)
	pomPath := filepath.Join(dirPath, "pom.xml")
	if data, err := os.ReadFile(pomPath); err == nil {
		lang = "java"
		content := string(data)
		if idx := strings.Index(content, "<artifactId>"); idx != -1 {
			endIdx := strings.Index(content[idx:], "</artifactId>")
			if endIdx != -1 {
				artId := content[idx+len("<artifactId>") : idx+endIdx]
				name = FormatProjectDisplayName(artId)
			}
		}
		if name != "" {
			return name, lang
		}
	}

	// 5. build.gradle / build.gradle.kts / settings.gradle (Java/Kotlin Gradle)
	for _, gradleFile := range []string{"settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts"} {
		gPath := filepath.Join(dirPath, gradleFile)
		if data, err := os.ReadFile(gPath); err == nil {
			lang = "java"
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.Contains(line, "rootProject.name") && strings.Contains(line, "=") {
					parts := strings.SplitN(line, "=", 2)
					val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					name = FormatProjectDisplayName(val)
					break
				}
			}
			if name != "" {
				return name, lang
			}
		}
	}

	// 6. composer.json (PHP)
	composerPath := filepath.Join(dirPath, "composer.json")
	if data, err := os.ReadFile(composerPath); err == nil {
		lang = "php"
		var comp struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &comp) == nil && comp.Name != "" {
			name = FormatProjectDisplayName(comp.Name)
			return name, lang
		}
	}

	// 7. dubbo.properties (Java Dubbo)
	dubboPath := filepath.Join(dirPath, "dubbo.properties")
	if data, err := os.ReadFile(dubboPath); err == nil {
		lang = "java"
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "dubbo.application.name=") {
				val := strings.TrimPrefix(line, "dubbo.application.name=")
				name = FormatProjectDisplayName(val)
				return name, lang
			}
		}
	}

	// 8. pyproject.toml / requirements.txt (Python)
	pyprojectPath := filepath.Join(dirPath, "pyproject.toml")
	if data, err := os.ReadFile(pyprojectPath); err == nil {
		lang = "python"
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "name") && strings.Contains(trimmed, "=") {
				parts := strings.SplitN(trimmed, "=", 2)
				val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				name = FormatProjectDisplayName(val)
				break
			}
		}
		if name != "" {
			return name, lang
		}
	}
	if _, err := os.Stat(filepath.Join(dirPath, "requirements.txt")); err == nil {
		lang = "python"
	}

	// Fallback para o nome do diretório formatado
	base := filepath.Base(dirPath)
	if base != "" && base != "." && base != "/" {
		name = FormatProjectDisplayName(base)
	}

	return name, lang
}

// BrowseEntry representa um diretório ou arquivo no navegador de pastas.
type BrowseEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsDir        bool   `json:"is_dir"`
	IsGit        bool   `json:"is_git"`
	HasCode      bool   `json:"has_code"`
	IsProject    bool   `json:"is_project"`
	IsRegistered bool   `json:"is_registered"`
	DetectedName string `json:"detected_name,omitempty"`
	DetectedLang string `json:"detected_lang,omitempty"`
	ItemCount    int    `json:"item_count,omitempty"`
}

// BrowseResponse encapsula a listagem de um caminho no sistema de arquivos do host.
type BrowseResponse struct {
	CurrentPath string        `json:"current_path"`
	ParentPath  string        `json:"parent_path"`
	RootPath    string        `json:"root_path,omitempty"`
	RootLabel   string        `json:"root_label,omitempty"`
	Entries     []BrowseEntry `json:"entries"`
}

// BrowsePath navega pelos diretórios montados no container (/mnt/host, /mnt/repos) ou no host local.
func (s *ProjectService) BrowsePath(targetPath string) (*BrowseResponse, error) {
	homeDir, _ := os.UserHomeDir()
	pwd, _ := os.Getwd()

	// Detecta se estamos rodando dentro do Docker
	isDocker := false
	if _, err := os.Stat("/.dockerenv"); err == nil {
		isDocker = true
	} else if _, err := os.Stat("/mnt/host"); err == nil {
		isDocker = true
	}

	var rootPath, rootLabel string

	if isDocker {
		// Restringe ao container
		if targetPath == "" {
			if _, err := os.Stat("/mnt/host"); err == nil {
				targetPath = "/mnt/host"
			} else {
				targetPath = "/mnt/repos"
			}
		}
		targetPath = filepath.Clean(targetPath)

		// Restringe o acesso aos volumes montados dentro do Docker
		isAllowed := strings.HasPrefix(targetPath, "/mnt/host") || strings.HasPrefix(targetPath, "/mnt/repos")
		if !isAllowed {
			if _, err := os.Stat("/mnt/host"); err == nil {
				targetPath = "/mnt/host"
			} else {
				targetPath = "/mnt/repos"
			}
		}

		if strings.HasPrefix(targetPath, "/mnt/repos") {
			rootPath = "/mnt/repos"
			rootLabel = "Repos (/mnt/repos)"
		} else {
			rootPath = "/mnt/host"
			rootLabel = "Host (/mnt/host)"
		}
	} else {
		// Ambiente Nativo Local (fora do Docker): Raiz padrão é sempre a Home do usuário
		if targetPath == "" {
			if homeDir != "" {
				targetPath = homeDir
			} else if pwd != "" {
				targetPath = pwd
			} else {
				targetPath = "/"
			}
		} else {
			// Expande atalho de home ~ se fornecido
			if strings.HasPrefix(targetPath, "~") && homeDir != "" {
				targetPath = filepath.Join(homeDir, strings.TrimPrefix(targetPath, "~"))
			}
		}
		targetPath = filepath.Clean(targetPath)

		if homeDir != "" && strings.HasPrefix(targetPath, homeDir) {
			rootPath = homeDir
			rootLabel = fmt.Sprintf("Home (%s)", homeDir)
		} else {
			rootPath = "/"
			rootLabel = "Raiz (/)"
		}
	}

	info, err := os.Stat(targetPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("caminho não encontrado ou não é um diretório: %s", targetPath)
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler diretório: %w", err)
	}

	// Carrega caminhos de projetos já cadastrados
	registeredPaths := make(map[string]bool)
	if allProjects, err := s.projectRepo.ListAll(); err == nil {
		for _, p := range allProjects {
			if p != nil {
				registeredPaths[filepath.Clean(p.Path)] = true
			}
		}
	}

	results := make([]BrowseEntry, 0)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && name != ".git" {
			continue
		}

		fullPath := filepath.Join(targetPath, name)
		isDir := entry.IsDir()
		isGit := false
		hasCode := false
		detectedName := ""
		detectedLang := ""

		if isDir {
			if _, err := os.Stat(filepath.Join(fullPath, ".git")); err == nil {
				isGit = true
			}
			if subEntries, err := os.ReadDir(fullPath); err == nil {
				for _, sub := range subEntries {
					subExt := strings.ToLower(filepath.Ext(sub.Name()))
					if subExt == ".go" || subExt == ".py" || subExt == ".ts" || subExt == ".js" || subExt == ".tsx" || subExt == ".jsx" || subExt == ".java" || subExt == ".php" {
						hasCode = true
						break
					}
				}
			}

			detectedName, detectedLang = DetectProjectManifestInfo(fullPath)
			// Um diretório é considerado projeto se tem manifesto identificado OU repositório .git
			isProject := detectedName != "" || isGit
			isRegistered := registeredPaths[filepath.Clean(fullPath)]

			results = append(results, BrowseEntry{
				Name:         name,
				Path:         fullPath,
				IsDir:        true,
				IsGit:        isGit,
				HasCode:      hasCode,
				IsProject:    isProject,
				IsRegistered: isRegistered,
				DetectedName: detectedName,
				DetectedLang: detectedLang,
			})
		}
	}

	parentPath := ""
	if isDocker {
		if targetPath != "/mnt/host" && targetPath != "/mnt/repos" {
			parent := filepath.Dir(targetPath)
			if strings.HasPrefix(parent, "/mnt/host") || strings.HasPrefix(parent, "/mnt/repos") {
				parentPath = parent
			}
		}
	} else {
		parent := filepath.Dir(targetPath)
		if parent != targetPath && parent != "" {
			parentPath = parent
		}
	}

	return &BrowseResponse{
		CurrentPath: targetPath,
		ParentPath:  parentPath,
		RootPath:    rootPath,
		RootLabel:   rootLabel,
		Entries:     results,
	}, nil
}
