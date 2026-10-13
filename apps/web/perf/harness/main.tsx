// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import './styles.css';

import { Providers, WorkflowCanvas } from '@orcon/ui';
import { StrictMode, useEffect, useMemo } from 'react';
import { createRoot } from 'react-dom/client';

import { markEditorReady } from '../../src/app/perf/marks';

import { referenceGraph } from './reference-flow';

/**
 * Mounts the design system's flow canvas with the 500-node reference flow and nothing else, so
 * the measurement sees the canvas and not the app around it (orcon-web PF-4). The canvas marks
 * `data-fitted` on its container once React Flow reports the nodes initialized and the first fit
 * is applied; that is the moment the editor is ready.
 */
function Harness() {
  const graph = useMemo(referenceGraph, []);

  useEffect(() => {
    const observer = new MutationObserver(() => {
      if (!document.querySelector('[data-fitted="true"]')) return;
      observer.disconnect();
      markEditorReady();
    });
    observer.observe(document.body, { attributes: true, attributeFilter: ['data-fitted'], subtree: true });
    return () => {
      observer.disconnect();
    };
  }, []);

  return (
    <Providers theme="light" density="comfortable" locale="en">
      <div className="bg-app h-screen w-screen">
        <WorkflowCanvas defaultGraph={graph} tools={false} />
      </div>
    </Providers>
  );
}

const root = document.getElementById('root');
if (!root) throw new Error('The harness page has no #root element.');
createRoot(root).render(
  <StrictMode>
    <Harness />
  </StrictMode>,
);
