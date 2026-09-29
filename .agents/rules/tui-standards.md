# Diretrizes para Interfaces de Terminal (TUI) e CLI

Ao desenvolver ou modificar comandos CLI e interfaces interativas (TUI com Huh? / Bubble Tea):

1. **Uso Obrigatório de Alternate Screen Buffer (AltScreen)**:
   - Todo fluxo interativo (menus, formulários, wizards, diálogos de confirmação) DEVE ser encapsulado em `ui.EnterAltScreen()` e `defer ui.ExitAltScreen()`.
   - Ao cancelar ou sair via `Esc` ou "Cancelar", o terminal deve retornar ao prompt original sem deixar nenhum rastro visual, formulário não preenchido ou resíduo no histórico/scrollback.

2. **Tratamento Silencioso de Abort/Cancelamento**:
   - `huh.ErrUserAborted`, cancelamento por `Esc` ou seleção explícita de "Cancelar" são saídas normais de fluxo.
   - NUNCA trate cancelamentos como falha ou imprima `[ERRO] user aborted`. Termine com código de saída 0 limpo.

3. **Alinhamento Rígido na Coluna 0**:
   - Todas as saídas de comandos standalone e formulários devem iniciar na coluna 0.
   - Evite prefixos que gerem desalinhamento artificial (como setas `> ` no meio de listas).
   - Utilize formatação padronizada chave/valor (`ui.KeyValue`) para garantir que os valores fiquem alinhados verticalmente.

4. **Legendas e Keymaps Explícitos**:
   - Mapeie a tecla `Esc` em todos os campos (`Select`, `MultiSelect`, `Input`, `Confirm`).
   - Garanta que a legenda de ajuda sempre informe `esc sair` ou `esc cancelar` para que o usuário saiba como desistir da operação.
