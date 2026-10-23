package pb

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// Este arquivo concentra os helpers transversais do adapter: tradução de erros
// do PocketBase para os erros sentinela de internal/app, serialização de datas
// (TextField ISO-8601, nunca AutodateField — psicoman-sqlite.md), e filtros de
// soft-delete. Cada repositório por porta vive no arquivo do seu bounded context.

// notDeletedFilter é o filtro de listagem que exclui registros soft-deleted
// (deleted_at vazio = ativo). As coleções sem soft-delete simplesmente não têm
// o campo, então este filtro só é aplicado onde o domínio o exige.
const notDeletedFilter = "deleted_at = ''"

// mapError traduz um erro do PocketBase para os erros sentinela de internal/app.
//   - sql.ErrNoRows (FindRecordById/FindFirstRecordByFilter) -> app.ErrNotFound.
//   - violação de UNIQUE (validação "must be unique" do PocketBase) -> app.ErrConflict.
//
// Demais erros são propagados como estão (falha de I/O, validação de schema).
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return app.ErrNotFound
	}
	if isUniqueViolation(err) {
		return app.ErrConflict
	}
	return err
}

// isUniqueViolation detecta a violação de índice único. O PocketBase roda a
// validação de unicidade no Save e devolve uma validation.Errors cuja mensagem
// é "Value must be unique." (probe confirmado na v0.40.6); o driver SQLite
// também pode surfacar "UNIQUE constraint failed". Cobrimos ambas as formas.
func isUniqueViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "must be unique") ||
		strings.Contains(msg, "unique constraint")
}

// formatTime serializa um instante em ISO-8601 (RFC3339 America/Sao_Paulo) para
// persistência em TextField. Instante zero -> string vazia (coluna vazia).
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return clock.Format(t)
}

// parseTime interpreta uma string ISO-8601 (RFC3339). Vazia -> instante zero.
// Uma string malformada devolve o zero (tolerante na leitura, como o legado).
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := clock.Parse(s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// getRecordByID busca um record por id numa coleção e traduz o "não encontrado".
func getRecordByID(a core.App, collection, id string) (*core.Record, error) {
	rec, err := a.FindRecordById(collection, id)
	if err != nil {
		return nil, mapError(err)
	}
	return rec, nil
}

// isNoRows indica "nenhum registro" (FindFirst/FindRecordById) antes da tradução
// para app.ErrNotFound — útil nos upserts, que distinguem "não existe" de erro.
func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// dbxParams é um atalho para montar dbx.Params com um único par chave/valor,
// usado para vincular variáveis não confiáveis em filtros (NB! do PocketBase:
// sempre bindar valores de usuário, nunca interpolar em string).
func dbxParams(key string, value any) dbx.Params {
	return dbx.Params{key: value}
}

// existsByField indica se há algum registro ativo (não soft-deleted) numa
// coleção cujo campo tem o valor dado, ignorando opcionalmente um id (para
// checagens de unicidade em update). Usado por EmailExists/CPFExists.
func existsByField(a core.App, collection, field, value, exceptID string) (bool, error) {
	filter := field + " = {:v} && " + notDeletedFilter
	params := dbx.Params{"v": value}
	if exceptID != "" {
		filter += " && id != {:except}"
		params["except"] = exceptID
	}
	rec, err := a.FindFirstRecordByFilter(collection, filter, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return rec != nil, nil
}
