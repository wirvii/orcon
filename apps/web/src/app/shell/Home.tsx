// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { WelcomeContent } from '@orcon/ui';
import { createFileRoute } from '@tanstack/react-router';
import { useEffect } from 'react';

import { markFirstUsefulScreen } from '../perf/marks';

function Home() {
  useEffect(() => {
    markFirstUsefulScreen('/');
  }, []);
  return <WelcomeContent />;
}

export const Route = createFileRoute('/')({ component: Home });
