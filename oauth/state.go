package oauth

import "context"

// ClientStateStore provides state management used by the OAuth client.
type ClientStateStore interface {
	Generate(ctx context.Context) (string, error)
	Validate(ctx context.Context, state string) error
}

