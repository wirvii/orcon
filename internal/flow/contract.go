// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"fmt"
	"slices"
)

// SafetyLevel is how strongly an operation can be recovered (ADR-0029 section 5).
type SafetyLevel string

// Safety levels.
const (
	// SafetyReadOnly marks an operation with no side effects.
	SafetyReadOnly SafetyLevel = "READ_ONLY"
	// SafetyIdempotencyKey marks an operation deduplicated by a key.
	SafetyIdempotencyKey SafetyLevel = "IDEMPOTENCY_KEY"
	// SafetyReconcile marks an operation whose outcome can be looked up.
	SafetyReconcile SafetyLevel = "RECONCILE"
	// SafetyUnsafe marks an operation that is neither keyed nor verifiable.
	SafetyUnsafe SafetyLevel = "UNSAFE"
)

// SafetyLevels returns every valid [SafetyLevel].
func SafetyLevels() []SafetyLevel {
	return []SafetyLevel{SafetyReadOnly, SafetyIdempotencyKey, SafetyReconcile, SafetyUnsafe}
}

// KeyLocation says where an idempotency key travels in the request.
type KeyLocation string

// Key locations.
const (
	// KeyInHeader puts the key in a header.
	KeyInHeader KeyLocation = "HEADER"
	// KeyInQuery puts the key in the query string.
	KeyInQuery KeyLocation = "QUERY"
	// KeyInBody puts the key in the request body.
	KeyInBody KeyLocation = "BODY"
)

// KeyLocations returns every valid [KeyLocation].
func KeyLocations() []KeyLocation { return []KeyLocation{KeyInHeader, KeyInQuery, KeyInBody} }

// Contract is the machine-readable safety contract of an operation
// (ADR-0029 section 5, ADR-0012 section 1). It belongs to a node type, not to
// a flow document: a flow cannot declare itself safer than its node type.
type Contract struct {
	Effect       EffectKind
	Level        SafetyLevel
	Idempotency  *Idempotency
	Verification *Verification
	OnUncertain  OnUncertain
	RetrySafe    bool
}

// Idempotency describes how repeated requests are deduplicated.
type Idempotency struct {
	Mode IdempotencyMode
	// Location is where the key travels; required with a key mode.
	Location *KeyPlace
	// TTL is the ISO 8601 duration a key stays valid at the provider.
	TTL string
}

// KeyPlace names where an idempotency key is carried.
type KeyPlace struct {
	In   KeyLocation
	Name string
}

// Verification names the operation that looks an outcome up.
type Verification struct {
	Operation string
	// OperationPurity is the purity of that operation, resolved by the caller;
	// reconciling needs it to be READ.
	OperationPurity Purity
}

// CheckContract applies the validity rules of ADR-0029 section 5 and returns
// one message per broken rule; an empty result means the contract is valid.
// It is a pure function of the contract:
//   - UNKNOWN is treated as WRITE, so it is held to the WRITE rules;
//   - READ_ONLY is allowed only with effect READ;
//   - IDEMPOTENCY_KEY needs a key mode (PROVIDER_KEY or NATURAL_KEY), a key
//     location and a ttl;
//   - RECONCILE needs a verification operation whose purity is READ;
//   - UNSAFE needs retrySafe false and onUncertain SUSPEND;
//   - RESEND_WITH_KEY needs the IDEMPOTENCY_KEY level; RECONCILE as the
//     recovery needs a verification operation.
func CheckContract(c *Contract) []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	if !slices.Contains(EffectKinds(), c.Effect) {
		add("effect %q is not one of READ, WRITE, DELETE, UNKNOWN", c.Effect)
	}
	if !slices.Contains(SafetyLevels(), c.Level) {
		add("level %q is not one of READ_ONLY, IDEMPOTENCY_KEY, RECONCILE, UNSAFE", c.Level)
	}
	if c.Level == SafetyReadOnly && c.Effect != EffectRead {
		add("level READ_ONLY is allowed only with effect READ, not %s", c.Effect)
	}
	if c.Level == SafetyIdempotencyKey {
		problems = append(problems, keyProblems(c.Idempotency)...)
	}
	if c.Level == SafetyReconcile {
		problems = append(problems, verificationProblems(c.Verification)...)
	}
	if c.Level == SafetyUnsafe {
		if c.RetrySafe {
			add("level UNSAFE needs retrySafe false")
		}
		if c.OnUncertain != OnUncertainSuspend {
			add("level UNSAFE needs onUncertain SUSPEND")
		}
	}
	if c.Level != SafetyReadOnly {
		problems = append(problems, recoveryProblems(c)...)
	}
	return problems
}

func keyProblems(idem *Idempotency) []string {
	if idem == nil || (idem.Mode != IdempotencyProviderKey && idem.Mode != IdempotencyNaturalKey) {
		return []string{"level IDEMPOTENCY_KEY needs an idempotency mode of PROVIDER_KEY or NATURAL_KEY"}
	}
	var problems []string
	if idem.Location == nil || idem.Location.Name == "" || !slices.Contains(KeyLocations(), idem.Location.In) {
		problems = append(problems, "level IDEMPOTENCY_KEY needs an idempotency location (HEADER, QUERY or BODY, and a name)")
	}
	if idem.TTL == "" {
		problems = append(problems, "level IDEMPOTENCY_KEY needs an idempotency ttl")
	} else if !durationPattern.MatchString(idem.TTL) {
		problems = append(problems, fmt.Sprintf("idempotency ttl %q is not an ISO 8601 duration", idem.TTL))
	}
	return problems
}

func verificationProblems(v *Verification) []string {
	switch {
	case v == nil || v.Operation == "":
		return []string{"level RECONCILE needs a verification operation"}
	case v.OperationPurity != PurityRead:
		return []string{fmt.Sprintf("the verification operation %q must have purity READ, not %q", v.Operation, v.OperationPurity)}
	default:
		return nil
	}
}

// recoveryProblems checks that the recovery the contract names is backed by
// the rest of it.
func recoveryProblems(c *Contract) []string {
	switch c.OnUncertain {
	case OnUncertainResendWithKey:
		if c.Level != SafetyIdempotencyKey {
			return []string{"onUncertain RESEND_WITH_KEY needs level IDEMPOTENCY_KEY"}
		}
	case OnUncertainReconcile:
		if c.Verification == nil || c.Verification.Operation == "" {
			return []string{"onUncertain RECONCILE needs a verification operation"}
		}
	case OnUncertainSuspend:
		// Always possible: a person decides.
	default:
		return []string{fmt.Sprintf("onUncertain %q is not one of RESEND_WITH_KEY, RECONCILE, SUSPEND", c.OnUncertain)}
	}
	return nil
}
