// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import "github.com/wirvii/orcon/internal/flow"

// FormatTag is the value of the "format" field of a manifest.
const FormatTag = "orcon/connector@1"

// SchemaID is the $id of the published JSON Schema of the manifest.
const SchemaID = "urn:orcon:schema:connector:1"

// Every enumerated value below is UPPER_SNAKE, with one spelling in the
// manifest, the API and the database (ADR-0029 section 10, ADR-0080 section 3).

// Kind is how a connector runs (ADR-0029 section 1).
type Kind string

// Connector kinds.
const (
	// KindDeclarative is a data-only description of an HTTP API.
	KindDeclarative Kind = "DECLARATIVE"
	// KindNative is Go code compiled into the binary; official connectors only.
	KindNative Kind = "NATIVE"
	// KindWASM is a WebAssembly module run in the sandbox.
	KindWASM Kind = "WASM"
	// KindLibrary is a Code node library package.
	KindLibrary Kind = "LIBRARY"
	// KindRuntime is a Code node language runtime package.
	KindRuntime Kind = "RUNTIME"
)

// Kinds returns every valid [Kind].
func Kinds() []Kind {
	return []Kind{KindDeclarative, KindNative, KindWASM, KindLibrary, KindRuntime}
}

// Group says whether a node starts a flow or acts inside it.
type Group string

// Node groups.
const (
	// GroupAction is a node that runs inside a flow.
	GroupAction Group = "ACTION"
	// GroupTrigger is a node that starts a flow.
	GroupTrigger Group = "TRIGGER"
)

// Groups returns every valid [Group].
func Groups() []Group { return []Group{GroupAction, GroupTrigger} }

// ItemMode says how an operation consumes items.
type ItemMode string

// Item modes.
const (
	// ItemPerItem runs the operation once per item.
	ItemPerItem ItemMode = "PER_ITEM"
	// ItemBatch hands the operation all the items at once.
	ItemBatch ItemMode = "BATCH"
)

// ItemModes returns every valid [ItemMode].
func ItemModes() []ItemMode { return []ItemMode{ItemPerItem, ItemBatch} }

// OperationPurities returns the purity classes a manifest operation may
// declare (ADR-0029 section 4); AGENTIC belongs to the AI design.
func OperationPurities() []flow.Purity {
	return []flow.Purity{flow.PurityPure, flow.PurityRead, flow.PurityEffect, flow.PurityInternalEffect}
}

// ProofLevel says who vouches for a safety contract (ADR-0029 section 5).
type ProofLevel string

// Proof levels.
const (
	// ProofDeclared is what the publisher says.
	ProofDeclared ProofLevel = "DECLARED"
	// ProofTested means the package tests pass against the simulated provider.
	ProofTested ProofLevel = "TESTED"
	// ProofVerified means Orcon's review checked it against the real service.
	ProofVerified ProofLevel = "VERIFIED"
)

// ProofLevels returns every valid [ProofLevel].
func ProofLevels() []ProofLevel { return []ProofLevel{ProofDeclared, ProofTested, ProofVerified} }

// KeyScope is how far an idempotency key is unique.
type KeyScope string

// Idempotency key scopes.
const (
	// ScopeCredential makes the key unique per credential.
	ScopeCredential KeyScope = "CREDENTIAL"
	// ScopeAccount makes the key unique per provider account.
	ScopeAccount KeyScope = "ACCOUNT"
	// ScopeGlobal makes the key unique across the provider.
	ScopeGlobal KeyScope = "GLOBAL"
)

// KeyScopes returns every valid [KeyScope].
func KeyScopes() []KeyScope { return []KeyScope{ScopeCredential, ScopeAccount, ScopeGlobal} }

// TriggerFamily is the family of a trigger node (ADR-0020 section 1).
type TriggerFamily string

// Trigger families.
const (
	// FamilyWebhook is inbound HTTP.
	FamilyWebhook TriggerFamily = "WEBHOOK"
	// FamilyAppEvent is inbound HTTP registered at a provider by Orcon.
	FamilyAppEvent TriggerFamily = "APP_EVENT"
	// FamilySchedule is calendar or interval rules.
	FamilySchedule TriggerFamily = "SCHEDULE"
	// FamilyPoll is a periodic pull with a durable cursor.
	FamilyPoll TriggerFamily = "POLL"
	// FamilyListen is a persistent connection to a broker, stream or mailbox.
	FamilyListen TriggerFamily = "LISTEN"
	// FamilyInternal is a manual run, a call from a flow or another internal source.
	FamilyInternal TriggerFamily = "INTERNAL"
)

// TriggerFamilies returns every valid [TriggerFamily].
func TriggerFamilies() []TriggerFamily {
	return []TriggerFamily{FamilyWebhook, FamilyAppEvent, FamilySchedule, FamilyPoll, FamilyListen, FamilyInternal}
}

// Delivery is the delivery guarantee of a trigger (ADR-0020 section 3).
type Delivery string

// Trigger delivery guarantees.
const (
	// DeliveryAtLeastOnce may deliver an event more than once.
	DeliveryAtLeastOnce Delivery = "AT_LEAST_ONCE"
	// DeliveryAtMostOnce may lose an event but never repeats it.
	DeliveryAtMostOnce Delivery = "AT_MOST_ONCE"
	// DeliveryCursorExact resumes from a durable cursor without repeats.
	DeliveryCursorExact Delivery = "CURSOR_EXACT"
)

// Deliveries returns every valid [Delivery].
func Deliveries() []Delivery {
	return []Delivery{DeliveryAtLeastOnce, DeliveryAtMostOnce, DeliveryCursorExact}
}

