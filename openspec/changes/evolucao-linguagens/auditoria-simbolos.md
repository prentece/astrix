# Auditoria de símbolos: relatório (Fase 2, passos 2.1 a 2.3)

Gerado em 2026-10-16 por `tests/indexer/languages/ast_audit_test.go` (`ASTRIX_AUDIT=1`). Fixtures em `tests/indexer/languages/testdata/audit/`.
Corpus real: Go = este repositório (94 arquivos); TS = `code-auditor` (66); JS = `prentece.github.io` (8); Python = 400 arquivos do SDK local; Java = 43 arquivos de um projeto local; **PHP: sem corpus real, só a fixture**.

## Achados que NÃO são só "falta uma query"

| # | Achado | Evidência |
|---|---|---|
| A | **Capturas descartadas em silêncio pelo engine.** O `switch` de `ExtractASTDataAndDigest` (`engine.go:965-1016`) só conhece `function/method/struct/interface/class/type/const/var/variable/enum/import`. As queries já capturam `@record`, `@constructor` (Java) e `@mod` (PHP namespace), que são ignorados. | Java: 45/45 `constructor_declaration` e o `record_declaration` da fixture caem em `ignored`. PHP: `namespace_definition` em `ignored`. |
| B | **Python indexa variáveis locais como `constant`.** A query `(expression_statement (assignment left: identifier))` casa em qualquer profundidade. | Teste mínimo: `local_a` e `local_b` (dentro de `def`/`for`) viram `constant`. No corpus, 11.804 de 16.766 `assignment` viram símbolo. É ruído, não lacuna. |
| C | `VariablesQuery` do TypeScript **não compila** (parêntese sobrando, `typescript.go:98`). Nenhum código chama `VariablesQuery()`, então hoje é código morto. | Auditoria reporta `invalid syntax at line 19`. |
| D | Observações a confirmar: `parent` só é preenchido em Python (Go/Java/TS/PHP vêm com `parent=""`); `trait` do PHP vira `class` apesar de existir `KindTrait`; `struct`/`interface` do Go aparecem como `kind=type`. | Dump dos símbolos extraídos das fixtures. |
| E | A fixture `sample.php` gera `[TREE-SITTER WARN]` (erro de sintaxe, recuperação parcial). A gramática PHP da dependência pode não cobrir toda a sintaxe PHP 8. | Warning do engine ao indexar a fixture. |

## Lacunas por linguagem (nós declarativos sem símbolo)

Impacto = ocorrências no corpus real (ou "fixture" quando não há corpus).

| Linguagem | Lacuna | Impacto |
|---|---|---|
| **Go** | campos de struct (`field_declaration`) | 365 |
| | métodos de interface (`method_elem`) | 57 |
| | type alias (`type_alias`, `type Alias = X`) | fixture |
| | `var`/`func` anônimos: `var x = func(){}` já é capturado como `variable` | ok |
| **Java** | construtores e `record` (achado A) | 45 / fixture |
| | campos (`field_declaration`) | 139 |
| | constantes de enum (`enum_constant`) | 6 |
| | `@interface` (`annotation_type_declaration`) e seus elementos | 1 / 4 |
| **PHP** | `namespace` (achado A) | fixture |
| | propriedades (`property_declaration`/`property_element`) | fixture |
| | `use Trait;` dentro de classe (`use_declaration`) | fixture |
| **Python** | nenhuma lacuna estrutural (classes, funções, decorators, `async`, aninhadas: ok). Problema é o ruído do achado B. | n/a |
| **TypeScript** | `abstract class` (`abstract_class_declaration`) | fixture |
| | propriedades de classe (`public_field_definition`) | 143 |
| | membros de interface (`property_signature` / `method_signature`) | 236 / 6 |
| | `namespace`/`module` e `declare` (`internal_module`, `ambient_declaration`, `function_signature`) | fixture |
| | `export const X = <valor não-função>` (`lexical_declaration`) | 444 |
| | membros de enum (`enum_assignment`) | fixture |
| **JavaScript** | campos de classe (`field_definition`) | fixture |
| | `const X = <valor não-função>` | 99 |

Falsos positivos da auditoria (não são lacunas): `import_*`/`package_declaration` (tratados pelo grafo de dependências), `parameter_declaration`, `short_var_declaration`, `local_variable_declaration`, `*_case`, `literal_element`/`keyed_element`, `with_item`, `spread_element`, `const_declaration` do PHP (filho `const_element` é capturado).

