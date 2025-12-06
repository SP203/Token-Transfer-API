package graph

import (
	"context"

	"github.com/SP203/Token-Transfer-API/graph/generated"
)

func (r *mutationResolver) Transfer(ctx context.Context, fromAddress string, toAddress string, amount int) (int, error) {
	newBal, err := r.Repo.Transfer(ctx, fromAddress, toAddress, int64(amount))
	if err != nil {
		return 0, err
	}
	return int(newBal), nil
}

func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

type mutationResolver struct{ *Resolver }
