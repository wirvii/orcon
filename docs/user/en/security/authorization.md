---
title: Authorization
description: The roles and permissions of Orcon, how custom roles work, and how execution data is kept apart from execution status.
sources:
  - ADR-0041
  - ADR-0038
  - ADR-0049
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Authorization

> This page describes behavior specified by ADR 0041 and ADR 0049 that is not implemented yet. The slices that deliver authorization remove this note.

Orcon denies everything that has not been granted. There are no explicit deny rules in version 1. A permission is checked at four levels, from the widest to the narrowest: the instance, the organization, the project and the resource.

## Roles

Predefined roles cannot be edited.

| Level | Role | What it can do |
| --- | --- | --- |
| Organization | Owner | Everything in the organization and in all its projects, including deleting the organization and transferring ownership. |
| Organization | Admin | The same as Owner, without deleting the organization or transferring ownership. |
| Organization | Member | See the organization, its members and roles, create projects (an organization can turn this off), and reference external secret stores. Access to projects comes only from project roles. |
| Project | Project Admin | Every permission in that project. |
| Project | Editor | Create, edit, publish, activate, roll back and run flows; work with executions except deleting them; resolve human tasks; manage the credentials and variables of the project. |
| Project | Operator | See flows and executions; activate flows; retry and cancel executions; resolve human tasks. Cannot edit and, by default, cannot see execution data. |
| Project | Viewer | Read-only: the project, flows, executions, credentials (metadata only) and variables. |

## Execution data is a separate permission

Seeing that an execution ran is not the same as seeing what it processed. The permission `execution:read` shows status, timing, attempts and sanitized errors. The permission `execution:read-data` is needed to see items, inputs and outputs, payloads, variable values, blobs, to load past data into the editor, and to export. Without it, API answers leave out the data and say so with `dataHidden: true`. Every view or export of protected data is audited.

The Operator role does not have `execution:read-data`. If your operators need it, create a custom role: for example, "Operator with data" is Operator plus `execution:read-data`.

## Custom roles

An organization can create roles from the catalog of permissions. Nobody can create, change or assign a role that includes a permission they do not hold themselves in that scope, so roles cannot be used to escalate. An instance can switch custom roles off, or remove permissions from all roles, but only to tighten.

## Groups and sharing

Roles can be assigned to users, to groups and to service accounts. A credential belongs to one project. The owner can share it with another project of the same organization, and the other project then gets the right to use it, and nothing more. Sharing never crosses organizations. The check runs again at every use.

## Instance permissions

Instance roles are separate from organization roles. `instanceOwner` and `instanceAdmin` hold the instance permissions: `instance:orgs`, `instance:keys`, `instance:connectors`, `instance:policy`, `instance:limits`, `instance:siem`, `instance:users`, `instance:backups` and `instance:maintenance`. These roles give no access to organization data. In a single-organization install, the first user receives `instanceOwner` and is the Owner of the organization.

## How changes take effect

Changing a role, a binding, a group, a share or an organization's policy takes effect across the cluster within one second. Sensitive operations check the database again within their own transaction.

## Actions from notifications

An email or webhook link never authorizes an action by itself. A person who acts on a human task from a notification must sign in to Orcon and hold the permission for that action in the project of the execution. Notifications carry identifiers, names and a link, not secret values and, by default, no execution data.
