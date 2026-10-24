package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// 0020_billing_costs cria o grupo B do MER canônico (D4b): as coleções de
// faturamento (contract, plan, debt, payment) e custos (cost_category,
// cost_item, session_cost). Depende do grupo A (0010) — patient, session,
// location e origin precisam existir.
//
// Ordem respeita as dependências de relation (CollectionId exige o alvo já
// existente) e resolve os ciclos por patch após ambas as coleções existirem:
//
//	cost_category -> cost_item -> contract (sem plan_id) -> plan ->
//	  PATCH contract.plan_id -> debt -> payment -> session_cost ->
//	  PATCH session.contract_id/plan_id
func init() {
	m.Register(upBillingCosts, downBillingCosts, "0020_billing_costs")
}

func upBillingCosts(app core.App) error {
	// cost_category --------------------------------------------------------
	costCategory := newBaseCollection("cost_category")
	costCategory.Fields.Add(selectField("kind", false, "local", "crp", "infra", "plataforma"))
	costCategory.Fields.Add(textField("name", true))
	timestamps(costCategory)
	if err := app.Save(costCategory); err != nil {
		return err
	}

	// cost_item ------------------------------------------------------------
	costItem := newBaseCollection("cost_item")
	ciCategoryRel, err := relationTo(app, "category_id", "cost_category", false)
	if err != nil {
		return err
	}
	costItem.Fields.Add(ciCategoryRel)
	costItem.Fields.Add(textField("name", true))
	costItem.Fields.Add(numberIntField("amount"))
	costItem.Fields.Add(selectField("period", false, "por_sessao", "diario", "mensal", "anual"))
	ciOriginRel, err := relationTo(app, "origin_id", "origin", false)
	if err != nil {
		return err
	}
	costItem.Fields.Add(ciOriginRel)
	ciLocationRel, err := relationTo(app, "location_id", "location", false)
	if err != nil {
		return err
	}
	costItem.Fields.Add(ciLocationRel)
	timestamps(costItem)
	softDelete(costItem)
	costItem.AddIndex("idx_cost_item_deleted_at", false, "deleted_at", "")
	if err := app.Save(costItem); err != nil {
		return err
	}

	// contract (sem plan_id ainda — ciclo contract<->plan) -----------------
	contract := newBaseCollection("contract")
	cPatientRel, err := relationTo(app, "patient_id", "patient", true)
	if err != nil {
		return err
	}
	contract.Fields.Add(cPatientRel)
	contract.Fields.Add(isoTextField("starts_at"))
	contract.Fields.Add(isoTextField("ends_at"))
	contract.Fields.Add(textField("adjustment_rule", false))
	contract.Fields.Add(textField("artifact_file_id", false))
	timestamps(contract)
	softDelete(contract)
	contract.AddIndex("idx_contract_deleted_at", false, "deleted_at", "")
	if err := app.Save(contract); err != nil {
		return err
	}

	// plan -----------------------------------------------------------------
	plan := newBaseCollection("plan")
	pPatientRel, err := relationTo(app, "patient_id", "patient", true)
	if err != nil {
		return err
	}
	plan.Fields.Add(pPatientRel)
	pContractRel, err := relationTo(app, "contract_id", "contract", false)
	if err != nil {
		return err
	}
	plan.Fields.Add(pContractRel)
	plan.Fields.Add(selectField("type", false,
		"pagamento_por_consulta", "pagamento_por_mes",
		"plano_fechado_mensal", "plano_fechado_trimestral", "atendimento_social"))
	plan.Fields.Add(numberIntField("amount"))
	plan.Fields.Add(numberIntField("due_day"))
	plan.Fields.Add(isoTextField("starts_at"))
	plan.Fields.Add(isoTextField("ends_at"))
	timestamps(plan)
	softDelete(plan)
	plan.AddIndex("idx_plan_deleted_at", false, "deleted_at", "")
	if err := app.Save(plan); err != nil {
		return err
	}

	// PATCH contract.plan_id -> plan (fecha o ciclo contract<->plan) --------
	contract, err = app.FindCollectionByNameOrId("contract")
	if err != nil {
		return err
	}
	cPlanRel, err := relationTo(app, "plan_id", "plan", false)
	if err != nil {
		return err
	}
	contract.Fields.Add(cPlanRel)
	if err := app.Save(contract); err != nil {
		return err
	}

	// debt -----------------------------------------------------------------
	debt := newBaseCollection("debt")
	dPatientRel, err := relationTo(app, "patient_id", "patient", true)
	if err != nil {
		return err
	}
	debt.Fields.Add(dPatientRel)
	dSessionRel, err := relationTo(app, "session_id", "session", false)
	if err != nil {
		return err
	}
	debt.Fields.Add(dSessionRel)
	dPlanRel, err := relationTo(app, "plan_id", "plan", false)
	if err != nil {
		return err
	}
	debt.Fields.Add(dPlanRel)
	debt.Fields.Add(textField("billing_period", false))
	debt.Fields.Add(numberIntField("amount"))
	debt.Fields.Add(isoTextField("due_date"))
	debt.Fields.Add(selectField("status", false, "aberto", "pago", "parcial"))
	debt.Fields.Add(textField("idempotency_key", true))
	debt.Fields.Add(textField("pdf_file_id", false))
	timestamps(debt)
	debt.AddIndex("ux_debt_idempotency", true, "idempotency_key", "")
	if err := app.Save(debt); err != nil {
		return err
	}

	// payment --------------------------------------------------------------
	payment := newBaseCollection("payment")
	payDebtRel, err := relationTo(app, "debt_id", "debt", true)
	if err != nil {
		return err
	}
	payment.Fields.Add(payDebtRel)
	payment.Fields.Add(numberIntField("amount"))
	payment.Fields.Add(isoTextField("paid_at"))
	payment.Fields.Add(textField("method", false))
	timestamps(payment)
	if err := app.Save(payment); err != nil {
		return err
	}

	// session_cost ---------------------------------------------------------
	sessionCost := newBaseCollection("session_cost")
	scSessionRel, err := relationTo(app, "session_id", "session", true)
	if err != nil {
		return err
	}
	sessionCost.Fields.Add(scSessionRel)
	sessionCost.Fields.Add(numberIntField("amount"))
	sessionCost.Fields.Add(selectField("method", false, "direto", "rateio"))
	sessionCost.Fields.Add(textField("base_snapshot", false))
	timestamps(sessionCost)
	sessionCost.AddIndex("ux_session_cost_session", true, "session_id", "")
	if err := app.Save(sessionCost); err != nil {
		return err
	}

	// PATCH session.contract_id/plan_id (ciclo session->contract/plan) ------
	session, err := app.FindCollectionByNameOrId("session")
	if err != nil {
		return err
	}
	sContractRel, err := relationTo(app, "contract_id", "contract", false)
	if err != nil {
		return err
	}
	session.Fields.Add(sContractRel)
	sPlanRel, err := relationTo(app, "plan_id", "plan", false)
	if err != nil {
		return err
	}
	session.Fields.Add(sPlanRel)
	if err := app.Save(session); err != nil {
		return err
	}

	return nil
}

func downBillingCosts(app core.App) error {
	// Remove primeiro os campos de relation adicionados por patch às coleções
	// do grupo A (session), para manter o down idempotente e não deixar
	// relations órfãs apontando para coleções prestes a sumir.
	if session, err := app.FindCollectionByNameOrId("session"); err == nil {
		changed := false
		for _, name := range []string{"contract_id", "plan_id"} {
			if f := session.Fields.GetByName(name); f != nil {
				session.Fields.RemoveByName(name)
				changed = true
			}
		}
		if changed {
			if err := app.Save(session); err != nil {
				return err
			}
		}
	}

	// ordem inversa à criação (respeita as relations).
	return dropCollections(app,
		"session_cost",
		"payment",
		"debt",
		"plan",
		"contract",
		"cost_item",
		"cost_category",
	)
}
