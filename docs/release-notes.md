# Release Notes — Psicoman v0.9 (Pre-release)

Data: 2026-07-22

## Visão geral

Sistema de gestão para consultórios de psicologia. Monolito Go auto-contido servindo interface HTML moderna com zero dependências externas em runtime.

## Features implementadas

### Gestão de pacientes
- CRUD completo (nome, email, telefone, data de nascimento)
- Página 360° com calendário mensal, evoluções, documentos, contratos, financeiro
- Calendário mensal interativo com navegação entre meses (htmx)
- Clicar em consulta → modal de edição; clicar em dia vazio → criar consulta

### Agendamentos
- Online e presencial com detecção automática de conflitos
- Regras configuráveis: mínimo de horas para cancelar/reagendar, máximo de reagendamentos/mês
- Integração Google Calendar + Meet (criação automática de eventos e links)
- Visão diária e semanal no dashboard

### Evoluções de sessão (Session Notes)
- Registro de evolução clínica por atendimento
- Notas privadas (não compartilhadas com paciente)
- Métricas de tempo: minutos com paciente, análise, administrativo
- Relatório mensal consolidado de horas

### Anamnese estruturada
- Templates personalizáveis por faixa etária (adulto, criança, universal)
- Campos dinâmicos: texto, textarea, select, checkbox, date, number, escala
- Paciente preenche online; psicólogo visualiza respostas
- Compatibilidade com anamnese em texto livre (legado)

### Contrato terapêutico
- Templates HTML com placeholders (nome, email, telefone, data)
- Envio para paciente com aceite digital (IP + user-agent + timestamp)
- Status: pendente, assinado, expirado, revogado
- Armazenamento no GED

### Supervisões
- Registro de supervisores (nome, email, especialidade, CRP)
- Sessões de supervisão com data, duração, notas, tópicos, custo
- Métricas de horas de supervisão por mês

### Espaços e consultórios
- Gestão de salas: fixas, alugadas, temporárias
- Custos por uso e mensais
- Reservas vinculadas a atendimentos
- Controle de disponibilidade

### Financeiro
- Pagamentos por paciente com status (pendente/recebido)
- Custos operacionais por mês/categoria
- Relatório mensal com saldo (receita - custos)
- Métricas na página do paciente

### GED (Gestão Eletrônica de Documentos)
- Upload/download de documentos por paciente
- Tipos: laudo, nota fiscal, relatório, outro
- Organizado por tenant/paciente no filesystem
- Registro de quem fez upload (psicólogo/paciente/sistema)

### Interface e UX
- Design system CSS com tema claro e escuro (toggle + auto via prefers-color-scheme)
- Layout responsivo mobile-first (sidebar colapsável, touch targets 44px)
- Agenda semanal no dashboard com grid de 7 dias
- Métricas com barras de progresso CSS

### Segurança
- Auth dupla: Pangolin (psicólogo) + Google OAuth/JWT (paciente)
- Security headers (X-Frame-Options, X-Content-Type-Options, Referrer-Policy)
- DEV_MODE com rotas de debug e login sem Google

### Infraestrutura de testes
- Script seed: `scripts/seed-test-data.sh` (pacientes TEST + consultas)
- Endpoint cleanup: `DELETE /api/dev/test-data` (remove dados de teste)
- Testes automatizados Go (httptest) com DB isolado por teste
- Prefixo "TEST " em nomes e @test.com em emails para identificação segura

### Arquitetura
- Monolito Go com SQLite (pure-Go, zero CGO)
- Interface: Go html/template + htmx + Alpine.js (embutido no binário)
- Build: único `go build`, zero Node.js/npm
- Docker: imagem Alpine ~20MB
- Logs: zerolog + lumberjack (JSON, rotação diária)
- Multi-tenant via header (DB separado por tenant)
