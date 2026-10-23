package httpx

// Chaves do store do RequestEvent (e.Get/e.Set) usadas pela middleware chain.
// São strings de pacote para evitar colisão com chaves do PocketBase.
const (
	storeRequestID = "psicoman_request_id"
	storeStartedAt = "psicoman_started_at"
)
