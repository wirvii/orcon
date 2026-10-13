<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Connect Acme to Orcon

## Before you start

You need an Acme account with permission to create API keys. Any plan works.

## Create the credential in Acme

1. Sign in to Acme and open **Settings > API keys**.
2. Select **New key** and name it so it says it is used by Orcon.
3. Copy the key. Acme shows it only once.

## Scopes or permissions to grant

Grant only what the operations you use need:

| Operation | Scope or permission |
|---|---|
| contact.create, contact.update, contact.delete | `contacts:write` |
| contact.get, contact.getMany | `contacts:read` |
| message.send | `messages:send` |

## Redirect URL

Not needed: this connection uses an API key.

## Enter it in Orcon

1. In Orcon, open **Credentials** and select **New credential**.
2. Choose **Acme** and the credential type **Acme API key**.
3. Paste the key and save. Orcon stores it encrypted and never shows it again.

## Test the connection

Select **Test**. Orcon lists one contact; a success message means the key works.

## Troubleshooting

- **AuthInvalid**: the key was revoked or copied incompletely. Create a new one.
- **PermissionDenied**: a scope from the table above is missing. Edit the key in Acme and add it.
- **RateLimited**: Acme allows 10 requests per second per key. Orcon waits and retries automatically.
