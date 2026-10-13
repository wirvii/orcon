// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export function savedName(): string | null {
  return window.localStorage.getItem('name');
}