---

# Saída bruta da ferramenta

# Auditoria de cobertura de símbolos

`covered` = o engine gera símbolo (captura reconhecida). `ignored` = há captura na query, mas com nome que o engine descarta. `uncovered` = nenhuma captura.

## go


### fixtures (1 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `parameter_declaration` | 6 | 0 | 0 | 6 | sample.go:17, sample.go:38 |
| `field_declaration` | 5 | 0 | 0 | 5 | sample.go:29, sample.go:30 |
| `literal_element` | 4 | 0 | 0 | 4 | sample.go:39, sample.go:39 |
| `import_spec` | 2 | 0 | 0 | 2 | sample.go:4, sample.go:5 |
| `keyed_element` | 2 | 0 | 0 | 2 | sample.go:39, sample.go:39 |
| `method_elem` | 2 | 0 | 0 | 2 | sample.go:24, sample.go:25 |
| `type_parameter_declaration` | 2 | 0 | 0 | 2 | sample.go:46, sample.go:50 |
| `import_declaration` | 1 | 0 | 0 | 1 | sample.go:3 |
| `type_alias` | 1 | 0 | 0 | 1 | sample.go:19 |
| `type_declaration` | 6 | 5 | 0 | 1 | sample.go:19 |
| `type_elem` | 1 | 0 | 0 | 1 | sample.go:54 |
| `const_declaration` | 2 | 2 | 0 | 0 |  |
| `const_spec` | 3 | 3 | 0 | 0 |  |
| `function_declaration` | 2 | 2 | 0 | 0 |  |
| `method_declaration` | 2 | 2 | 0 | 0 |  |
| `type_spec` | 5 | 5 | 0 | 0 |  |
| `var_declaration` | 2 | 2 | 0 | 0 |  |
| `var_spec` | 2 | 2 | 0 | 0 |  |

### corpus real (94 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `literal_element` | 3856 | 0 | 0 | 3856 | cmd_skills.go:64, cmd_status.go:88 |
| `short_var_declaration` | 2599 | 0 | 0 | 2599 | main.go:9, cmd_clean.go:20 |
| `keyed_element` | 1751 | 0 | 0 | 1751 | cmd_status.go:88, cmd_status.go:89 |
| `parameter_declaration` | 1500 | 0 | 0 | 1500 | cmd_clean.go:14, cmd_clean.go:19 |
| `import_spec` | 571 | 0 | 0 | 571 | main.go:4, main.go:5 |
| `field_declaration` | 365 | 0 | 0 | 365 | cmd_status.go:17, cmd_status.go:18 |
| `expression_case` | 94 | 0 | 0 | 94 | cmd_dashboard.go:79, cmd_dashboard.go:81 |
| `import_declaration` | 92 | 0 | 0 | 92 | main.go:3, cmd_clean.go:3 |
| `method_elem` | 57 | 0 | 0 | 57 | project_service.go:18, project_service.go:19 |
| `var_spec` | 272 | 252 | 0 | 20 | styles.go:13, styles.go:14 |
| `variadic_parameter_declaration` | 14 | 0 | 0 | 14 | cmd_clean.go:19, cmd_list.go:13 |
| `communication_case` | 11 | 0 | 0 | 11 | cmd_serve.go:118, cmd_serve.go:120 |
| `default_case` | 10 | 0 | 0 | 10 | cmd_mcp.go:38, cmd_mcp.go:60 |
| `type_elem` | 8 | 0 | 0 | 8 | cmd_dashboard.go:43, cmd_dashboard.go:136 |
| `var_declaration` | 256 | 252 | 0 | 4 | styles.go:12, styles.go:27 |
| `type_case` | 3 | 0 | 0 | 3 | params.go:53, params.go:60 |
| `const_declaration` | 36 | 36 | 0 | 0 |  |
| `const_spec` | 58 | 58 | 0 | 0 |  |
| `function_declaration` | 352 | 352 | 0 | 0 |  |
| `method_declaration` | 206 | 206 | 0 | 0 |  |
| `type_declaration` | 100 | 100 | 0 | 0 |  |
| `type_spec` | 100 | 100 | 0 | 0 |  |

## java