// DedupeSource says where the deduplication key of a trigger event comes from.
type DedupeSource string

// Deduplication key sources.
const (
	// DedupeHeader reads the key from a request header.
	DedupeHeader DedupeSource = "HEADER"
	// DedupeJSONPath reads the key from the payload.
	DedupeJSONPath DedupeSource = "JSON_PATH"
	// DedupeConnector lets the connector compute the key.
	DedupeConnector DedupeSource = "CONNECTOR"
)

// DedupeSources returns every valid [DedupeSource].
func DedupeSources() []DedupeSource {
	return []DedupeSource{DedupeHeader, DedupeJSONPath, DedupeConnector}
}

// FieldType is the type of a credential field (ADR-0030 section 3).
type FieldType string

// Credential field types.
const (
	// FieldString is a text value.
	FieldString FieldType = "STRING"
	// FieldNumber is a number.
	FieldNumber FieldType = "NUMBER"
	// FieldBoolean is a true or false value.
	FieldBoolean FieldType = "BOOLEAN"
	// FieldOptions is one value out of a list.
	FieldOptions FieldType = "OPTIONS"
)

// FieldTypes returns every valid [FieldType].
func FieldTypes() []FieldType {
	return []FieldType{FieldString, FieldNumber, FieldBoolean, FieldOptions}
}

// SchemeType is the authentication scheme of a credential (ADR-0030 section 4).
type SchemeType string

// Authentication schemes.
const (
	// SchemeNone sends no credential.
	SchemeNone SchemeType = "NONE"
	// SchemeAPIKey sends a key in a header, the query string or a cookie.
	SchemeAPIKey SchemeType = "API_KEY"
	// SchemeBasic sends HTTP basic authentication.
	SchemeBasic SchemeType = "BASIC"
	// SchemeBearer sends a bearer token.
	SchemeBearer SchemeType = "BEARER"
	// SchemeOAuth2 runs an OAuth 2 flow owned by the security design.
	SchemeOAuth2 SchemeType = "OAUTH2"
	// SchemeCustomHeaders sends templated headers.
	SchemeCustomHeaders SchemeType = "CUSTOM_HEADERS"
	// SchemeAWSSigV4 signs requests with AWS Signature Version 4.
	SchemeAWSSigV4 SchemeType = "AWS_SIG_V4"
	// SchemeSessionToken logs in with an operation and reuses its token.
	SchemeSessionToken SchemeType = "SESSION_TOKEN"
)

// SchemeTypes returns every valid [SchemeType].
func SchemeTypes() []SchemeType {
	return []SchemeType{SchemeNone, SchemeAPIKey, SchemeBasic, SchemeBearer, SchemeOAuth2, SchemeCustomHeaders, SchemeAWSSigV4, SchemeSessionToken}
}

// Placement says where an API key travels.
type Placement string

// API key placements.
const (
	// PlaceHeader puts the key in a header.
	PlaceHeader Placement = "HEADER"
	// PlaceQuery puts the key in the query string.
	PlaceQuery Placement = "QUERY"
	// PlaceCookie puts the key in a cookie.
	PlaceCookie Placement = "COOKIE"
)

// Placements returns every valid [Placement].
func Placements() []Placement { return []Placement{PlaceHeader, PlaceQuery, PlaceCookie} }

// OAuthGrant is the OAuth 2 grant of a credential.
type OAuthGrant string

// OAuth 2 grants.
const (
	// GrantAuthorizationCode is the authorization code grant.
	GrantAuthorizationCode OAuthGrant = "AUTHORIZATION_CODE"
	// GrantPKCE is the authorization code grant with a proof key.
	GrantPKCE OAuthGrant = "PKCE"
	// GrantClientCredentials is the client credentials grant.
	GrantClientCredentials OAuthGrant = "CLIENT_CREDENTIALS" //nolint:gosec // G101: the name mentions credentials; the value is an enumeration, not a secret
)

// OAuthGrants returns every valid [OAuthGrant].
func OAuthGrants() []OAuthGrant {
	return []OAuthGrant{GrantAuthorizationCode, GrantPKCE, GrantClientCredentials}
}

// ResetFormat is how a provider writes the time its rate limit resets.
type ResetFormat string

// Rate limit reset formats.
const (
	// ResetEpoch is seconds since the Unix epoch.
	ResetEpoch ResetFormat = "EPOCH"
	// ResetEpochMillis is milliseconds since the Unix epoch.
	ResetEpochMillis ResetFormat = "EPOCH_MS"
	// ResetSeconds is seconds from now.
	ResetSeconds ResetFormat = "SECONDS"
	// ResetHTTPDate is an HTTP date.
	ResetHTTPDate ResetFormat = "HTTP_DATE"
)

// ResetFormats returns every valid [ResetFormat].
func ResetFormats() []ResetFormat {
	return []ResetFormat{ResetEpoch, ResetEpochMillis, ResetSeconds, ResetHTTPDate}
}

// Rate limit keys other than an expression ("=...").
const (
	// RateKeyCredential keys the bucket by credential.
	RateKeyCredential = "CREDENTIAL"
	// RateKeyHost keys the bucket by host.
	RateKeyHost = "HOST"
)

// PackageStatus is the state of an installed package.
type PackageStatus string

// Package statuses.
const (
	// StatusInstalled is a usable package.
	StatusInstalled PackageStatus = "INSTALLED"
	// StatusRetired is a package no flow may newly pin (ADR-0034 section 6).
	StatusRetired PackageStatus = "RETIRED"
)

// PackageStatuses returns every valid [PackageStatus].
func PackageStatuses() []PackageStatus { return []PackageStatus{StatusInstalled, StatusRetired} }
