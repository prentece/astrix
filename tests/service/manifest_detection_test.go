package service_test

import (
	"astrix/internal/service"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatProjectDisplayName(t *testing.T) {
	assert.Equal(t, "Fibra Backend Core", service.FormatProjectDisplayName("fibra-backend-core"))
	assert.Equal(t, "Astrix Engine", service.FormatProjectDisplayName("astrix-engine"))
	assert.Equal(t, "Customer Care", service.FormatProjectDisplayName("customer_care"))
	assert.Equal(t, "My Tool", service.FormatProjectDisplayName("@scope/my-tool"))
	assert.Equal(t, "Repo Name", service.FormatProjectDisplayName("github.com/user/repo-name"))
}

func TestDetectProjectManifestInfo(t *testing.T) {
	// 1. package.json
	t.Run("package.json", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "my-cool-frontend"}`), 0644)
		name, lang := service.DetectProjectManifestInfo(dir)
		assert.Equal(t, "My Cool Frontend", name)
		assert.Equal(t, "javascript", lang)
	})

	// 2. package.json with tsconfig.json -> TypeScript
	t.Run("package.json with tsconfig", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "@company/api-gateway"}`), 0644)
		_ = os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{}`), 0644)
		name, lang := service.DetectProjectManifestInfo(dir)
		assert.Equal(t, "Api Gateway", name)
		assert.Equal(t, "typescript", lang)
	})

	// 3. go.mod
	t.Run("go.mod", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/user/order-service\n\ngo 1.23\n"), 0644)
		name, lang := service.DetectProjectManifestInfo(dir)
		assert.Equal(t, "Order Service", name)
		assert.Equal(t, "go", lang)
	})

	// 4. Cargo.toml
	t.Run("Cargo.toml", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"fast-parser\"\nversion = \"0.1.0\"\n"), 0644)
		name, lang := service.DetectProjectManifestInfo(dir)
		assert.Equal(t, "Fast Parser", name)
		assert.Equal(t, "rust", lang)
	})

	// 5. pom.xml
	t.Run("pom.xml", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project><artifactId>payment-processor</artifactId></project>"), 0644)
		name, lang := service.DetectProjectManifestInfo(dir)
		assert.Equal(t, "Payment Processor", name)
		assert.Equal(t, "java", lang)
	})

	// 6. build.gradle
	t.Run("build.gradle", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "settings.gradle"), []byte("rootProject.name = 'notification-service'\n"), 0644)
		name, lang := service.DetectProjectManifestInfo(dir)
		assert.Equal(t, "Notification Service", name)
		assert.Equal(t, "java", lang)
	})

	// 7. composer.json
	t.Run("composer.json", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"name": "vendor/billing-app"}`), 0644)
		name, lang := service.DetectProjectManifestInfo(dir)
		assert.Equal(t, "Billing App", name)
		assert.Equal(t, "php", lang)
	})

	// 8. dubbo.properties
	t.Run("dubbo.properties", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "dubbo.properties"), []byte("dubbo.application.name=user-rpc-service\n"), 0644)
		name, lang := service.DetectProjectManifestInfo(dir)
		assert.Equal(t, "User Rpc Service", name)
		assert.Equal(t, "java", lang)
	})

	// 9. pyproject.toml
	t.Run("pyproject.toml", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"ai-agent-engine\"\n"), 0644)
		name, lang := service.DetectProjectManifestInfo(dir)
		assert.Equal(t, "Ai Agent Engine", name)
		assert.Equal(t, "python", lang)
	})
}
