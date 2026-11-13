// Where everything goes, computed from the graph alone. Deterministic: the same graph always
// gives the same layout, so screenshots and videos repeat.
//
// Each layer is a sheet. Districts are packed on it with a squarified treemap sized by unit
// count, and units sit on a grid inside their district. Districts that share a feature name
// across layers ("auth" in UI, "auth routes" in API, "auth" in Services) share one treemap
// cell, so a flow through that feature drops straight down instead of zig-zagging. Districts
// that exist on one layer only share cells with other layers' one-layer districts, so the
// sheets don't fill up with empty space.

import type { Model, Unit } from './graph';

export interface Rect { x: number; z: number; w: number; d: number } // x/z: min corner
export interface LayerBox { id: string; name: string; order: number; y: number }
export interface DistrictBox extends Rect { id: string; layer: string; name: string; y: number }
export interface Pos { x: number; y: number; z: number }

export interface Layout {
  layers: LayerBox[];
  slabW: number;
  slabD: number;
  gap: number;               // vertical distance between sheets
  scale: number;             // 1 for the mockup-sized app, larger for bigger graphs
  districts: DistrictBox[];
  pos: Pos[];                // by unit index: on the sheet surface (y = the layer's y)
  extX: number;              // where externals stand, beyond the right edge
  extY: number;
  center: Pos;               // what the camera looks at
}

const BASE_W = 24, BASE_D = 15, BASE_GAP = 6;
const AREA_PER_WEIGHT = 17;  // slab area per treemap weight unit, tuned on the mockup

// Words that say what tier a folder is rather than what feature it holds.
const GENERIC = new Set(['route', 'router', 'routes', 'controller', 'handler', 'endpoint', 'api', 'service', 'svc', 'page', 'view',
  'component', 'ui', 'screen', 'widget', 'model', 'repo', 'repository', 'module', 'lib', 'core', 'src', 'app', 'http', 'rest', 'server', 'client']);

function singular(t: string): string {
  if (t.length > 4 && t.endsWith('ies')) return t.slice(0, -3) + 'y';
  if (t.length > 3 && t.endsWith('s') && !t.endsWith('ss')) return t.slice(0, -1);
  return t;
}

export function featureKey(name: string): string {
  const toks = name.toLowerCase().split(/[^a-z0-9]+/).filter(Boolean).map(singular);
  return toks.find(t => !GENERIC.has(t)) ?? toks[0] ?? name.toLowerCase();
}

// Squarified treemap (Bruls, Huizing, van Wijk). weights must be sorted largest first.
export function treemap(weights: number[], r: Rect): Rect[] {
  const out: Rect[] = new Array(weights.length);
  const total = weights.reduce((a, b) => a + b, 0) || 1;
  const areas = weights.map(w => w * r.w * r.d / total);
  let { x, z, w, d } = r;
  let i = 0;
  const worst = (from: number, to: number, side: number) => {
    let s = 0, mx = 0, mn = Infinity;
    for (let k = from; k < to; k++) { s += areas[k]; mx = Math.max(mx, areas[k]); mn = Math.min(mn, areas[k]); }
    return Math.max(side * side * mx / (s * s), (s * s) / (side * side * mn));
  };
  while (i < areas.length) {
    const side = Math.min(w, d);
    let end = i + 1;
    while (end < areas.length && worst(i, end + 1, side) <= worst(i, end, side)) end++;
    let s = 0;
    for (let k = i; k < end; k++) s += areas[k];
    if (w >= d) { // a column on the left
      const t = s / d;
      let zz = z;
      for (let k = i; k < end; k++) { const h = areas[k] / t; out[k] = { x, z: zz, w: t, d: h }; zz += h; }
      x += t; w -= t;
    } else {      // a row at the back
      const t = s / w;
      let xx = x;
      for (let k = i; k < end; k++) { const ww = areas[k] / t; out[k] = { x: xx, z, w: ww, d: t }; xx += ww; }
      z += t; d -= t;
    }
    i = end;
  }
  return out;
}

interface Slot { key: string; layers: Set<string>; weight: number; members: { district: string; layer: string }[]; shared: boolean }

