<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Connect <Service> to Orcon

<!-- Copy to docs/connection-guide.en.md in the connector folder, replace every
<...> placeholder and keep every heading: the admission lint requires them
(DC-4). Delete "Redirect URL" content only when the service does not use
OAuth; keep the heading and write "Not needed: this connection uses an API key." -->

## Before you start

You need a <Service> account with permission to <create API keys | register applications>. <Say whether a paid plan is needed.>

## Create the credential in <Service>

1. Sign in to <Service> and open <exact menu path>.
2. Select <button name> and give the key or application a name that says it is used by Orcon.
3. Copy the <key | client ID and client secret>. <Service> shows it only once.

## Scopes or permissions to grant

Grant only what the operations you use need:

| Operation | Scope or permission |
|---|---|
| <resource.verb> | <scope> |

## Redirect URL

Enter `https://<your-orcon-host>/oauth/callback` as the redirect URL of the application.

## Enter it in Orcon

1. In Orcon, open **Credentials** and select **New credential**.
2. Choose **<Service>** and the credential type <type display name>.
3. Paste the values from the previous steps and save. Orcon stores them encrypted and never shows them again.

## Test the connection

Select **Test**. Orcon runs <test operation display name>; a success message means the credential works.

## Troubleshooting

- **AuthInvalid**: the key was revoked or copied incompletely. Create a new one.
- **PermissionDenied**: a scope from the table above is missing. Add it and authorize again.
- <Other frequent problem and what to do.>
