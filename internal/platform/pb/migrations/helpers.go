// Package migrations define o schema do PocketBase (coleções do MER canônico D4b)
// como migrations idempotentes aplicadas no boot (RunAllMigrations).
//
// Faz parte da fronteira platform/pb (o ÚNICO lugar que importa o PocketBase).
// O domínio e o internal/app nunca importam este pacote (ver archtest).
//
// Convenções do projeto (psicoman-sqlite.md, psicoman-golang.md):
//   - id é um ULID (26 chars Crockford base32) gerado pela app, NÃO pelo PocketBase;
//     o campo id default do PB (15 chars [a-z0-9], autogerado) é substituído por um
//     TextField custom sem AutogeneratePattern (ver newBaseCollection).
//   - created_at/updated_at/deleted_at são TextField ISO-8601 (RFC3339 America/Sao_Paulo)
//     gravados pela app — nunca AutodateField nem DEFAULT CURRENT_TIMESTAMP (que usa UTC).
//   - dinheiro é NumberField{OnlyInt:true} em centavos.
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// ulidPattern é o conjunto Crockford base32 maiúsculo usado pelo gerador de ULID
// do projeto (internal/platform/ulid). 26 caracteres, sem I/L/O/U.
const ulidPattern = "^[0-9A-HJKMNP-TV-Z]{26}$"

// newBaseCollection cria uma coleção base e troca o campo id default do PocketBase
// por um TextField ULID custom (26 chars, sem autogenerate: a app fornece o id).
//
// initIdField (core/collection_model.go) respeita um campo id pré-existente — só
// reforça System/Required/PrimaryKey e, como já definimos Pattern, não o sobrescreve.
// Sem AutogeneratePattern, o PB não autogera o id (field_text.go), então o ULID
// gravado pela app via rec.Set("id", ...) é preservado.
func newBaseCollection(name string) *core.Collection {
	col := core.NewBaseCollection(name)
	col.Fields.Add(&core.TextField{
		Name:       "id",
		System:     true,
		PrimaryKey: true,
		Required:   true,
		Min:        26,
		Max:        26,
		Pattern:    ulidPattern,
	})
	return col
}

// textField cria um TextField simples.
func textField(name string, required bool) *core.TextField {
	return &core.TextField{Name: name, Required: required}
}

// isoTextField cria um TextField para timestamps ISO-8601 (RFC3339). Sempre
// opcional no schema: a app controla o preenchimento (created_at/updated_at
// sempre; deleted_at vazio = ativo).
func isoTextField(name string) *core.TextField {
	return &core.TextField{Name: name}
}

// selectField cria um SelectField de valor único a partir de uma lista fechada.
func selectField(name string, required bool, values ...string) *core.SelectField {
	return &core.SelectField{
		Name:      name,
		Required:  required,
		MaxSelect: 1,
		Values:    values,
	}
}

// numberIntField cria um NumberField inteiro (dinheiro em centavos, contadores).
func numberIntField(name string) *core.NumberField {
	return &core.NumberField{Name: name, OnlyInt: true}
}

// boolField cria um BoolField.
func boolField(name string) *core.BoolField {
	return &core.BoolField{Name: name}
}

// timestamps adiciona created_at/updated_at (sempre) à coleção.
func timestamps(col *core.Collection) {
	col.Fields.Add(isoTextField("created_at"))
	col.Fields.Add(isoTextField("updated_at"))
}

// softDelete adiciona o campo deleted_at (TextField ISO, vazio = ativo).
func softDelete(col *core.Collection) {
	col.Fields.Add(isoTextField("deleted_at"))
}

// relationTo resolve o CollectionId da coleção alvo (que precisa já existir) e
// devolve um RelationField de seleção única. Retorna erro se o alvo não existir
// — o que indica ordem de criação errada na migration.
func relationTo(app core.App, name, targetName string, required bool) (*core.RelationField, error) {
	target, err := app.FindCollectionByNameOrId(targetName)
	if err != nil {
		return nil, err
	}
	return &core.RelationField{
		Name:         name,
		CollectionId: target.Id,
		MaxSelect:    1,
		Required:     required,
	}, nil
}

// dropCollections remove, na ordem dada, as coleções informadas (usado nos down).
// Ignora coleções já inexistentes para manter o down idempotente.
func dropCollections(app core.App, names ...string) error {
	for _, name := range names {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			continue // já removida
		}
		if err := app.Delete(col); err != nil {
			return err
		}
	}
	return nil
}