export function computeLayout(m: Model): Layout {
  const g = m.graph;
  const layerList = [...g.layers].sort((a, b) => a.order - b.order || (a.id < b.id ? -1 : 1));

  // Units per district, in graph order.
  const inDistrict = new Map<string, Unit[]>();
  for (const u of m.units) if (u.district) {
    if (!inDistrict.has(u.district)) inDistrict.set(u.district, []);
    inDistrict.get(u.district)!.push(u);
  }
  const districts = g.districts.filter(d => layerList.some(l => l.id === d.layer));

  // Feature key per district, unique within its layer.
  const keyOf = new Map<string, string>();
  const usedPerLayer = new Map<string, Set<string>>();
  for (const d of districts) {
    const used = usedPerLayer.get(d.layer) ?? new Set();
    usedPerLayer.set(d.layer, used);
    let k = featureKey(d.name), n = 2;
    while (used.has(k)) k = featureKey(d.name) + '#' + n++;
    used.add(k);
    keyOf.set(d.id, k);
  }
  const weightOf = (id: string) => Math.max(1, inDistrict.get(id)?.length ?? 0);

  // Features on two or more layers get a cell of their own.
  const byKey = new Map<string, typeof districts>();
  for (const d of districts) {
    const k = keyOf.get(d.id)!;
    if (!byKey.has(k)) byKey.set(k, []);
    byKey.get(k)!.push(d);
  }
  const slots: Slot[] = [];
  const single: typeof districts = [];
  for (const [key, ds] of byKey) {
    if (ds.length > 1) slots.push({ key, layers: new Set(ds.map(d => d.layer)), weight: Math.max(...ds.map(d => weightOf(d.id))), members: ds.map(d => ({ district: d.id, layer: d.layer })), shared: true });
    else single.push(ds[0]);
  }
  slots.sort((a, b) => b.layers.size - a.layers.size || b.weight - a.weight || (a.key < b.key ? -1 : 1));
  // One-layer districts: biggest first, each into the first one-layer cell its layer isn't using yet.
  const order = new Map(layerList.map((l, i) => [l.id, i]));
  single.sort((a, b) => weightOf(b.id) - weightOf(a.id) || order.get(a.layer)! - order.get(b.layer)! || (a.id < b.id ? -1 : 1));
  const loose: Slot[] = [];
  for (const d of single) {
    let s = loose.find(s => !s.layers.has(d.layer));
    if (!s) { s = { key: keyOf.get(d.id)!, layers: new Set(), weight: 0, members: [], shared: false }; loose.push(s); }
    s.layers.add(d.layer); s.members.push({ district: d.id, layer: d.layer }); s.weight = Math.max(s.weight, weightOf(d.id));
  }
  const all = [...slots, ...loose].map(s => ({ s, w: s.weight + 0.6 }));
  all.sort((a, b) => b.w - a.w || (a.s.shared === b.s.shared ? 0 : a.s.shared ? -1 : 1) || (a.s.key < b.s.key ? -1 : 1));

  // Slab size: the mockup's 24 × 15 until the graph needs more room.
  const totalW = all.reduce((a, b) => a + b.w, 0);
  const scale = Math.max(1, Math.sqrt(totalW * AREA_PER_WEIGHT / (BASE_W * BASE_D)));
  const slabW = BASE_W * scale, slabD = BASE_D * scale;
  const gap = BASE_GAP * Math.max(1, Math.pow(scale, 0.6));
  const top = (layerList.length - 1) / 2 * gap;
  const layers: LayerBox[] = layerList.map((l, i) => ({ id: l.id, name: l.name, order: l.order, y: top - i * gap }));
  const layerY = new Map(layers.map(l => [l.id, l.y]));

  const rects = treemap(all.map(a => a.w), { x: -slabW / 2, z: -slabD / 2, w: slabW, d: slabD });
  const boxes: DistrictBox[] = [];
  const name = new Map(districts.map(d => [d.id, d.name]));
  const pad = 0.5;
  all.forEach((a, i) => {
    const r = rects[i];
    for (const mbr of a.s.members) {
      boxes.push({
        id: mbr.district, layer: mbr.layer, name: name.get(mbr.district)!, y: layerY.get(mbr.layer)!,
        x: r.x + pad, z: r.z + pad, w: Math.max(0.4, r.w - 2 * pad), d: Math.max(0.4, r.d - 2 * pad),
      });
    }
  });
  boxes.sort((a, b) => order.get(a.layer)! - order.get(b.layer)! || (a.id < b.id ? -1 : 1));

  // Units on a grid inside their district.
  const pos: Pos[] = new Array(m.units.length);
  for (const b of boxes) {
    const list = inDistrict.get(b.id) ?? [];
    const n = list.length;
    if (!n) continue;
    const cols = Math.min(n, Math.max(1, Math.round(Math.sqrt(n * b.w / b.d))));
    const rows = Math.ceil(n / cols);
    const mx = Math.min(3, b.w * 0.45), mz = Math.min(2.6, b.d * 0.45);
    const cx = b.x + b.w / 2, cz = b.z + b.d / 2;
    list.forEach((u, i) => {
      const c = i % cols, r = Math.floor(i / cols);
      pos[u.index] = {
        x: cols === 1 ? cx : cx - (b.w - mx) / 2 + c * (b.w - mx) / (cols - 1),
        y: b.y,
        z: rows === 1 ? cz : cz - (b.d - mz) / 2 + r * (b.d - mz) / (rows - 1),
      };
    });
  }

  // Externals: beyond the right edge, at Services height (or the middle of the stack).
  const ext = m.units.filter(u => u.kind === 'external');
  const svc = layers.find(l => /^(svc|service|services)$/i.test(l.id) || /^services?$/i.test(l.name));
  const extY = svc ? svc.y : layers.length > 1 ? (layers[Math.floor((layers.length - 1) / 2)].y + layers[Math.ceil((layers.length - 1) / 2)].y) / 2 : 0;
  const extX = slabW / 2 + 4.5 * Math.sqrt(scale);
  const span = Math.min(slabD * 0.85, (ext.length - 1) * 6.5);
  ext.forEach((u, i) => {
    pos[u.index] = { x: extX, y: extY, z: ext.length === 1 ? 0 : span / 2 - i * span / (ext.length - 1) };
  });
  // Anything left (a unit whose district has no box) goes to its layer's centre.
  m.units.forEach(u => { if (!pos[u.index]) pos[u.index] = { x: 0, y: layerY.get(u.layer ?? '') ?? 0, z: 0 }; });

  const minX = -slabW / 2, maxX = ext.length ? extX + 1 : slabW / 2;
  return { layers, slabW, slabD, gap, scale, districts: boxes, pos, extX, extY, center: { x: (minX + maxX) / 2, y: 0, z: 0 } };
}
