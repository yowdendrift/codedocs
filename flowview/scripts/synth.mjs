// A synthetic flow graph at the size the view must stay smooth at: 2000 units, 200 flows.
// Writes dist/synthetic.json; open dist/flowview.html?src=synthetic.json (serve.mjs serves it).
//   node scripts/synth.mjs [units] [flows]
import { writeFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const nUnits = +(process.argv[2] || 2000), nFlows = +(process.argv[3] || 200);
let s = 7;
const rand = () => ((s = (s * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff);
const pick = a => a[Math.floor(rand() * a.length)];

const layers = [['ui', 'UI'], ['api', 'API'], ['svc', 'Services'], ['data', 'Data']].map(([id, name], order) => ({ id, name, order }));
const features = ['users', 'billing', 'orders', 'search', 'catalog', 'auth', 'reports', 'mail', 'files', 'admin', 'cart', 'shipping'];
const districts = [];
for (const l of layers) for (const f of features) if (rand() < 0.8) districts.push({ id: `${l.id}/${f}`, layer: l.id, name: f, path: `src/${l.id}/${f}` });

const units = [];
const byLayer = Object.fromEntries(layers.map(l => [l.id, []]));
for (let i = 0; i < nUnits - 20; i++) {
  const d = pick(districts);
  const u = { id: `${d.path}/m${i}.ts:fn${i}`, name: `fn${i}`, kind: 'code', layer: d.layer, district: d.id, file: `${d.path}/m${i % 37}.ts`, lines: [1 + (i % 50) * 10, 9 + (i % 50) * 10] };
  units.push(u); byLayer[d.layer].push(u);
}
const dataD = districts.filter(d => d.layer === 'data');
for (let i = 0; i < 14; i++) { const d = pick(dataD); const u = { id: `db.t${i}`, name: `table${i}`, kind: 'store', layer: 'data', district: d.id }; units.push(u); byLayer.data.push(u); }
const ext = [];
for (let i = 0; i < 6; i++) { const u = { id: `ext.s${i}`, name: `Service ${i}`, kind: 'external' }; units.push(u); ext.push(u); }

const effects = { data: ['db-read', 'db-write'] };
const flows = [];
for (let f = 0; f < nFlows; f++) {
  const n = 6 + Math.floor(rand() * 7), steps = [];
  for (let k = 0; k < n; k++) {
    const depth = Math.min(3, Math.floor((k / n) * 4 + rand() * 0.8));
    const layer = layers[depth].id;
    if (k === n - 1 && rand() < 0.3) { steps.push({ unit: pick(ext).id, label: `Call out ${k}`, effect: 'external' }); continue; }
    const u = pick(byLayer[layer]);
    const st = { unit: u.id, label: `Step ${k + 1}` };
    if (layer === 'data') st.effect = pick(effects.data);
    steps.push(st);
  }
  const fl = { id: `f${f}`, name: `Flow ${f}`, lens: 'system', color: null, steps };
  if (rand() < 0.3) fl.branches = [{ atStep: Math.floor(rand() * (n - 1)), label: 'error' }];
  flows.push(fl);
}

const g = { version: 1, generatedAt: new Date(0).toISOString(), repo: { name: 'synthetic', commit: null, web: null, root: '..' }, layers, districts, units, flows };
const out = join(here, '..', 'dist', 'synthetic.json');
mkdirSync(dirname(out), { recursive: true });
writeFileSync(out, JSON.stringify(g));
console.log(`${out}: ${units.length} units, ${flows.length} flows, ${districts.length} districts`);
