package httpx

// Chaves do store do RequestEvent (e.Get/e.Set) usadas pela middleware chain.
// São strings de pacote para evitar colisão com chaves do PocketBase.
const (
	storeRequestID = "psicoman_request_id"
	storeStartedAt = "psicoman_started_at"
	// storeActorIdentity guarda a identidade do ator (terapeuta/operação)
	// resolvida do header do Pangolin pelo middleware de auth (Fase 5).
	storeActorIdentity = "psicoman_actor_identity"
	// storePortalPatientID guarda o id do paciente resolvido pelo gate de
	// aprovação do portal, disponível ao handler (Fase 5; seam Fase 9).
	storePortalPatientID = "psicoman_portal_patient_id"
)
