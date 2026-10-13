// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { existsSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';

import license, { type Dependency } from 'rollup-plugin-license';

/** What `.vite/bundled-modules.json` holds for one package that is bundled into the application. */
export interface BundledModule {
  name: string;
  version: string;
  license: string;
}

/** Packages whose files reach the build as assets, not as JavaScript modules: the self-hosted fonts (ADR 0074). */
const assetPackages = ['@fontsource-variable/inter', '@fontsource-variable/jetbrains-mono'];

interface PackageFile {
  name: string;
  version: string;
  license?: string;
}

/** The notice of a package that ships as assets: its name, license and the text of its LICENSE file. */
function assetNotice(packageName: string, resolveFrom: string): string {
  const require = createRequire(join(resolveFrom, 'package.json'));
  const manifestPath = require.resolve(`${packageName}/package.json`);
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8')) as PackageFile;
  const licensePath = ['LICENSE', 'LICENSE.md', 'LICENSE.txt'].map((name) => join(dirname(manifestPath), name)).find((path) => existsSync(path));
  const text = licensePath ? readFileSync(licensePath, 'utf8').trim() : '(the package ships no license file)';
  return `${manifest.name}@${manifest.version} (${manifest.license ?? 'unknown license'}), shipped as font files\n\n${wrapLines(text)}`;
}

/** Wraps every line longer than `width` at spaces, so a notice stays readable in a narrow viewer, and a license text that names a font file next to a URL never reads like a request for that font. */
export function wrapLines(text: string, width = 56): string {
  return text
    .split('\n')
    .map((line) => {
      if (line.length <= width) return line;
      const out: string[] = [];
      let current = '';
      for (const word of line.split(' ')) {
        if (current !== '' && current.length + 1 + word.length > width) {
          out.push(current);
          current = word;
        } else {
          current = current === '' ? word : `${current} ${word}`;
        }
      }
      out.push(current);
      return out.join('\n');
    })
    .join('\n');
}

function bundled(dependencies: readonly Dependency[]): BundledModule[] {
  return dependencies
    .flatMap((dependency) => (dependency.name === null ? [] : [{ name: dependency.name, version: dependency.version ?? '', license: dependency.license ?? '' }]))
    .sort((a, b) => a.name.localeCompare(b.name) || a.version.localeCompare(b.version));
}

/**
 * Collects the packages Rollup really bundles and writes two files into the build:
 *
 * - `third-party-notices.txt`: each package with its license text, plus the font packages that
 *   ship as assets. It is embedded in the binary, as ADR 0074 requires (orcon-web DP-5).
 * - `.vite/bundled-modules.json`: the bundled packages with their licenses, for `pnpm licenses:check`
 *   to compare against the allowed list (orcon-web DP-2).
 *
 * It is a Rollup plugin: it goes in `build.rollupOptions.plugins`.
 *
 * @param outDir Absolute path of the build output folder.
 * @param resolveFrom Folder whose dependencies include the font packages (the design system package).
 */
export function thirdParty(outDir: string, resolveFrom: string) {
  return license({
    thirdParty: {
      multipleVersions: true,
      output: [
        {
          file: join(outDir, 'third-party-notices.txt'),
          template: (dependencies) => {
            const packages = bundled(dependencies).map((entry) => {
              const dependency = dependencies.find((candidate) => candidate.name === entry.name && candidate.version === entry.version);
              const text = wrapLines(dependency?.licenseText?.trim() ?? '(the package ships no license file)');
              return `${entry.name}@${entry.version} (${entry.license || 'unknown license'})\n\n${text}`;
            });
            const separator = `\n\n${'-'.repeat(72)}\n\n`;
            return [
              'Third-party software in the Orcon web application',
              '',
              'The application bundles the packages below. Each is distributed under its own license, reproduced here.',
              '',
              ...[...packages, ...assetPackages.map((name) => assetNotice(name, resolveFrom))].flatMap((notice) => [separator.trimEnd(), '', notice, '']),
            ].join('\n');
          },
        },
        {
          file: join(outDir, '.vite', 'bundled-modules.json'),
          template: (dependencies) => `${JSON.stringify(bundled(dependencies), null, 2)}\n`,
        },
      ],
    },
  });
}
