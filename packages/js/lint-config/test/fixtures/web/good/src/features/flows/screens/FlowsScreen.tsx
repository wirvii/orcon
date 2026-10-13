// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query';

import { flowQueries } from '../api/queries';
import { FlowRow } from '../components/FlowRow';

export function FlowsScreen({ org, id }: { org: string; id: string }) {
  const flow = useQuery(flowQueries.detail(org, id));
  if (!flow.data) return null;
  return (
    <ul>
      <FlowRow flow={flow.data} />
    </ul>
  );
}
