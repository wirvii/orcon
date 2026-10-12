<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0042 — Key hierarchy, envelope encryption, master key providers, rotation and backup ceremony

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12 (owner answers; consistency with ADR-0077)

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
S2: envelope encryption, per-credential data keys wrapped by a master key never stored in the database; selectable providers; online rotation without touching ciphertext; forced backup at install; key verification at startup. S3: a per-organization level in the hierarchy.

## Decision

### 1. Hierarchy
- **Master key (MK)**: from a provider; never in the database.
- **Organization keys (OK)**: 256-bit, one active version per organization; stored wrapped by the MK in `org_key(org_id, id, version_number, wrapped_key, master_key_reference, status: active|retiring|retired|destroyed, created_at)`, unique `(org_id, version_number)`.
- **Instance key (IK)**: same shape, for instance-level secrets (TOTP seeds, instance configuration secrets), in the instance table `instance_key(id, version_number, wrapped_key, master_key_reference, status, created_at)`.
- **Data keys (DEK)**: 256-bit, per credential version, per execution (ADR-0017), per sensitive configuration object; wrapped by the OK (or IK) version recorded with the ciphertext.
- Algorithm AES-256-GCM with 96-bit random nonces; DEKs are single-purpose, so nonce limits are never approached. Additional authenticated data: `orcon:v1|org_id|object_type|object_id|version|field`.
- Ciphertext envelope: `{v:1, kid:{level, org_id, version}, nonce, ct}`.

### 2. Providers
- `file`: path to a 32-byte key (raw or base64); refuses to start if group/other-readable.
- `env`: `ORCON_MASTER_KEY` (base64); startup warning.
- `awskms`: key ARN; wrap/unwrap via KMS Encrypt/Decrypt (AWS SDK v2, Apache-2.0).
- `gcpkms`: key resource name; Encrypt/Decrypt (Apache-2.0 client).
- `azurekv`: key identifier; wrapKey/unwrapKey with RSA-OAEP-256 or AES-KW on Managed HSM (`azkeys`, MIT).
- `vault-transit`: mount + key name; encrypt/decrypt via Vault/OpenBao REST (own client).
With KMS providers the MK never leaves the KMS; only OK/IK are unwrapped and held in memory (best-effort `mlock`), re-unwrapped hourly so that disabling the KMS key takes effect within one hour. Ambient cloud identity is instance-level and allowed for the MK provider; organizations may use it only through an explicit, audited per-organization grant by an instance admin (ADR-0044 §2; owner decision, SPEC-004 Q14).

### 3. Canaries and startup verification
Canaries store an AEAD encryption of a fixed constant: `instance_key_canary(id, level: master|instance, version_number, wrapped_probe)` for the MK and the IK, and `org_key_canary(org_id, id, version_number, wrapped_probe)` per organization key version (organization data never lives in a table with an optional `org_id`, ADR-0077 §5). At startup, before migrations or any write, the process unwraps the MK canary and each active OK/IK and decrypts their probes; on failure it exits with the provider, key reference and expected fingerprint (`SHA-256(MK)` truncated, or the KMS key reference). A process never writes data under an unverified key. Reading every organization's canary at startup is an engine-plane function (ADR-0047 §2).

### 4. Rotation (online)
- **MK rotation** (`orcon key rotate-master --to <provider config>`): re-wraps every OK/IK version under the new MK in batches, recording `master_key_reference`; both MKs are configured during rotation; ciphertext is untouched; resumable after crash (idempotent per row); completion verified by canaries; old MK removable after completion.
- **OK rotation** (`orcon key rotate-org <org>` or scheduled, default yearly): creates version n+1 (new writes use it), a background job re-wraps DEKs in batches of ≤ 1000 rows per transaction, resumable; version n retired when no DEK references it.
- Rotations hold no long transactions, pause under load, and are audited.

### 5. Installation ceremony
First start without keys: the setup wizard (or `orcon key init`) generates the MK (file provider) or validates the KMS key; produces a recovery kit (key material or KMS reference, fingerprint, restore instructions, explicit warning that loss is irreversible) that the operator must download; requires the operator to type the last 8 characters of the fingerprint. Headless installs must set `ORCON_MASTER_KEY_BACKUP_ACK=<fingerprint>` (automated tests set it too). KMS: warns if deletion protection or key rotation settings are absent. Acknowledgement is recorded (`key.backup_acknowledged`). There is no skip flag (owner decision, SPEC-004 Q6).

### 6. Optional recovery key
`keys.recoveryPublicKey` (X25519, age format, `filippo.io/age` BSD-3): when set, every OK version is additionally wrapped to it and stored in `org_key_recovery(org_id, id, org_key_id, recovery_ciphertext)`; recovery uses the offline private key with `orcon key recover`. Offered, optional and disabled by default (owner decision, SPEC-004 Q7).

### 7. Destruction
Deleting an organization (after grace) destroys all its OK versions (rows overwritten and deleted, recovery wraps deleted); its credentials and encrypted working state become unreadable even in old backups unless the backup is restored together with an MK that can unwrap a backup copy of the OK — documented.

## Consequences
MK rotation touches tens of rows; organization deletion is cryptographic; wrong-key startups are impossible.

## Alternatives considered
Single-level (MK → DEK): rotation touches every credential and no per-organization shredding (rejected); encrypting the whole database (rejected: delegates to infrastructure, no per-organization keys); fetching every DEK through the KMS (rejected: latency and cost per dispatch); a skip flag for ephemeral environments (rejected: it would be used in production); one canary table with an optional `org_id` (rejected by ADR-0077).