- capturas da SymbolsQuery **descartadas pelo engine**: `@constructor.def`, `@constructor.name`, `@record.def`, `@record.name`

### fixtures (1 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `field_declaration` | 3 | 0 | 0 | 3 | Sample.java:33, Sample.java:34 |
| `variable_declarator` | 3 | 0 | 0 | 3 | Sample.java:33, Sample.java:34 |
| `enum_constant` | 2 | 0 | 0 | 2 | Sample.java:10, Sample.java:10 |
| `annotation_type_declaration` | 1 | 0 | 0 | 1 | Sample.java:5 |
| `annotation_type_element_declaration` | 1 | 0 | 0 | 1 | Sample.java:6 |
| `compact_constructor_declaration` | 1 | 0 | 0 | 1 | Sample.java:18 |
| `constructor_declaration` | 1 | 0 | 1 | 0 |  |
| `import_declaration` | 1 | 0 | 0 | 1 | Sample.java:3 |
| `package_declaration` | 1 | 0 | 0 | 1 | Sample.java:1 |
| `record_declaration` | 1 | 0 | 1 | 0 |  |
| `class_declaration` | 2 | 2 | 0 | 0 |  |
| `enum_declaration` | 1 | 1 | 0 | 0 |  |
| `interface_declaration` | 1 | 1 | 0 | 0 |  |
| `method_declaration` | 6 | 6 | 0 | 0 |  |

### corpus real (43 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `import_declaration` | 223 | 0 | 0 | 223 | Application.java:3, Application.java:4 |
| `variable_declarator` | 181 | 0 | 0 | 181 | ContractService.java:15, ContractService.java:16 |
| `field_declaration` | 139 | 0 | 0 | 139 | ContractService.java:15, ContractService.java:16 |
| `constructor_declaration` | 45 | 0 | 45 | 0 |  |
| `package_declaration` | 43 | 0 | 0 | 43 | Application.java:1, ContractService.java:1 |
| `local_variable_declaration` | 30 | 0 | 0 | 30 | ContractService.java:29, ContractService.java:30 |
| `enum_constant` | 6 | 0 | 0 | 6 | Status.java:5, Status.java:6 |
| `annotation_type_element_declaration` | 4 | 0 | 0 | 4 | ValidateString.java:13, ValidateString.java:14 |
| `annotation_type_declaration` | 1 | 0 | 0 | 1 | ValidateString.java:7 |
| `class_declaration` | 40 | 40 | 0 | 0 |  |
| `enum_declaration` | 2 | 2 | 0 | 0 |  |
| `interface_declaration` | 3 | 3 | 0 | 0 |  |
| `method_declaration` | 257 | 257 | 0 | 0 |  |

## php

- capturas da SymbolsQuery **descartadas pelo engine**: `@mod.def`, `@mod.name`

### fixtures (1 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `const_declaration` | 4 | 0 | 0 | 4 | sample.php:7, sample.php:27 |
| `property_declaration` | 2 | 0 | 0 | 2 | sample.php:42, sample.php:43 |
| `property_element` | 2 | 0 | 0 | 2 | sample.php:42, sample.php:43 |
| `namespace_definition` | 1 | 0 | 1 | 0 |  |
| `namespace_use_declaration` | 1 | 0 | 0 | 1 | sample.php:5 |
| `use_declaration` | 1 | 0 | 0 | 1 | sample.php:39 |
| `class_declaration` | 1 | 1 | 0 | 0 |  |
| `const_element` | 4 | 4 | 0 | 0 |  |
| `enum_case` | 2 | 2 | 0 | 0 |  |
| `enum_declaration` | 1 | 1 | 0 | 0 |  |
| `function_definition` | 1 | 1 | 0 | 0 |  |
| `interface_declaration` | 1 | 1 | 0 | 0 |  |
| `method_declaration` | 5 | 5 | 0 | 0 |  |
| `trait_declaration` | 1 | 1 | 0 | 0 |  |

_Sem corpus real configurado para esta linguagem._

## python


### fixtures (1 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `assignment` | 7 | 6 | 0 | 1 | sample.py:26 |
| `class_definition` | 3 | 3 | 0 | 0 |  |
| `decorated_definition` | 4 | 4 | 0 | 0 |  |
| `function_definition` | 7 | 7 | 0 | 0 |  |

