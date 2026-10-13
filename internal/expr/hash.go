// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"crypto/md5"  //nolint:gosec // md5 is the documented default algorithm of the n8n-compatible hash() method, not a security control.
	"crypto/sha1" //nolint:gosec // sha1 is an algorithm of the n8n-compatible hash() method, not a security control.
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// hashText answers OpHash: "<algorithm>\t<text>" becomes the hex digest of the
// text. The option base64 is the only one of the method that is not a digest,
// and an unknown algorithm answers nothing.
func hashText(payload string) []byte {
	algo, text, ok := strings.Cut(payload, "\t")
	if !ok {
		return nil
	}
	data := []byte(text)
	var sum []byte
	switch algo {
	case "md5":
		s := md5.Sum(data) //nolint:gosec // see the import.
		sum = s[:]
	case "sha1":
		s := sha1.Sum(data) //nolint:gosec // see the import.
		sum = s[:]
	case "sha224":
		s := sha256.Sum224(data)
		sum = s[:]
	case "sha256":
		s := sha256.Sum256(data)
		sum = s[:]
	case "sha384":
		s := sha512.Sum384(data)
		sum = s[:]
	case "sha512":
		s := sha512.Sum512(data)
		sum = s[:]
	case "sha3":
		s := sha3.Sum256(data)
		sum = s[:]
	case "base64":
		return []byte(base64.StdEncoding.EncodeToString(data))
	default:
		return nil
	}
	return []byte(hex.EncodeToString(sum))
}
