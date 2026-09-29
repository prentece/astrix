# Padrão de Especificação OpenSpec (Spec-Driven Development)

Ao criar ou atualizar especificações no padrão **OpenSpec**:

1. **Estrutura de Pastas**:
   - As especificações devem ser salvas em `openspec/specs/<capability>/spec.md` (ou `specs/<capability>/spec.md`).

2. **Formato Obrigatório do Documento**:
   - `# <Capability> Specification`
   - `## Purpose`: Resumo do propósito e razão de existir da capacidade.
   - `## Requirements`: Lista numerada ou categorizada de requisitos.

3. **Definição de Requisitos**:
   - Cada requisito deve usar o formato `### Requirement: <Nome do Requisito>`.
   - Use palavras-chave normativas RFC 2119 (**SHALL**, **MUST**, **SHOULD**).

4. **Cenários Comportamentais (BDD)**:
   - Todo requisito DEVE conter pelo menos um cenário `#### Scenario: <Nome do Cenário>`.
   - A estrutura do cenário deve seguir rigorosamente:
     - `- **GIVEN** <estado inicial ou pré-condição>`
     - `- **WHEN** <evento ou gatilho da ação>`
     - `- **THEN** <resultado esperado observável>`
     - `- **AND** <condição ou consequência adicional>`
