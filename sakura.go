package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	ogen "github.com/ogen-go/ogen/validate"
	secretmanager "github.com/sacloud/sacloud-sdk-go/api/secretmanager"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"
)

// maxNameLen is answered here, not asked of Unveil: a Vault holds no
// longer name, and a 422 for one would not mean a missing name.
const maxNameLen = 255

type sakuraStore struct {
	op secretmanager.SecretAPI
}

// newSakuraStore leaves the endpoint to the SDK, not built from a zone:
// every zone serves the same Vaults, and SAKURA_ENDPOINTS_SECRETMANAGER
// still overrides it.
func newSakuraStore(vaultID string) (*sakuraStore, error) {
	var sc saclient.Client
	if err := sc.SetEnviron(os.Environ()); err != nil {
		return nil, fmt.Errorf("failed to configure saclient: %w", err)
	}
	if err := sc.Populate(); err != nil {
		return nil, fmt.Errorf("failed to configure saclient: %w", err)
	}
	client, err := secretmanager.NewClient(&sc)
	if err != nil {
		return nil, fmt.Errorf("failed to create Secret Manager client: %w", err)
	}
	return &sakuraStore{op: secretmanager.NewSecretOp(client, vaultID)}, nil
}

func (s *sakuraStore) Get(ctx context.Context, name string) (secret, error) {
	if len(name) > maxNameLen {
		return secret{}, errNotFound
	}
	res, err := s.op.Unveil(ctx, secretmanager.UnveilParams{Name: name})
	if isNotFound(err) {
		return secret{}, errNotFound
	}
	if err != nil {
		return secret{}, err
	}
	return secret{value: []byte(res.Value), version: res.Version}, nil
}

// isNotFound reads 422, not 404, as a missing name: Unveil answers a name
// that does not exist with 422 and {"Errors":{"Secret":{"Name":"Not found"}}},
// which saclient.IsNotFoundError does not recognize. The name has passed
// namePattern and maxNameLen by then, so a 422 cannot be about its form.
func isNotFound(err error) bool {
	unexpected, ok := errors.AsType[*ogen.UnexpectedStatusCodeError](err)
	return ok && unexpected.StatusCode == http.StatusUnprocessableEntity
}
