package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// 0030_clinical_therapist_transversais cria o grupo C do MER canônico (D4b):
// as coleções clínicas (anamnesis, note, ai_summary, tag_index), de terapeuta
// (therapist_profile, therapist_platform_link) e transversais (ged_file,
// audit_log, google_token). Depende dos grupos A (0010 — patient, session,
// origin) e B (0020 — debt, payment).
//
// Campos clínicos cifrados em repouso (anamnesis.conteudo, note.conteudo,
// ai_summary.resumo, google_token.token) são TextField que guardam o ENVELOPE
// (versao.algoritmo.id_da_chave.nonce.ciphertext) — nunca o claro, nunca uma
// validação de formato que atrapalhe o envelope. tag_index.tag fica em CLARO
// (índice invertido para busca; não é dado sensível por si só).
//
// Ordem respeita as dependências de relation (CollectionId exige o alvo já
// existente):
//
//	anamnesis -> note -> ai_summary -> tag_index -> therapist_profile ->
//	  therapist_platform_link -> ged_file -> audit_log -> google_token
func init() {
	m.Register(upClinicalTherapistTransversais, downClinicalTherapistTransversais, "0030_clinical_therapist_transversais")
}

func upClinicalTherapistTransversais(app core.App) error {
	// anamnesis ------------------------------------------------------------
	// 1:1 com patient (UNIQUE patient_id). conteudo = envelope cifrado.
	anamnesis := newBaseCollection("anamnesis")
	anPatientRel, err := relationTo(app, "patient_id", "patient", true)
	if err != nil {
		return err
	}
	anamnesis.Fields.Add(anPatientRel)
	anamnesis.Fields.Add(textField("conteudo", false))
	timestamps(anamnesis)
	anamnesis.AddIndex("ux_anamnesis_patient", true, "patient_id", "")
	if err := app.Save(anamnesis); err != nil {
		return err
	}

	// note -----------------------------------------------------------------
	// session_id nullable (nota pode ser avulsa do paciente). conteudo = envelope.
	note := newBaseCollection("note")
	ntPatientRel, err := relationTo(app, "patient_id", "patient", true)
	if err != nil {
		return err
	}
	note.Fields.Add(ntPatientRel)
	ntSessionRel, err := relationTo(app, "session_id", "session", false)
	if err != nil {
		return err
	}
	note.Fields.Add(ntSessionRel)
	note.Fields.Add(textField("conteudo", false))
	timestamps(note)
	if err := app.Save(note); err != nil {
		return err
	}

	// ai_summary -----------------------------------------------------------
	// resumo = envelope cifrado. status do ciclo de revisão humana.
	aiSummary := newBaseCollection("ai_summary")
	asSessionRel, err := relationTo(app, "session_id", "session", true)
	if err != nil {
		return err
	}
	aiSummary.Fields.Add(asSessionRel)
	aiSummary.Fields.Add(textField("resumo", false))
	aiSummary.Fields.Add(selectField("status", false, "sugerido", "revisado", "consolidado"))
	aiSummary.Fields.Add(isoTextField("generated_at"))
	timestamps(aiSummary)
	if err := app.Save(aiSummary); err != nil {
		return err
	}

	// tag_index ------------------------------------------------------------
	// tag em CLARO (índice invertido por paciente/ano). session_ids = JSON.
	tagIndex := newBaseCollection("tag_index")
	tiPatientRel, err := relationTo(app, "patient_id", "patient", true)
	if err != nil {
		return err
	}
	tagIndex.Fields.Add(tiPatientRel)
	tagIndex.Fields.Add(numberIntField("ano"))
	tagIndex.Fields.Add(textField("tag", false))
	tagIndex.Fields.Add(jsonField("session_ids"))
	timestamps(tagIndex)
	tagIndex.AddIndex("idx_tag_index_patient_ano_tag", false, "patient_id, ano, tag", "")
	if err := app.Save(tagIndex); err != nil {
		return err
	}

	// therapist_profile ----------------------------------------------------
	therapistProfile := newBaseCollection("therapist_profile")
	therapistProfile.Fields.Add(textField("name", true))
	therapistProfile.Fields.Add(textField("crp", false))
	therapistProfile.Fields.Add(textField("email", false))
	therapistProfile.Fields.Add(textField("bio", false))
	therapistProfile.Fields.Add(jsonField("contacts"))
	therapistProfile.Fields.Add(textField("photo_file_id", false))
	therapistProfile.Fields.Add(jsonField("location_ids"))
	timestamps(therapistProfile)
	if err := app.Save(therapistProfile); err != nil {
		return err
	}

	// therapist_platform_link ----------------------------------------------
	platformLink := newBaseCollection("therapist_platform_link")
	plProfileRel, err := relationTo(app, "profile_id", "therapist_profile", true)
	if err != nil {
		return err
	}
	platformLink.Fields.Add(plProfileRel)
	platformLink.Fields.Add(textField("label", true))
	platformLink.Fields.Add(textField("url", true))
	plOriginRel, err := relationTo(app, "origin_id", "origin", false)
	if err != nil {
		return err
	}
	platformLink.Fields.Add(plOriginRel)
	timestamps(platformLink)
	if err := app.Save(platformLink); err != nil {
		return err
	}

	// ged_file -------------------------------------------------------------
	// metadados de arquivos; todas as relations nullable (um GED pode anexar
	// a paciente, sessão, débito ou pagamento). índice (patient_id, sha256)
	// para deduplicação por paciente.
	gedFile := newBaseCollection("ged_file")
	gfPatientRel, err := relationTo(app, "patient_id", "patient", false)
	if err != nil {
		return err
	}
	gedFile.Fields.Add(gfPatientRel)
	gfSessionRel, err := relationTo(app, "session_id", "session", false)
	if err != nil {
		return err
	}
	gedFile.Fields.Add(gfSessionRel)
	gfDebtRel, err := relationTo(app, "debt_id", "debt", false)
	if err != nil {
		return err
	}
	gedFile.Fields.Add(gfDebtRel)
	gfPaymentRel, err := relationTo(app, "payment_id", "payment", false)
	if err != nil {
		return err
	}
	gedFile.Fields.Add(gfPaymentRel)
	gedFile.Fields.Add(textField("rel_path", false))
	gedFile.Fields.Add(textField("mime", false))
	gedFile.Fields.Add(numberIntField("size"))
	gedFile.Fields.Add(textField("sha256", false))
	timestamps(gedFile)
	gedFile.AddIndex("idx_ged_file_patient_sha256", false, "patient_id, sha256", "")
	if err := app.Save(gedFile); err != nil {
		return err
	}

	// audit_log ------------------------------------------------------------
	// metadata = JSON SEM conteúdo clínico. índice (entity, entity_id) p/ ListByEntity.
	auditLog := newBaseCollection("audit_log")
	auditLog.Fields.Add(textField("actor", false))
	auditLog.Fields.Add(textField("action", false))
	auditLog.Fields.Add(textField("entity", false))
	auditLog.Fields.Add(textField("entity_id", false))
	auditLog.Fields.Add(jsonField("metadata"))
	auditLog.Fields.Add(isoTextField("created_at"))
	auditLog.AddIndex("idx_audit_log_entity", false, "entity, entity_id", "")
	if err := app.Save(auditLog); err != nil {
		return err
	}

	// google_token ---------------------------------------------------------
	// token = envelope cifrado. SEM porta/adapter nesta fase (chega com o OAuth
	// do Google — Fase 6/7). scopes = JSON; expiry = ISO.
	googleToken := newBaseCollection("google_token")
	googleToken.Fields.Add(textField("token", false))
	googleToken.Fields.Add(jsonField("scopes"))
	googleToken.Fields.Add(isoTextField("expiry"))
	timestamps(googleToken)
	if err := app.Save(googleToken); err != nil {
		return err
	}

	return nil
}

func downClinicalTherapistTransversais(app core.App) error {
	// ordem inversa à criação (respeita as relations).
	return dropCollections(app,
		"google_token",
		"audit_log",
		"ged_file",
		"therapist_platform_link",
		"therapist_profile",
		"tag_index",
		"ai_summary",
		"note",
		"anamnesis",
	)
}
