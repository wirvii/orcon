<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Connect WhatsApp Cloud API to Orcon

## Before you start

You need a Meta Business account with a WhatsApp Business account and a phone number registered for the Cloud API. Messages you send to customers follow the WhatsApp Business Messaging Policy: the customer must have opted in, and a business-started conversation must use an approved template outside the 24-hour customer service window. Orcon does not enforce these rules; the flow that sends the message must.

## Create the credential in Meta

1. In Meta Business Settings, open **Users > System users** and create a system user with the **Admin** role.
2. Assign the WhatsApp Business account and the app to the system user, with the `whatsapp_business_messaging` and `whatsapp_business_management` permissions.
3. Select **Generate new token**, choose the app, and select the same two permissions. Copy the token. Meta shows it only once.
4. In WhatsApp Manager, open the phone number and copy its **Phone number ID** (digits only, not the phone number itself).

## Scopes or permissions to grant

| Operation | Permission |
|---|---|
| message.send, templateMessage.send, imageMessage.send, documentMessage.send, message.update | `whatsapp_business_messaging` |
| phoneNumber.get | `whatsapp_business_management` |

## Redirect URL

Not needed: this connection uses a system user token.

## Enter it in Orcon

1. In Orcon, open **Credentials** and select **New credential**.
2. Choose **WhatsApp Cloud API** and the credential type **System user access token**.
3. Paste the token and save. Orcon stores it encrypted and never shows it again.
4. In each node, fill **Phone number ID** with the value copied in step 4 above.

## Test the connection

Select **Test**. Orcon reads the phone number with its Phone number ID; the display number and the quality rating appear when the token and the ID are correct.

## Troubleshooting

- **AuthInvalid**: the token expired, was revoked, or was copied incompletely. Generate a new token for the system user.
- **PermissionDenied**: the system user does not have the phone number or the permission from the table above. Assign them in Meta Business Settings.
- **ValidationFailed**: Meta refused the request, for example a recipient outside the 24-hour window without a template, a missing template, or a bad media link. Read the message in the execution log.
- **RateLimited**: Meta asked Orcon to slow down. Orcon waits before the next request; a message is never resent automatically.
- **UNCERTAIN (suspended)**: the connection failed after the message may have been sent. Check the chat in WhatsApp Business as the execution log says, then choose whether the message was sent. Orcon does not resend it on its own.
