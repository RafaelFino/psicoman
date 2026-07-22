---
inclusion: auto
---

# Documentação — Padrões e Convenções

## Diagramas

- **Sempre usar Mermaid** para diagramas em Markdown (nunca ASCII art)
- Tipos de diagramas disponíveis:
  - `flowchart TB/LR` — arquitetura, fluxos de dados
  - `erDiagram` — modelo de dados (MER/ERD)
  - `sequenceDiagram` — interações entre componentes
  - `gantt` — roadmap/timeline
- Mermaid renderiza nativamente no GitHub, GitLab, VS Code (com extensão)
- Usar cores nos subgraphs para diferenciar domínios:
  - Azul (`fill:#e0f2fe`) para cloud/infra
  - Verde (`fill:#f0fdf4`) para ambiente local
  - Lilás (`fill:#eef2ff`) para ações do psicólogo
  - Laranja (`fill:#fff7ed`) para ações do paciente

## Estrutura de documentação

```
docs/
├── release-notes.md   — O que já está implementado (changelog por versão)
├── next-steps.md      — Roadmap: o que falta fazer, com prioridades e esforço
├── testing.md         — Guia de testes manuais e automatizados, seed de dados
```

## Convenções de escrita

- Documentação em **português brasileiro**
- Títulos em markdown H2 (`##`) para seções principais
- Tabelas para listas de features, variáveis, ou comparações
- Blocos de código com linguagem especificada (```bash, ```go, ```json, ```mermaid)
- Links relativos entre documentos (`[texto](docs/arquivo.md)`)

## README.md

O README deve conter:
- Descrição curta do projeto (1 linha)
- "O que é" com casos de uso em linguagem simples
- Diagrama de arquitetura (Mermaid flowchart)
- Modelo de dados (Mermaid erDiagram)
- Diagrama de casos de uso (Mermaid flowchart)
- Quick start (Docker + local)
- Seção de dados de teste
- Variáveis de ambiente
- Estrutura do projeto (tree)
- Links para docs/

## Steering files (.kiro/steering/)

- Convenções técnicas que o agente deve seguir ao gerar código
- Inclusão automática (`inclusion: auto`) para regras globais
- Inclusão por file match (`inclusion: fileMatch`) para regras específicas
- Cada decisão arquitetural relevante deve ser documentada em steering
