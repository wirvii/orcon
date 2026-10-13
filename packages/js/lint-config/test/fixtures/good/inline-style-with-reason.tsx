// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export const View = ({ x }: { x: number }) => (
  // eslint-disable-next-line orcon/no-inline-style -- canvas node position is computed at runtime
  <div style={{ transform: `translateX(${x}px)` }} />
);
