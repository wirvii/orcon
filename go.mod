// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

module github.com/wirvii/orcon

go 1.25.0

toolchain go1.26.9

ignore node_modules

require go.uber.org/goleak v1.3.0

require (
	github.com/dop251/goja v0.0.0-20261001165353-dc0ad5d0dc92
	github.com/google/go-cmp v0.7.0
	github.com/tetratelabs/wazero v1.11.0
)

require (
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)
