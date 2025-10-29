package graph

import "github.com/SP203/Token-Transfer-API/internal/db"

// Resolver trzyma zaleznosci dla resolverow (tu: Repo do DB).
type Resolver struct {
	Repo *db.Repo
}
