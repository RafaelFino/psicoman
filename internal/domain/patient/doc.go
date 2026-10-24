// Package patient é o bounded context do paciente: cadastro, origem/indicação e
// estado de aprovação (pendente|aprovado|rejeitado). Autoridade sobre as
// invariantes do paciente (email/CPF únicos, telefone canônico, gate de
// aprovação). Ver MER canônico em .kiro/specs/pocketbase-core/design.md (D4b).
package patient
