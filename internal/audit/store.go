// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"time"

	"github.com/wirvii/orcon/internal/platform/id"
)

// MACPurpose is the purpose the chain MAC key is derived for (HKDF, see envelope.Key.Subkey).
// A change of this string makes every existing block fail verification.
const MACPurpose = "orcon/audit@1 block-mac"

// Store is what the sealer and the verifier need from storage. Each adapter
// implements it for both chain kinds; the organization chain runs under its own
// organization scope, the instance chain in the instance tables (orcon-db IN-12).
type Store interface {
	// PendingChains returns at most limit chains that have pending entries.
	PendingChains(ctx context.Context, limit int) ([]Chain, error)
	// ListPending returns at most limit pending entries of the chain, ordered by
	// occurrence time and then identity, the order in which the sealer seals them.
	ListPending(ctx context.Context, chain Chain, limit int) ([]Entry, error)
	// Tail returns the last sealed sequence number and hash of the chain; the zero
	// Tail when nothing is sealed yet.
	Tail(ctx context.Context, chain Chain) (Tail, error)
	// CommitBlock inserts the sealed events and their block and deletes the pending
	// entries the batch consumed, in one transaction. It fails with [ErrTailMoved] unless
	// the chain still ends where expect says.
	CommitBlock(ctx context.Context, chain Chain, expect Tail, batch Batch) error
	// ListSealed returns at most limit sealed events of the chain with a sequence
	// number greater than after, ascending.
	ListSealed(ctx context.Context, chain Chain, after int64, limit int) ([]Sealed, error)
	// ListBlocks returns at most limit blocks of the chain whose last sequence number
	// is greater than after, ascending.
	ListBlocks(ctx context.Context, chain Chain, after int64, limit int) ([]Block, error)
}

// KeySource gives the MAC keys of a chain. The composition root implements it over
// the keyring: the organization key of the chain, or the instance key for the instance chain.
type KeySource interface {
	// ActiveMACKey returns the key version the chain is sealed with and its MAC key.
	ActiveMACKey(ctx context.Context, chain Chain) (version int64, key []byte, err error)
	// MACKey returns the MAC key of one version, so blocks sealed before a rotation still verify.
	MACKey(ctx context.Context, chain Chain, version int64) ([]byte, error)
}

// Tail is the end of a chain: the last sequence number and the hash of that event.
// Sequence zero means the chain is empty, and its hash is then all zeros.
type Tail struct {
	Sequence int64
	Hash     [32]byte
}

// Sealed is an event as stored: its position, the link to the previous event and the entry.
type Sealed struct {
	Sequence     int64
	PreviousHash [32]byte
	Hash         [32]byte
	Entry        Entry
}

// Block closes a batch of sealed events. The MAC covers the chain label, the last
// sequence number and the head hash, under the key of KeyVersion.
type Block struct {
	ID                  id.UUID
	Chain               Chain
	FirstSequenceNumber int64
	LastSequenceNumber  int64
	HeadHash            [32]byte
	MAC                 [32]byte
	KeyVersion          int64
	SealedAt            time.Time
}

// Batch is what one commit stores: the block and the events it closes. PendingIDs are
// the pending entries the batch consumes; they are deleted in the same transaction.
type Batch struct {
	Block      Block
	Events     []Sealed
	PendingIDs []id.UUID
}

// blockMAC is HMAC-SHA-256 over the label of the chain, the last sequence number as
// eight big-endian bytes and the head hash, keyed with the chain's MAC key. The
// label and a fixed prefix keep a MAC of one chain or one purpose from verifying as another.
func blockMAC(key []byte, chain Chain, last int64, head [32]byte) [32]byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte("orcon/audit@1 block\x00"))
	m.Write([]byte(chain.Label()))
	m.Write([]byte{0})
	seq := sequenceBytes(last)
	m.Write(seq[:])
	m.Write(head[:])
	var out [32]byte
	copy(out[:], m.Sum(nil))
	return out
}
