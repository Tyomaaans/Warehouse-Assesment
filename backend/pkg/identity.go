package pkg

import (
	"context"

	"github.com/google/uuid"
)

const Operator = "OPERATOR"
const Supervisor = "SUPERVISOR"

type Actor struct {
	ID        uuid.UUID
	SessionID uuid.UUID
	Role      string
}

type actorKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

func RequireActor(ctx context.Context, role string) (Actor, error) {
	actor, ok := ctx.Value(actorKey{}).(Actor)
	if !ok || actor.ID == uuid.Nil {
		return Actor{}, ErrUnauthorized
	}
	if actor.Role != Operator && actor.Role != Supervisor {
		return Actor{}, ErrForbidden
	}
	if role != "" && actor.Role != role {
		return Actor{}, ErrForbidden
	}
	return actor, nil
}
