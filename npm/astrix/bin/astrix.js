#!/usr/bin/env node

const { spawn } = require('child_process');
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

// O Ctrl+C é entregue a todo o process group (wrapper + binário). Se o wrapper
// morrer, o shell retoma o terminal e o binário Go passa a ser background,
// quebrando o modo raw (EIO). Por isso o wrapper ignora os sinais de
// interrupção e deixa o binário tratá-los.
process.on('SIGINT', () => {});
process.on('SIGTERM', () => {});

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: 'inherit',
  env: process.env,
});

child.on('error', (err) => {
  console.error(`[astrix] Falha ao executar o binário:`, err);
  process.exit(1);
});

child.on('exit', (code, signal) => {
  if (signal) {
    process.removeAllListeners(signal);
    process.kill(process.pid, signal);
  } else {
    process.exit(code !== null ? code : 0);
  }
});
