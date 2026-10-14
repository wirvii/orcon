#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Downloads the five OMG BPMN 2.0 schema files into a directory and verifies
# their SHA-256 digests. The files are not vendored: the repository carries
# only the digests. TestRoundTripXSD in internal/adapter/codec/bpmn validates
# the exported XML against them when ORCON_BPMN_XSD_DIR names the directory.
#
# Usage: sh scripts/fetch-bpmn-xsd.sh <directory>
set -eu

dir=${1:?usage: fetch-bpmn-xsd.sh <directory>}
base=https://www.omg.org/spec/BPMN/20100501
mkdir -p "$dir"

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

fetch() {
  name=$1
  want=$2
  curl -fsSL --retry 4 --retry-delay 3 --retry-all-errors -o "$dir/$name" "$base/$name"
  got=$(sha256 "$dir/$name")
  if [ "$got" != "$want" ]; then
    echo "fetch-bpmn-xsd: $name has digest $got, want $want" >&2
    exit 1
  fi
}

fetch Semantic.xsd c4318842f7d2bbc262d7954c9452c501db16f0868eac0b8732ec5d7fb384d9a7
fetch BPMN20.xsd a07c159cb0594573dd7c97b1370dd116112378f377e43c89a8bf512ac5030705
fetch BPMNDI.xsd f0dff1cd559d1514d8ebfc8c646f58402bcaced27ec22e2aa6456c2dcc80b038
fetch DC.xsd a2f90e5ad9bb48c6915e4e034b4e27ac838264a1d4f27bfc70dbdfc69351312d
fetch DI.xsd 8220b179c175572df74e08a51bffabe957867962035cee7b5fee0b6acb4c4498
echo "fetch-bpmn-xsd: ok ($dir)"
