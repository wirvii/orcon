// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package audit

// ChainHashForTest exposes the event hash to the external tests, which recompute
// a chain the way an attacker without the key would.
func ChainHashForTest(prev [32]byte, sequence int64, body []byte) [32]byte {
	return chainHash(prev, sequence, body)
}
