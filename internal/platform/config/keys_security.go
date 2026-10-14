// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

// keysSecurity declares the keys of the master key (SPEC-004, ADR 0042), the
// recovery key and the first-run owner account. Provider settings for the
// master key are all under keys.master.<provider>.
func keysSecurity() []Key {
	return []Key{
		{
			Path: "keys.master.provider", Kind: KindEnum, Enum: []string{"file", "env", "awskms", "gcpkms", "azurekv", "vault-transit"}, Default: "file",
			Description: text(
				"Where the master key lives: a file, the environment (not recommended), or a key management service.",
				"Dónde vive la clave maestra: un archivo, el entorno (no recomendado) o un servicio de gestión de claves."),
		},
		{
			Path: "keys.master.file.path", Kind: KindPath, DefaultFunc: underDataDir("keys", "master.key"), DefaultDoc: docUnderDataDir("keys/master.key"),
			Description: text(
				"Path of the master key file, for the file provider.",
				"Ruta del archivo de la clave maestra, para el proveedor file."),
		},
		{
			Path: "keys.master.key", Kind: KindString, Secret: true, Env: "ORCON_MASTER_KEY", Validate: validateBase64,
			Description: text(
				"The master key, base64 encoded, for the env provider. Prefer keys.master.keyFile.",
				"La clave maestra en base64, para el proveedor env. Prefiera keys.master.keyFile."),
		},
		{
			Path: "keys.master.backupAcknowledgement", Kind: KindString, Env: "ORCON_MASTER_KEY_BACKUP_ACK", Reload: ReloadFirstStart,
			Description: text(
				"Fingerprint of the master key, given to confirm that its backup was made. It completes the key ceremony without the web wizard. A wrong fingerprint stops the start.",
				"Huella de la clave maestra, que confirma que se hizo su respaldo. Completa la ceremonia de la clave sin el asistente web. Una huella equivocada impide arrancar."),
		},
		{
			Path: "keys.master.awskms.keyArn", Kind: KindString,
			Description: text("ARN of the AWS KMS key.", "ARN de la clave de AWS KMS."),
		},
		{
			Path: "keys.master.gcpkms.keyName", Kind: KindString,
			Description: text("Resource name of the Google Cloud KMS key.", "Nombre del recurso de la clave de Google Cloud KMS."),
		},
		{
			Path: "keys.master.azurekv.keyId", Kind: KindString,
			Description: text("Identifier of the Azure Key Vault key.", "Identificador de la clave de Azure Key Vault."),
		},
		{
			Path: "keys.master.vaultTransit.address", Kind: KindURL,
			Description: text("Address of the Vault server.", "Dirección del servidor Vault."),
		},
		{
			Path: "keys.master.vaultTransit.mount", Kind: KindString,
			Description: text("Mount path of the Vault transit engine.", "Ruta de montaje del motor transit de Vault."),
		},
		{
			Path: "keys.master.vaultTransit.keyName", Kind: KindString,
			Description: text("Name of the Vault transit key.", "Nombre de la clave transit de Vault."),
		},
		{
			Path: "keys.master.vaultTransit.token", Kind: KindString, Secret: true,
			Description: text("Vault token used to call the transit engine.", "Token de Vault que se usa para llamar al motor transit."),
		},
		{
			Path: "keys.recoveryPublicKey", Kind: KindString, Validate: validateAgeRecipient,
			Description: text(
				"Public key (age1...) of an offline recovery key. When set, every backup can also be decrypted by its private half.",
				"Clave pública (age1...) de una clave de recuperación guardada fuera de línea. Si se fija, cada respaldo también puede descifrarse con su mitad privada."),
		},
		{
			Path: "setup.owner.email", Kind: KindString, Validate: validateEmail, Reload: ReloadFirstStart, Roles: rolesServing,
			Description: text(
				"Email of the owner account created at the first start, without the web wizard. Ignored once the instance is set up.",
				"Correo de la cuenta propietaria que se crea en el primer arranque, sin el asistente web. Se ignora cuando la instancia ya está preparada."),
		},
		{
			Path: "setup.owner.name", Kind: KindString, Reload: ReloadFirstStart, Roles: rolesServing,
			Description: text(
				"Display name of the owner account created at the first start.",
				"Nombre visible de la cuenta propietaria que se crea en el primer arranque."),
		},
		{
			Path: "setup.owner.password", Kind: KindString, Secret: true, Reload: ReloadFirstStart, Roles: rolesServing,
			Description: text(
				"Password of the owner account created at the first start. Remove it once the instance is set up.",
				"Contraseña de la cuenta propietaria que se crea en el primer arranque. Quítela cuando la instancia ya esté preparada."),
		},
	}
}
