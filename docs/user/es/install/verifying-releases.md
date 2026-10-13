---
title: Verificar publicaciones
description: Comprueba que un binario, una imagen o una etiqueta de versión fue construida por el flujo de publicación de Orcon, sin confiar en los servidores de nadie.
sources:
  - ADR-0081
  - ADR-0083
sourceHash: 1cc4d1413ffaa2e86f1a5de9aee3be7af6376989d1c5ef0a24bb1eef40056aed
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Verificar publicaciones

Toda versión de Orcon se puede comprobar sin confiar en los servidores del proyecto ni en sus mantenedores. El flujo de publicación firma cada artefacto con un certificado de vida corta ligado a la identidad del flujo, y publica una procedencia de construcción que dice qué repositorio, qué commit y qué flujo lo produjeron. Esta página muestra cómo comprobar tú mismo una versión. La decisión que hay detrás es el ADR 0081. Los instaladores de una línea ejecutan por ti las comprobaciones de firma y de suma; consulta [Instalar el binario único](binary.md).

En las órdenes de abajo, sustituye `0.1.0` por la versión que descargaste.

## Qué está firmado

| Artefacto | Cómo está protegido | Cómo lo compruebas |
|---|---|---|
| Etiqueta de versión `vX.Y.Z` | Etiqueta anotada firmada con gitsign por el flujo de publicación | `gitsign verify-tag` |
| `checksums.txt` | Paquete de firma de cosign sin claves, `checksums.txt.sigstore.json`; lista el SHA-256 de cada archivo comprimido y del SBOM | `cosign verify-blob`, y luego `sha256sum -c` |
| Archivos comprimidos `orcon_X.Y.Z_<os>_<arch>.tar.gz` (`.zip` en Windows) | Cubiertos por `checksums.txt`, más la procedencia SLSA v1 Build Level 3 `orcon_X.Y.Z.intoto.jsonl` | `slsa-verifier verify-artifact` |
| SBOM (CycloneDX, módulos de Go) | Listado en `checksums.txt`; adjunto a la imagen como atestación firmada | `sha256sum -c`, `cosign verify-attestation` |
| Imagen `ghcr.io/wirvii/orcon:X.Y.Z` (linux/amd64, linux/arm64) | Firma de cosign sin claves sobre el digest de la imagen, atestación CycloneDX, procedencia SLSA v1 Build Level 3 | `cosign verify`, `slsa-verifier verify-image` |

No existe ninguna clave de firma de larga vida: no hay clave pública que descargar. Cada firma se comprueba contra la identidad del flujo de GitHub Actions que la hizo y el emisor OpenID Connect de GitHub, registrados en el registro público de transparencia de Sigstore.

## Herramientas

Las tres herramientas son proyectos de código abierto con licencia Apache-2.0:

- cosign — <https://docs.sigstore.dev/cosign/system_config/installation/>
- slsa-verifier — <https://github.com/slsa-framework/slsa-verifier#installation>
- gitsign — <https://docs.sigstore.dev/cosign/signing/gitsign/> (solo hace falta para comprobar la etiqueta)

## Verificar un archivo descargado

Descarga en un mismo directorio, desde la página de la versión, el archivo comprimido de tu plataforma, `checksums.txt`, `checksums.txt.sigstore.json` y `orcon_X.Y.Z.intoto.jsonl`, y ejecuta:

```sh
VERSION=0.1.0
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/wirvii/orcon/.github/workflows/release.yml@refs/tags/v${VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --ignore-missing -c checksums.txt
slsa-verifier verify-artifact "orcon_${VERSION}_linux_amd64.tar.gz" \
  --provenance-path "orcon_${VERSION}.intoto.jsonl" \
  --source-uri github.com/wirvii/orcon --source-tag "v${VERSION}"
```

1. `cosign verify-blob` demuestra que `checksums.txt` lo firmó el flujo de publicación de Orcon para esa etiqueta.
2. `sha256sum` demuestra que tu archivo coincide con la suma firmada. En macOS usa `shasum -a 256 --ignore-missing -c checksums.txt`.
3. `slsa-verifier verify-artifact` demuestra que el archivo lo construyó el constructor de confianza a partir de `wirvii/orcon` en esa etiqueta. Sustituye `linux_amd64` por tu plataforma.

Cada orden debe terminar sin error. Si alguna falla, no uses el archivo.

## Verificar la imagen de contenedor

```sh
VERSION=0.1.0
IDENTITY="https://github.com/wirvii/orcon/.github/workflows/release.yml@refs/tags/v${VERSION}"
ISSUER=https://token.actions.githubusercontent.com
cosign verify "ghcr.io/wirvii/orcon:${VERSION}" \
  --certificate-identity "${IDENTITY}" \
  --certificate-oidc-issuer "${ISSUER}"
cosign verify-attestation --type cyclonedx "ghcr.io/wirvii/orcon:${VERSION}" \
  --certificate-identity "${IDENTITY}" \
  --certificate-oidc-issuer "${ISSUER}"
```

`cosign verify` imprime el digest de la imagen (`sha256:…`). Usa ese digest, no la etiqueta, para comprobar la procedencia y para descargar la imagen:

```sh
slsa-verifier verify-image "ghcr.io/wirvii/orcon@sha256:<digest>" \
  --source-uri github.com/wirvii/orcon --source-tag "v${VERSION}"
```

## Verificar la etiqueta

```sh
VERSION=0.1.0
git fetch --tags
gitsign verify-tag "v${VERSION}" \
  --certificate-identity https://github.com/wirvii/orcon/.github/workflows/release-please.yml@refs/heads/main \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

GitHub no muestra las firmas de Sigstore como «Verified» en su interfaz web. Esa etiqueta no forma parte de esta comprobación: la orden de arriba sí.

## Leer el SBOM

El SBOM es un archivo JSON CycloneDX que lista los módulos de Go compilados dentro de `orcon`, con sus versiones y licencias. Después de comprobarlo contra `checksums.txt`, ábrelo con cualquier herramienta CycloneDX, o extrae el mismo documento de la atestación de la imagen:

```sh
cosign verify-attestation --type cyclonedx "ghcr.io/wirvii/orcon:${VERSION}" \
  --certificate-identity "${IDENTITY}" \
  --certificate-oidc-issuer "${ISSUER}" \
  | jq -r '.payload' | base64 -d | jq '.predicate'
```

## Informar de un problema

Si una comprobación falla para una versión oficial, o encuentras una firma que no debería existir, no abras una incidencia pública. Infórmalo en privado como se describe en [Informar de vulnerabilidades](../security/reporting-vulnerabilities.md).
