package graph

import (
	"context"
	"errors"

	"github.com/SP203/Token-Transfer-API/graph/generated"
	"github.com/SP203/Token-Transfer-API/internal/db"
)

func (r *mutationResolver) Transfer(ctx context.Context, fromAddress string, toAddress string, amount int) (int, error) {
	newBal, err := r.Repo.Transfer(ctx, fromAddress, toAddress, int64(amount))
	if err != nil {
		if errors.Is(err, db.ErrInsufficient) {
			// dokładnie taki komunikat wymaga zadanie
			return 0, db.ErrInsufficient
		}
		return 0, err
	}
	return int(newBal), nil
}

func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

type mutationResolver struct{ *Resolver }
