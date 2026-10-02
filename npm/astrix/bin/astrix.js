#!/usr/bin/env node

const { spawnSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const PLATFORMS = {
  'darwin-arm64': '@prentece/astrix-darwin-arm64',
  'darwin-x64': '@prentece/astrix-darwin-x64',
  'linux-arm64': '@prentece/astrix-linux-arm64',
  'linux-x64': '@prentece/astrix-linux-x64',
  'win32-x64': '@prentece/astrix-win32-x64',
};

const platformKey = `${process.platform}-${process.arch}`;
const packageName = PLATFORMS[platformKey];

if (!packageName) {
  console.error(`[astrix] Sistema operacional ou arquitetura não suportada: ${platformKey}`);
  console.error(`Plataformas suportadas: ${Object.keys(PLATFORMS).join(', ')}`);
  process.exit(1);
}

const exeName = process.platform === 'win32' ? 'astrix.exe' : 'astrix';

function resolveBinary() {
  // 1. Tentar resolver a partir do pacote de plataforma instalado via npm
  try {
    const pkgPath = require.resolve(`${packageName}/package.json`);
    const binPath = path.join(path.dirname(pkgPath), 'bin', exeName);
    if (fs.existsSync(binPath)) {
      return binPath;
    }
  } catch (e) {
    // Pacote não encontrado no require.resolve padrão
  }

  // 2. Fallback para desenvolvimento local ou monorepo (npm link / testes locais)
  const localPlatformPath = path.resolve(__dirname, '..', '..', 'platforms', platformKey, 'bin', exeName);
  if (fs.existsSync(localPlatformPath)) {
    return localPlatformPath;
  }

  // 3. Fallback para a pasta bin/ padrão do repositório
  const repoBinPath = path.resolve(__dirname, '..', '..', '..', 'bin', exeName);
  if (fs.existsSync(repoBinPath)) {
    return repoBinPath;
  }

  return null;
}

const binaryPath = resolveBinary();

if (!binaryPath) {
  console.error(`[astrix] Binário executável não encontrado para a plataforma ${platformKey}.`);
  console.error(`Certifique-se de que as dependências opcionais (${packageName}) foram instaladas corretamente.`);
  console.error(`Caso esteja em ambiente offline, tente reinstalar com: npm install -g @prentece/astrix`);
  process.exit(1);
}

// Garante permissão de execução em ambientes Unix
if (process.platform !== 'win32') {
  try {
    fs.chmodSync(binaryPath, 0o755);
  } catch (e) {
    // Ignora se não for possível alterar permissões
  }
}

const result = spawnSync(binaryPath, process.argv.slice(2), {
  stdio: 'inherit',
  env: process.env,
});

if (result.error) {
  console.error(`[astrix] Falha ao executar o binário:`, result.error);
  process.exit(1);
}

if (result.signal) {
  process.kill(process.pid, result.signal);
} else {
  process.exit(result.status !== null ? result.status : 0);
}
