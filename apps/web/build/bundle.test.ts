// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { describe, expect, it } from 'vitest';

import { chunkModules, type ChunkModules } from './chunk-modules';

const scripts = join(dirname(fileURLToPath(import.meta.url)), '..', 'scripts');

interface InitialChunks {
  initialJavaScript: (distFolder: string) => string[];
}

interface ChunkCheck {
  leaks: (distFolder: string) => { file: string; module: string }[];
  forbidden: string[];
}

const { initialJavaScript } = (await import(pathToFileURL(join(scripts, 'initial-chunks.mjs')).href)) as InitialChunks;
const { leaks, forbidden } = (await import(pathToFileURL(join(scripts, 'check-initial-chunks.mjs')).href)) as ChunkCheck;

interface ManifestChunk {
  file: string;
  src?: string;
  isEntry?: boolean;
  imports?: string[];
  dynamicImports?: string[];
}

/** A build output with a manifest and the modules of each chunk. */
function dist(manifest: Record<string, ManifestChunk>, chunks: ChunkModules[]): string {
  const root = mkdtempSync(join(tmpdir(), 'orcon-dist-'));
  mkdirSync(join(root, '.vite'), { recursive: true });
  writeFileSync(join(root, '.vite', 'manifest.json'), JSON.stringify(manifest));
  writeFileSync(join(root, '.vite', 'chunk-modules.json'), JSON.stringify(chunks));
  return root;
}

const entry: ManifestChunk = { file: 'assets/index.js', src: 'index.html', isEntry: true, imports: ['vendor.ts'], dynamicImports: ['src/features/editor/routes/editor.tsx'] };
const manifest: Record<string, ManifestChunk> = {
  'index.html': entry,
  'vendor.ts': { file: 'assets/vendor.js', imports: ['deep.ts'] },
  'deep.ts': { file: 'assets/deep.js' },
  'src/features/editor/routes/editor.tsx': { file: 'assets/editor.js', imports: ['flow.ts'] },
  'flow.ts': { file: 'assets/flow.js' },
  'style.css': { file: 'assets/index.css' },
};

describe('initialJavaScript', () => {
  it('lists the entry and everything it imports statically, however deep, and nothing loaded on demand', () => {
    expect(initialJavaScript(dist(manifest, [])).sort()).toEqual(['assets/deep.js', 'assets/index.js', 'assets/vendor.js']);
  });

  it('leaves out files that are not JavaScript', () => {
    const withStyle = { ...manifest, 'index.html': { ...entry, imports: ['vendor.ts', 'style.css'] } };
    expect(initialJavaScript(dist(withStyle, []))).not.toContain('assets/index.css');
  });

  it('fails clearly when the build has no entry', () => {
    expect(() => initialJavaScript(dist({ 'vendor.ts': { file: 'assets/vendor.js' } }, []))).toThrow('has no entry chunk for index.html');
  });
});

describe('leaks (the editor outside the initial chunks)', () => {
  const chunk = (file: string, modules: string[]): ChunkModules => ({ file, isEntry: false, modules });

  it('names the editor, React Flow, Zustand and Immer', () => {
    expect(forbidden.join(' ')).toMatch(/@xyflow.*zustand.*immer.*features\/editor/);
  });

  it('finds none when the editor is only in a lazy chunk', () => {
    const folder = dist(manifest, [
      chunk('assets/index.js', ['apps/web/src/main.tsx', 'node_modules/.pnpm/react@19/node_modules/react/index.js']),
      chunk('assets/editor.js', ['apps/web/src/features/editor/routes/editor.tsx', 'node_modules/.pnpm/@xyflow+react@12/node_modules/@xyflow/react/dist/index.js']),
    ]);
    expect(leaks(folder)).toEqual([]);
  });

  it('reports every forbidden module that reached an initial chunk', () => {
    const folder = dist(manifest, [
      chunk('assets/index.js', ['apps/web/src/main.tsx', 'node_modules/.pnpm/zustand@5/node_modules/zustand/index.js']),
      chunk('assets/deep.js', ['node_modules/.pnpm/@xyflow+react@12/node_modules/@xyflow/react/dist/index.js', 'node_modules/.pnpm/immer@10/node_modules/immer/dist/immer.js']),
      chunk('assets/vendor.js', ['apps/web/src/features/editor/store/editor-store.ts']),
    ]);
    expect(leaks(folder).map((found) => `${found.file} ${found.module.split('/').slice(-1)[0]}`).sort()).toEqual([
      'assets/deep.js immer.js',
      'assets/deep.js index.js',
      'assets/index.js index.js',
      'assets/vendor.js editor-store.ts',
    ]);
  });
});

describe('chunkModules plugin', () => {
  function emit(bundle: Record<string, unknown>): { fileName: string; source: string } | undefined {
    const plugin = chunkModules('/repo');
    const hook = plugin.generateBundle;
    const handler = typeof hook === 'function' ? hook : hook?.handler;
    let written: { fileName: string; source: string } | undefined;
    const context = {
      emitFile: (file: { fileName: string; source: string }) => {
        written = file;
        return 'ref';
      },
    };
    if (!handler) throw new Error('The plugin has no generateBundle hook.');
    Reflect.apply(handler, context, [{}, bundle]);
    return written;
  }

  it('writes the modules of every JavaScript chunk, relative to the repository root, without virtual modules', () => {
    const written = emit({
      'assets/index.js': {
        type: 'chunk',
        fileName: 'assets/index.js',
        isEntry: true,
        modules: { '/repo/apps/web/src/main.tsx': {}, '\0vite/preload-helper.js': {}, '/repo/node_modules/x/index.js?commonjs-es-import': {} },
      },
      'assets/index.css': { type: 'asset', fileName: 'assets/index.css' },
    });
    expect(written?.fileName).toBe('.vite/chunk-modules.json');
    expect(JSON.parse(written?.source ?? '[]')).toEqual([
      { file: 'assets/index.js', isEntry: true, modules: ['apps/web/src/main.tsx', 'node_modules/x/index.js'] },
    ]);
    expect(readFileSync(fileURLToPath(import.meta.url), 'utf8')).toContain('chunkModules');
  });
});