### corpus real (400 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `assignment` | 16766 | 11804 | 0 | 4962 | __init__.py:33, __init__.py:49 |
| `with_item` | 283 | 0 | 0 | 283 | __init__.py:56, __init__.py:128 |
| `class_definition` | 1405 | 1405 | 0 | 0 |  |
| `decorated_definition` | 358 | 358 | 0 | 0 |  |
| `function_definition` | 6359 | 6359 | 0 | 0 |  |

## typescript

- **query `variables` NÃO compila**: invalid syntax at line 19 column 0
)
^

### fixtures (1 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `public_field_definition` | 5 | 0 | 0 | 5 | sample.ts:21, sample.ts:22 |
| `ambient_declaration` | 2 | 0 | 0 | 2 | sample.ts:10, sample.ts:16 |
| `lexical_declaration` | 4 | 2 | 0 | 2 | sample.ts:17, sample.ts:53 |
| `property_signature` | 2 | 0 | 0 | 2 | sample.ts:41, sample.ts:42 |
| `variable_declarator` | 4 | 2 | 0 | 2 | sample.ts:17, sample.ts:53 |
| `abstract_class_declaration` | 1 | 0 | 0 | 1 | sample.ts:20 |
| `abstract_method_signature` | 1 | 0 | 0 | 1 | sample.ts:30 |
| `enum_assignment` | 1 | 0 | 0 | 1 | sample.ts:7 |
| `function_signature` | 1 | 0 | 0 | 1 | sample.ts:10 |
| `method_signature` | 1 | 0 | 0 | 1 | sample.ts:40 |
| `class_declaration` | 1 | 1 | 0 | 0 |  |
| `enum_declaration` | 2 | 2 | 0 | 0 |  |
| `function_declaration` | 1 | 1 | 0 | 0 |  |
| `interface_declaration` | 1 | 1 | 0 | 0 |  |
| `method_definition` | 2 | 2 | 0 | 0 |  |
| `type_alias_declaration` | 1 | 1 | 0 | 0 |  |

### corpus real (66 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `lexical_declaration` | 445 | 1 | 0 | 444 | crypto.service.spec.ts:6, crypto.service.spec.ts:7 |
| `variable_declarator` | 445 | 1 | 0 | 444 | crypto.service.spec.ts:6, crypto.service.spec.ts:7 |
| `property_signature` | 236 | 0 | 0 | 236 | crypto.service.ts:6, crypto.service.ts:7 |
| `public_field_definition` | 143 | 0 | 0 | 143 | crypto.service.ts:13, crypto.service.ts:14 |
| `switch_case` | 11 | 0 | 0 | 11 | git-provider.service.ts:114, git-provider.service.ts:116 |
| `spread_element` | 7 | 0 | 0 | 7 | jobs.service.ts:190, jobs.service.ts:206 |
| `method_signature` | 6 | 0 | 0 | 6 | logger.contract.ts:2, review.contracts.ts:13 |
| `class_declaration` | 48 | 48 | 0 | 0 |  |
| `function_declaration` | 14 | 14 | 0 | 0 |  |
| `interface_declaration` | 30 | 30 | 0 | 0 |  |
| `method_definition` | 114 | 114 | 0 | 0 |  |
| `type_alias_declaration` | 11 | 11 | 0 | 0 |  |

## javascript


### fixtures (1 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `field_definition` | 4 | 0 | 0 | 4 | sample.js:10, sample.js:11 |
| `lexical_declaration` | 3 | 2 | 0 | 1 | sample.js:5 |
| `variable_declarator` | 3 | 2 | 0 | 1 | sample.js:5 |
| `class_declaration` | 1 | 1 | 0 | 0 |  |
| `function_declaration` | 1 | 1 | 0 | 0 |  |
| `method_definition` | 3 | 3 | 0 | 0 |  |

### corpus real (8 arquivos)

| tipo de nó | total | covered | ignored | uncovered | exemplos |
|---|---:|---:|---:|---:|---|
| `variable_declarator` | 111 | 10 | 0 | 101 | cipher.js:12, cipher.js:15 |
| `lexical_declaration` | 109 | 10 | 0 | 99 | cipher.js:12, cipher.js:15 |
| `spread_element` | 1 | 0 | 0 | 1 | commands.js:8 |
| `function_declaration` | 26 | 26 | 0 | 0 |  |
| `method_definition` | 14 | 14 | 0 | 0 |  |

