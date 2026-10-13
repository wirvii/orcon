// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress

import "errors"

// ErrDenied is matched, with errors.Is, by every refusal of the egress policy.
// The host-level error code surfaced to flows is "EgressDenied" (ADR-0079 §5).
var ErrDenied = errors.New("egress denied")

// ErrResponseTooLarge is returned when a response, or a request body, exceeds
// the size limit.
var ErrResponseTooLarge = errors.New("egress: response exceeds the size limit")

// ErrRequestTooLarge is returned when a request body exceeds the size limit.
var ErrRequestTooLarge = errors.New("egress: request body exceeds the size limit")

// Reason says why a request was refused. The values are a closed set and are
// safe to use as metric labels.
type Reason string

// Refusal reasons.
const (
	ReasonBadURL                 Reason = "BAD_URL"
	ReasonBadPolicy              Reason = "BAD_POLICY"
	ReasonSchemeNotAllowed       Reason = "SCHEME_NOT_ALLOWED"
	ReasonPortNotAllowed         Reason = "PORT_NOT_ALLOWED"
	ReasonDomainNotGranted       Reason = "DOMAIN_NOT_GRANTED"
	ReasonCredentialHostPending  Reason = "CREDENTIAL_HOST_NOT_APPROVED" //nolint:gosec // a reason label, not a credential
	ReasonDenylisted             Reason = "DENYLISTED"
	ReasonPrivateAddress         Reason = "PRIVATE_ADDRESS"
	ReasonMetadataAddress        Reason = "METADATA_ADDRESS"
	ReasonTooManyRedirects       Reason = "TOO_MANY_REDIRECTS"
	ReasonUnsupportedNetworkType Reason = "UNSUPPORTED_NETWORK"
)

// DeniedError is a refusal of the egress policy. It carries the scheme and the
// host of the refused request and never the path, query or headers, because
// those can hold credentials (a webhook URL is a secret).
type DeniedError struct {
	Reason Reason
	// Scheme and Host identify the refused destination; either may be empty
	// when the URL could not be parsed.
	Scheme string
	Host   string
	// Detail is a short human explanation without secrets, such as the name of
	// the refused address range.
	Detail string
}

// Error implements error.
func (e *DeniedError) Error() string {
	msg := "egress denied: " + string(e.Reason)
	if e.Host != "" {
		msg += " (" + e.Scheme + "://" + e.Host + ")"
	}
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	return msg
}

// Is makes errors.Is(err, ErrDenied) true for every DeniedError.
func (e *DeniedError) Is(target error) bool { return target == ErrDenied }

// Code is the host error code of the refusal ("EgressDenied", ADR-0079 §5).
func (e *DeniedError) Code() string { return "EgressDenied" }
