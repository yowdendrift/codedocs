// Builds the flow view into tool/assets/lib/flowview.js.gz, which the codedocs binary embeds.
// Also writes dist/, a copy you can open straight from disk with the sample graph:
//   npm run build && open dist/flowview.html   (or ?src=synthetic.json after npm run synth)
import { build } from 'esbuild';
import { gzipSync } from 'node:zlib';
import { mkdirSync, readFileSync, writeFileSync, copyFileSync } from 'node:fs';

const here = new URL('.', import.meta.url);
const at = p => new URL(p, here);
const watch = process.argv.includes('--dev');

const out = await build({
  entryPoints: [at('src/main.ts').pathname.replace(/^\/(\w:)/, '$1')],
  bundle: true,
  format: 'iife',
  target: 'es2020',
  minify: !watch,
  legalComments: 'eof',
  write: false,
  logLevel: 'warning',
});
const js = out.outputFiles[0].contents;
// mtime 0 and a fixed level, so the same source always gives the same bytes.
writeFileSync(at('../tool/assets/lib/flowview.js.gz'), gzipSync(js, { level: 9 }));

mkdirSync(at('dist/lib'), { recursive: true });
writeFileSync(at('dist/lib/flowview.js'), js);
for (const f of ['flowview.html', 'flowview.css']) copyFileSync(at('../tool/assets/' + f), at('dist/' + f));
const sample = readFileSync(at('../docs/design/flowgraph.sample.json'), 'utf8');
writeFileSync(at('dist/flowgraph.json'), sample);
writeFileSync(at('dist/flowgraph.js'), 'window.__FLOWGRAPH__ = ' + sample.trim() + ';\n');
console.log(`flowview.js: ${(js.length / 1024).toFixed(0)} KB, ${(gzipSync(js, { level: 9 }).length / 1024).toFixed(0)} KB gzipped`);
