// Package gemini integra o Psicoman à IA do Google (Gemini): resumo clínico +
// tagueamento, com índice invertido de tags por paciente/ano.
//
// Segue a mesma disciplina de Ports & Adapters: o domínio conhece só a interface
// de resumo/tags; a implementação concreta vive aqui, atrás de interface + fake.
// Opt-in por instância (default OFF), disparo explícito pelo terapeuta, resultado
// revisável antes de consolidar, auditoria ai.process SEM conteúdo clínico e
// minimização dos dados enviados (R10, psicoman-seguranca-lgpd.md).
package gemini
