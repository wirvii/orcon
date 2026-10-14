// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package envelope is the key hierarchy and the envelope encryption of the
// credential area (ADR-0042, SPEC-004 R-KEY). It is an area package: it
// imports no adapter and no database, and reaches storage only through the
// [KeyStore] port that it declares as the consumer.
//
// The hierarchy has three levels. The master key (MK) comes from a provider
// ([MasterKey]) and is never stored. Organization keys (OK) and the instance
// key (IK) are 256-bit random keys stored wrapped by the MK. Data keys (DEK)
// are 256-bit random keys, one per encrypted object, wrapped by the OK or IK
// version recorded with the ciphertext. Every ciphertext is AES-256-GCM with
// a 96-bit random nonce and additional authenticated data that binds it to
// its organization, object type, object id, version and field ([AAD]), so a
// ciphertext copied to another row or organization does not decrypt.
//
// At startup, before migrations or any write, [Verify] unwraps the master key
// canary and the canary of every active OK and IK. A process that cannot do so
// must exit: it must never write data under an unverified key.
//
// This package holds the logic only. The tables behind [KeyStore], the
// installation ceremony, online rotation and the per-execution DEK arrive with
// the following cut of SPEC-004 (SE3b). The master key providers are adapters
// implementing [MasterKey] in internal/adapter/kms: file and env read their key
// into memory and build a [LocalMasterKey]; providers backed by a cloud service
// arrive later.
package envelope
