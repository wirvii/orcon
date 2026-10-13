// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineConfig, type UserConfig } from '@kubb/core';
import { pluginFaker } from '@kubb/plugin-faker';
import { pluginMsw } from '@kubb/plugin-msw';
import { pluginOas } from '@kubb/plugin-oas';
import { pluginTs } from '@kubb/plugin-ts';
import { pluginZod } from '@kubb/plugin-zod';

/** Every generated file starts with the licence header (orcon-web DP-6) and says it is generated. */
export const banner = '// SPDX-FileCopyrightText: The Orcon Authors\n// SPDX-License-Identifier: Apache-2.0\n//\n// Generated from the OpenAPI document by Kubb. Do not edit; run `pnpm generate`.\n';

export interface ContractOptions {
  /** The OpenAPI 3.1 document. */
  input: string;
  /** Where the generated folders `zod`, `faker` and `msw` go. */
  output: string;
}

/**
 * One generation run: Zod 4 schemas, example data (seeded, so output is stable) and Mock
 * Service Worker handlers that answer with that data. No second HTTP client is generated:
 * `@orcon/sdk` (openapi-fetch) is the only one (orcon-web DT-1).
 */
export function contractConfig({ input, output }: ContractOptions): UserConfig {
  return {
    root: '.',
    input: { path: input },
    // Each plugin writes the licence banner (below); the default Kubb banner is off. Imports carry no extension.
    output: { path: output, clean: true, defaultBanner: false, barrelType: false, format: false, lint: false, extension: { '.ts': '' } },
    plugins: [
      pluginOas({ validate: true, generators: [] }),
      // The TypeScript types are a required companion of the Zod, faker and msw plugins; they are
      // generated next to the schemas and are the types the generated handlers and data use.
      pluginTs({ output: { path: './types', barrelType: 'named', banner }, enumType: 'asConst' }),
      pluginZod({
        output: { path: './zod', barrelType: 'named', banner },
        version: '4',
        importPath: 'zod',
        typed: true,
        inferred: true,
        dateType: 'stringOffset',
        unknownType: 'unknown',
      }),
      pluginFaker({
        output: { path: './faker', barrelType: 'named', banner },
        seed: [100],
        dateType: 'string',
        unknownType: 'unknown',
      }),
      pluginMsw({
        output: { path: './msw', barrelType: 'named', banner },
        parser: 'faker',
        handlers: true,
      }),
    ],
  };
}

// `scripts/generate.mjs` sets the two variables to generate from another document or into another folder
// (the pipeline test does); without them this is the real contract.
export default defineConfig(
  contractConfig({
    input: process.env['ORCON_CONTRACT_INPUT'] ?? '../../openapi/v1/openapi.yaml',
    output: process.env['ORCON_CONTRACT_OUTPUT'] ?? './src/gen',
  }),
);
