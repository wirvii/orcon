// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// objectType and secretField are the AAD names of a credential secret (ADR-0042 section 1): a ciphertext moved to
// another organization, version or row fails to open.
const (
	objectType  = "credential_version"
	secretField = "secrets"
)

// encodeFields returns the canonical JSON object of a field map. Values are strings only.
func encodeFields(fields map[string]string) ([]byte, error) {
	members := make([]jsonvalue.Member, 0, len(fields))
	for k, v := range fields {
		members = append(members, jsonvalue.Member{Key: k, Value: jsonvalue.String(v)})
	}
	obj, err := jsonvalue.Object(members...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	doc, err := jsonvalue.Canonical(obj)
	if err != nil {
		return nil, fmt.Errorf("encode fields: %w", err)
	}
	return doc, nil
}

// decodeFields reads a field map back. A document that is not an object of strings is ErrDecrypt: it can only come
// from a damaged row, since the writer produced it.
func decodeFields(doc []byte) (map[string]string, error) {
	v, err := jsonvalue.Parse(doc)
	if err != nil {
		return nil, fmt.Errorf("fields: %w", ErrDecrypt)
	}
	members := v.Members()
	out := make(map[string]string, len(members))
	for _, m := range members {
		s, ok := m.Value.Str()
		if !ok {
			return nil, fmt.Errorf("fields: %w", ErrDecrypt)
		}
		out[m.Key] = s
	}
	return out, nil
}

// sealVersion encrypts the secret fields of a new version under a fresh data key wrapped by the active organization
// key. The public fields are stored as JSON beside it. The data key is wiped before the function returns.
func (s *Service) sealVersion(ctx context.Context, org identity.OrgID, in Input, versionID, credentialID id.UUID, number int, by identity.PrincipalID, at time.Time) (Version, error) {
	public, err := encodeFields(in.Public)
	if err != nil {
		return Version{}, err
	}
	secret, err := encodeFields(in.Secret)
	if err != nil {
		return Version{}, err
	}
	if len(secret) > maxSecretDocumentBytes {
		return Version{}, fmt.Errorf("%w: the secret fields are too large", ErrInvalid)
	}
	dk, err := s.keys.GenerateDataKey(ctx, envelope.OrgScope(org))
	if err != nil {
		return Version{}, fmt.Errorf("data key for a credential: %w", err)
	}
	defer dk.Key.Destroy()
	aad := envelope.AAD{OrgID: org, ObjectType: objectType, ObjectID: versionID.String(), Version: number, Field: secretField}
	env, err := envelope.Seal(dk.Key, dk.ID, aad, secret)
	if err != nil {
		return Version{}, fmt.Errorf("seal a credential: %w", err)
	}
	blob, err := env.MarshalBinary()
	if err != nil {
		return Version{}, fmt.Errorf("encode a credential ciphertext: %w", err)
	}
	return Version{
		ID: versionID, OrgID: org, CredentialID: credentialID, Number: number,
		PublicFields: public, SecretCiphertext: blob, DataKeyWrapped: dk.Wrapped, OrgKeyVersion: dk.ID.Version,
		CreatedBy: by, CreatedAt: at,
	}, nil
}

// openSecret decrypts the secret fields of a stored version. The data key is wiped before the function returns; the
// returned values live in memory only.
func (s *Service) openSecret(ctx context.Context, v Version) (map[string]string, error) {
	var env envelope.Envelope
	if err := env.UnmarshalBinary(v.SecretCiphertext); err != nil {
		return nil, fmt.Errorf("version %d: %w", v.Number, ErrDecrypt)
	}
	key, err := s.keys.UnwrapDataKey(ctx, env.KID, v.DataKeyWrapped)
	if err != nil {
		return nil, fmt.Errorf("version %d: %w: %w", v.Number, ErrDecrypt, err)
	}
	defer key.Destroy()
	aad := envelope.AAD{OrgID: v.OrgID, ObjectType: objectType, ObjectID: v.ID.String(), Version: v.Number, Field: secretField}
	plain, err := env.Open(key, aad)
	if err != nil {
		return nil, fmt.Errorf("version %d: %w", v.Number, ErrDecrypt)
	}
	return decodeFields(plain)
}
