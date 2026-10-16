// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { buildTimeline } from './steps';
import type { Execution, NodeExecution } from './types';

const run: Execution = {
  id: 'r1',
  flowId: 'f1',
  projectId: 'p1',
  flowVersion: 2,
  mode: 'MANUAL',
  status: 'FAILED',
  createdAt: '2026-10-14T09:00:00Z',
  startedAt: '2026-10-14T09:00:00Z',
  finishedAt: '2026-10-14T09:00:04Z',
  dataHidden: false,
};

const node = (id: string, nodeId: string, patch: Partial<NodeExecution> = {}): NodeExecution => ({
  id,
  executionId: 'r1',
  nodeId,
  status: 'SUCCEEDED',
  attemptCount: 1,
  startedAt: '2026-10-14T09:00:00Z',
  finishedAt: '2026-10-14T09:00:01Z',
  ...patch,
});

const document = [
  { id: 'a', name: 'First' },
  { id: 'b', name: undefined },
  { id: 'c', name: 'Third' },
];
const now = Date.parse('2026-10-14T09:10:00Z');

describe('buildTimeline', () => {
  it('places each step on the time of the run and lists the steps that never ran last', () => {
    const { steps, totalMs } = buildTimeline(
      run,
      [node('1', 'b', { startedAt: '2026-10-14T09:00:01Z', finishedAt: '2026-10-14T09:00:03Z' }), node('2', 'a')],
      document,
      now,
    );
    expect(totalMs).toBe(4000);
    expect(steps.map((step) => [step.id, step.name, step.ran, step.startMs, step.durationMs])).toEqual([
      ['a', 'First', true, 0, 1000],
      // A node without a name is shown by its id.
      ['b', 'b', true, 1000, 2000],
      ['c', 'Third', false, undefined, undefined],
    ]);
    expect(steps[2]?.status).toBe('NOT_RUN');
  });

  it('does not call a step "did not run" while the run goes on', () => {
    const running: Execution = { ...run, status: 'RUNNING', finishedAt: undefined };
    const { steps, totalMs } = buildTimeline(running, [node('1', 'a', { status: 'RUNNING', finishedAt: undefined })], document, now);
    expect(steps.map((step) => step.id)).toEqual(['a']);
    // The run is as long as it has been going.
    expect(totalMs).toBe(600_000);
  });

  it('names steps by their id and lists only the ones that ran when the version is not known', () => {
    const { steps } = buildTimeline(run, [node('1', 'a')], undefined, now);
    expect(steps.map((step) => [step.id, step.name])).toEqual([['a', 'a']]);
  });

  it('puts the runs of one node together: the strongest status wins and the failures are counted', () => {
    const { steps } = buildTimeline(
      run,
      [
        node('1', 'a'),
        node('2', 'a', { status: 'FAILED', attemptCount: 3, startedAt: '2026-10-14T09:00:01Z', finishedAt: '2026-10-14T09:00:02Z', failure: { code: 'boom', detail: 'It broke' } }),
        node('3', 'a', { startedAt: '2026-10-14T09:00:02Z', finishedAt: '2026-10-14T09:00:03Z' }),
      ],
      document,
      now,
    );
    const a = steps[0];
    expect(a).toMatchObject({ id: 'a', status: 'FAILED', runs: 3, failedRuns: 1, attempts: 5, startMs: 0, durationMs: 3000 });
    expect(a?.failure).toMatchObject({ code: 'boom', detail: 'It broke' });
    expect(a?.nodeExecutionIds).toEqual(['1', '2', '3']);
  });

  it('gives the failed step the plain title of the run when the run names that step', () => {
    const failed: Execution = { ...run, failure: { code: 'node-failed', title: 'The step failed', detail: 'Over the limit', nodeId: 'a' } };
    const { steps } = buildTimeline(failed, [node('1', 'a', { status: 'FAILED', failure: { code: 'node-failed', detail: 'Own detail' } })], document, now);
    expect(steps[0]?.failure).toMatchObject({ code: 'node-failed', title: 'The step failed', detail: 'Own detail' });
  });

  it('is empty and zero long for a run that has not started', () => {
    const queued: Execution = { ...run, status: 'QUEUED', startedAt: undefined, finishedAt: undefined };
    expect(buildTimeline(queued, [], document, now)).toEqual({ steps: [], totalMs: 0 });
  });
});
