// The 3D view. A port of docs/design/glass-layers-mockup.html (the visual reference: its look,
// timings and interactions are the target), with the layout computed from the graph and the
// drawing done in a few instanced meshes so large graphs stay fast.

import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { MockupBloomPass } from './bloom';
import { hash01, rng, type Flow, type Model, type Unit } from './graph';
import type { Layout } from './layout';
import type { Effect, UnitKind } from './types';

export interface Hooks {
  codeHref(u: Unit): string | null;   // null: no file to open
  openCode(u: Unit): boolean;         // true if the docs viewer's code panel took it
  pageHref(ref: string): string;
  openPage(ref: string): boolean;     // true if the docs viewer navigated
  hashChanged?(h: string): void;      // the view's deep link changed (the docs viewer mirrors it)
}

export interface Options { reduced: boolean; hooks: Hooks }

type View = '3d' | 'side' | 'top';
const EFFECT_NAME: Record<Effect, string> = { 'db-read': 'DB read', 'db-write': 'DB write', external: 'External call', 'disk-write': 'Disk write', queue: 'Queue', cache: 'Cache' };
const KIND_NAME: Record<UnitKind, [string, string]> = {
  code: ['Code unit', 'code'], store: ['Database table · side effect', ''], file: ['File storage · side effect', ''],
  external: ['External service · side effect', ''], queue: ['Queue · side effect', ''], cache: ['Cache · side effect', ''],
};
const LIFT: Record<UnitKind, number> = { code: 0.45, store: 0.14, file: 0.32, queue: 0.22, cache: 0.36, external: 0 };
const MAX_LABELS = 40;
const MAX_RINGS = 48;

const $ = <T extends HTMLElement = HTMLElement>(s: string) => document.querySelector(s) as T;
const esc = (s: unknown) => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!));

export function start(m: Model, L: Layout, opt: Options): { dispose(): void } {
  const reduced = opt.reduced;
  const H = opt.hooks;
  const NS = Math.min(2.2, Math.sqrt(L.scale)); // node size grows a little with big graphs
  // Colours are used as written, like the mockup (three.js r128 had no colour management).
  THREE.ColorManagement.enabled = false;

  // ---------- renderer ----------
  const renderer = new THREE.WebGLRenderer({ antialias: true, powerPreference: 'high-performance' });
  renderer.outputColorSpace = THREE.LinearSRGBColorSpace;
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  renderer.setSize(innerWidth, innerHeight);
  renderer.setClearColor(0x070a12, 1);
  $('#stage').appendChild(renderer.domElement);
  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(45, innerWidth / innerHeight, 0.1, 600 * L.scale);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true; controls.dampingFactor = 0.08;
  controls.autoRotate = !reduced; controls.autoRotateSpeed = 0.45;
  controls.minDistance = 6; controls.maxDistance = 140 * L.scale; controls.enablePan = true;

  let composer: EffectComposer | null = null;
  let bloomPass: MockupBloomPass | null = null;
  try {
    // 8-bit targets, as in three.js r128 that the mockup used: colours clip at 1 before the
    // bloom, so it glows the same instead of blowing out to white.
    composer = new EffectComposer(renderer, new THREE.WebGLRenderTarget(innerWidth * renderer.getPixelRatio(), innerHeight * renderer.getPixelRatio(), { type: THREE.UnsignedByteType }));
    composer.setPixelRatio(renderer.getPixelRatio());
    composer.setSize(innerWidth, innerHeight);
    composer.addPass(new RenderPass(scene, camera));
    bloomPass = new MockupBloomPass(new THREE.Vector2(innerWidth, innerHeight), 0.9, 0.55, 0.1);
    composer.addPass(bloomPass);
  } catch { composer = null; }

  // ---------- glass layers ----------
  const PW = L.slabW, PD = L.slabD;
  const slabGeo = new THREE.BoxGeometry(PW, 0.06, PD);
  const slabMat = new THREE.MeshBasicMaterial({ color: 0x6d8cff, transparent: true, opacity: 0.055, depthWrite: false, side: THREE.DoubleSide });
  const edgeGeo = new THREE.EdgesGeometry(slabGeo);
  const edgeMat = new THREE.LineBasicMaterial({ color: 0x8aa6ff, transparent: true, opacity: 0.55 });
  const step = 1.5 * Math.max(1, L.scale / 2);
  const gp: number[] = [];
  for (let x = -PW / 2 + step; x < PW / 2; x += step) gp.push(x, 0.04, -PD / 2, x, 0.04, PD / 2);
  for (let z = -PD / 2 + step; z < PD / 2; z += step) gp.push(-PW / 2, 0.04, z, PW / 2, 0.04, z);
  const gridGeo = new THREE.BufferGeometry(); gridGeo.setAttribute('position', new THREE.Float32BufferAttribute(gp, 3));
  const gridMat = new THREE.LineBasicMaterial({ color: 0x5a78d8, transparent: true, opacity: 0.07, depthWrite: false });
  for (const l of L.layers) {
    const g = new THREE.Group(); g.position.y = l.y;
    g.add(new THREE.Mesh(slabGeo, slabMat), new THREE.LineSegments(edgeGeo, edgeMat), new THREE.LineSegments(gridGeo, gridMat));
    scene.add(g);
  }
  // Districts: every outline in one draw call.
  const districtMat = new THREE.LineBasicMaterial({ color: 0x6f8ff0, transparent: true, opacity: 0.28 });
  {
    const p: number[] = [];
    for (const d of L.districts) {
      const y = d.y + 0.05, x0 = d.x, x1 = d.x + d.w, z0 = d.z, z1 = d.z + d.d;
      p.push(x0, y, z0, x1, y, z0, x1, y, z0, x1, y, z1, x1, y, z1, x0, y, z1, x0, y, z1, x0, y, z0);
    }
    const g = new THREE.BufferGeometry(); g.setAttribute('position', new THREE.Float32BufferAttribute(p, 3));
    scene.add(new THREE.LineSegments(g, districtMat));
  }

  // ---------- units: one instanced mesh per shape ----------
  const GEO: Record<UnitKind, THREE.BufferGeometry> = {
    code: new THREE.SphereGeometry(0.3, 20, 14),
    store: new THREE.CylinderGeometry(0.62, 0.62, 0.2, 30),
    file: new THREE.BoxGeometry(1, 0.55, 1),
    queue: new THREE.CapsuleGeometry(0.2, 0.7, 6, 14).rotateZ(Math.PI / 2),
    cache: new THREE.IcosahedronGeometry(0.42, 0),
    external: new THREE.OctahedronGeometry(0.62, 0),
  };
  const C = { code: new THREE.Color('#dce6ff'), eff: new THREE.Color('#ff8a5c'), dark: new THREE.Color('#283042'), gap: new THREE.Color('#ff4d6d') };
  const tmpC = new THREE.Color();
  // pos: the unit's home on its slab. show: where it's drawn, which moves between pos and its
  // place in the Focus view (from → to over mt 0..1). vis fades units the Focus view leaves out.
  interface UN {
    u: Unit; pos: THREE.Vector3; show: THREE.Vector3; from: THREE.Vector3; to: THREE.Vector3; mt: number; vis: number;
    mesh: THREE.InstancedMesh; slot: number; color: THREE.Color; pop: number; labelT: number; base: number; spin: boolean;
  }
  const un: UN[] = [];
  const byKind = new Map<UnitKind, Unit[]>();
  for (const u of m.units) { if (!byKind.has(u.kind)) byKind.set(u.kind, []); byKind.get(u.kind)!.push(u); }
  const kindMesh = new Map<UnitKind, THREE.InstancedMesh>();
  for (const [kind, list] of byKind) {
    const mesh = new THREE.InstancedMesh(GEO[kind], new THREE.MeshBasicMaterial({ color: 0xffffff }), list.length);
    mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    mesh.frustumCulled = false;
    kindMesh.set(kind, mesh);
    scene.add(mesh);
    list.forEach((u, slot) => {
      const p = L.pos[u.index], at = new THREE.Vector3(p.x, p.y + LIFT[kind] * NS, p.z);
      un[u.index] = {
        u, mesh, slot, pos: at, show: at.clone(), from: at.clone(), to: at.clone(), mt: 1, vis: 1, color: C.dark.clone(), pop: 0, labelT: 0,
        base: 1 + 0.2 * Math.max(0, u.flows.length - 1), spin: kind === 'external' || kind === 'file',
      };
      mesh.setColorAt(slot, C.dark);
    });
  }
  // Invisible hit spheres, about 2.7× a code node, so touch targets work on phones.
  const hit = new THREE.InstancedMesh(new THREE.SphereGeometry(0.8 * NS, 8, 6), new THREE.MeshBasicMaterial({ visible: false }), m.units.length);
  const mtx = new THREE.Matrix4(), q = new THREE.Quaternion(), sv = new THREE.Vector3(), eul = new THREE.Euler();
  un.forEach(n => { hit.setMatrixAt(n.u.index, mtx.makeTranslation(n.pos.x, n.pos.y, n.pos.z)); });
  hit.computeBoundingSphere();
  scene.add(hit);
  // A faint ring on the sheet under each code unit.
  const code = byKind.get('code') ?? [];
  const sheetRingMat = new THREE.MeshBasicMaterial({ color: 0x7d98ff, transparent: true, opacity: 0.22, depthWrite: false });
  if (code.length) {
    const g = new THREE.RingGeometry(0.34 * NS, 0.44 * NS, 28); g.rotateX(-Math.PI / 2);
    const rings = new THREE.InstancedMesh(g, sheetRingMat, code.length);
    code.forEach((u, i) => { const p = L.pos[u.index]; rings.setMatrixAt(i, mtx.makeTranslation(p.x, p.y + 0.05, p.z)); });
    scene.add(rings);
  }
  // Externals turn slowly inside a ring.
  const ext = byKind.get('external') ?? [];
  const gates = ext.length ? new THREE.InstancedMesh(new THREE.TorusGeometry(1.05, 0.035, 8, 48), new THREE.MeshBasicMaterial({ color: 0xff8a5c, transparent: true, opacity: 0.5 }), ext.length) : null;
  if (gates) { gates.frustumCulled = false; scene.add(gates); }

  // Dust, from a fixed seed so every run looks the same.
  const dustMat = new THREE.PointsMaterial({ color: 0x3d4f88, size: 0.12, transparent: true, opacity: 0.7, depthWrite: false });
  {
    const r = rng(7), p: number[] = [], S = L.scale;
    for (let i = 0; i < 700; i++) p.push((r() - 0.5) * 120 * S, (r() - 0.5) * 80 * S, (r() - 0.5) * 120 * S);
    const g = new THREE.BufferGeometry(); g.setAttribute('position', new THREE.Float32BufferAttribute(p, 3));
    scene.add(new THREE.Points(g, dustMat));
  }

  // ---------- flows ----------
  interface FR {
    f: Flow; path: THREE.CurvePath<THREE.Vector3>; total: number; stepT: number[]; segs: number;
    start: number; count: number; off: THREE.Vector3;
    run: { t: number; idx: number; hold: number };
    tube?: THREE.Mesh<THREE.TubeGeometry, THREE.MeshBasicMaterial>; stubs?: THREE.Group;
    stubLabels: { text: string; pos: THREE.Vector3; origin: number }[];
    dirty: boolean; // a unit on it moved: rebuild the path from where the units are drawn
  }
  const many = m.flows.length > 12;
  // The curve through a flow's units: arcs over hops within a layer, S-bends between layers.
  // In the 2D view (jump given) the column is a straight line, and a hop that isn't to the next
  // row down (a return to an earlier unit, or a skip past rows) bows out to the right instead
  // of running back over the line.
  function buildPath(pts: THREE.Vector3[], jump?: (i: number) => number) {
    const path = new THREE.CurvePath<THREE.Vector3>(), seg: number[] = [];
    for (let i = 0; i < pts.length - 1; i++) {
      const a = pts[i], b = pts[i + 1], dy = b.y - a.y, rows = jump?.(i) ?? 1;
      let c: THREE.Curve<THREE.Vector3>;
      if (rows !== 1 && rows !== 0) {
        const out = 1.4 + 0.45 * Math.abs(rows);
        c = new THREE.CubicBezierCurve3(a, new THREE.Vector3(a.x + out, a.y + dy * 0.1, a.z), new THREE.Vector3(b.x + out, b.y - dy * 0.1, b.z), b);
      } else if (Math.abs(dy) < 0.6) { const mid = a.clone().lerp(b, 0.5); mid.y = Math.max(a.y, b.y) + 1.1 + a.distanceTo(b) * 0.08; c = new THREE.QuadraticBezierCurve3(a, mid, b); }
      else c = new THREE.CubicBezierCurve3(a, new THREE.Vector3(a.x, a.y + dy * 0.55, a.z), new THREE.Vector3(b.x, b.y - dy * 0.55, b.z), b);
      path.add(c); seg.push(c.getLength());
    }
    if (pts.length === 1) { const a = pts[0], b = a.clone(); b.y += 0.01; const c = new THREE.LineCurve3(a, b); path.add(c); seg.push(0.01); }
    const total = seg.reduce((x, y) => x + y, 0) || 0.01;
    let acc = 0; const stepT = [0];
    seg.forEach(l => { acc += l; stepT.push(Math.min(1, acc / total)); });
    if (pts.length === 1) stepT.length = 1;
    return { path, total, stepT };
  }
  const ZERO = new THREE.Vector3();
  const pathPoints = (f: Flow, off: THREE.Vector3) => f.steps.map(s => un[s.unit.index].show.clone().add(off));
  const FL: FR[] = m.flows.map((f, fi) => {
    const lanes = Math.min(m.flows.length, 8), k = fi % 8;
    const o = (k - (lanes - 1) / 2) * 0.17;
    const off = new THREE.Vector3(o, 0, o);
    const { path, total, stepT } = buildPath(pathPoints(f, off));
    const segs = many ? Math.max(24, Math.min(f.steps.length * 16, Math.round(total * 3))) : Math.max(80, f.steps.length * 50);
    return { f, path, total, stepT, segs, start: 0, count: 0, off, run: { t: hash01(f.id), idx: -1, hold: 0 }, stubLabels: [], dirty: false };
  });
  // Idle flows: every tube merged into one mesh, coloured per vertex, in flow order so the
  // light-up can reveal them one after another with a single draw range.
  const tubes = (() => {
    const geos = FL.map(r => new THREE.TubeGeometry(r.path, r.segs, 0.055, 6, false));
    let nv = 0, ni = 0;
    geos.forEach(g => { nv += g.attributes.position.count; ni += g.index!.count; });
    const P = new Float32Array(nv * 3), Cl = new Float32Array(nv * 3), I = new Uint32Array(ni);
    let v = 0, ii = 0;
    geos.forEach((g, fi) => {
      const pos = g.attributes.position.array as Float32Array, idx = g.index!.array;
      P.set(pos, v * 3);
      const col = new THREE.Color(FL[fi].f.color);
      for (let k = 0; k < g.attributes.position.count; k++) { Cl[(v + k) * 3] = col.r; Cl[(v + k) * 3 + 1] = col.g; Cl[(v + k) * 3 + 2] = col.b; }
      FL[fi].start = ii; FL[fi].count = idx.length;
      for (let k = 0; k < idx.length; k++) I[ii + k] = idx[k] + v;
      v += g.attributes.position.count; ii += idx.length;
      g.dispose();
    });
    const g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.BufferAttribute(P, 3));
    g.setAttribute('color', new THREE.BufferAttribute(Cl, 3));
    g.setIndex(new THREE.BufferAttribute(I, 1));
    const mesh = new THREE.Mesh(g, new THREE.MeshBasicMaterial({ vertexColors: true, transparent: true, opacity: 0, depthWrite: false }));
    mesh.frustumCulled = false;
    scene.add(mesh);
    return mesh;
  })();
  // Pulses: a bright head and a fading trail of 8 points per flow, all in one instanced mesh.
  const TRAIL = 8;
  const heads = new THREE.InstancedMesh(new THREE.SphereGeometry(0.17, 14, 10),
    new THREE.MeshBasicMaterial({ transparent: true, depthWrite: false, blending: THREE.AdditiveBlending }), Math.max(1, FL.length * TRAIL));
  heads.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
  heads.frustumCulled = false;
  FL.forEach((r, fi) => {
    const c = new THREE.Color(r.f.color);
    for (let k = 0; k < TRAIL; k++) {
      heads.setMatrixAt(fi * TRAIL + k, mtx.makeScale(0, 0, 0));
      heads.setColorAt(fi * TRAIL + k, k === 0 ? c.clone().lerp(new THREE.Color('#ffffff'), 0.6) : c.clone().multiplyScalar(0.75 * (1 - k / TRAIL)));
    }
  });
  scene.add(heads);
  const headVisible = new Array(FL.length).fill(false);
  const headFade = new Array(FL.length).fill(1);

  // The selected flow gets a tube of its own on top, plus its branch stubs.
  function ensureSelected(r: FR) {
    if (!r.tube) {
      r.tube = new THREE.Mesh(new THREE.TubeGeometry(r.path, r.segs, 0.055, 6, false), new THREE.MeshBasicMaterial({ color: r.f.color, transparent: true, opacity: 0, depthWrite: false }));
      scene.add(r.tube);
    }
    if (!r.stubs) buildStubs(r);
  }
  const stubLineMat = new THREE.LineDashedMaterial({ color: 0x9aa3b5, dashSize: 0.16, gapSize: 0.12, transparent: true, opacity: 0.8 });
  const stubEndGeo = new THREE.SphereGeometry(0.11, 10, 8), stubEndMat = new THREE.MeshBasicMaterial({ color: 0x8a93a8 });
  // Branches not taken: a dashed stub off the step they leave from. On the slabs they fan out
  // around the unit; in the Focus view they drop away to the right of the column.
  function buildStubs(r: FR) {
    const visible = r.stubs?.visible ?? false;
    if (r.stubs) { scene.remove(r.stubs); r.stubs.children.forEach(o => { if (o instanceof THREE.Line) o.geometry.dispose(); }); }
    r.stubs = new THREE.Group(); r.stubLabels = [];
    const inFocus = !!FX && FX.r === r;
    r.f.branches.forEach((b, bi) => {
      const from = un[r.f.steps[b.at].unit.index].show.clone().add(r.off);
      let to: THREE.Vector3;
      if (inFocus) to = from.clone().add(new THREE.Vector3(1.5 + bi * 0.6, -0.95 - bi * 0.35, 0.4));
      else {
        const a = 0.9 + bi * 2.2 + r.f.index * 0.8, dir = new THREE.Vector3(Math.cos(a), 0, Math.sin(a));
        to = from.clone().addScaledVector(dir, 1.9); to.y += 0.7;
      }
      const mid = from.clone().lerp(to, 0.5); mid.y += inFocus ? 0.15 : 0.5;
      const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints(new THREE.QuadraticBezierCurve3(from, mid, to).getPoints(16)), stubLineMat);
      line.computeLineDistances(); r.stubs!.add(line);
      const end = new THREE.Mesh(stubEndGeo, stubEndMat); end.position.copy(to); r.stubs!.add(end);
      r.stubLabels.push({ text: b.label, pos: inFocus ? to.clone() : to.clone().add(new THREE.Vector3(0, 0.35, 0)), origin: b.at });
    });
    r.stubs.visible = visible; scene.add(r.stubs);
  }
  function rebuildPath(r: FR) {
    r.dirty = false;
    const F = FX && FX.r === r ? FX : null;
    const row = (i: number) => F!.rowOf.get(r.f.steps[i].unit.index)!;
    const b = buildPath(pathPoints(r.f, F ? ZERO : r.off), F ? i => row(i + 1) - row(i) : undefined);
    r.path = b.path; r.total = b.total; r.stepT = b.stepT;
    if (r.tube) { r.tube.geometry.dispose(); r.tube.geometry = new THREE.TubeGeometry(r.path, r.segs, 0.055, 6, false); }
    if (r.stubs) buildStubs(r);
  }

  // ---------- effect rings ----------
  const ringGeo = new THREE.RingGeometry(0.5, 0.62, 40); ringGeo.rotateX(-Math.PI / 2);
  const rings: { m: THREE.Mesh<THREE.RingGeometry, THREE.MeshBasicMaterial>; t: number; s: number }[] = [];
  function spawnRing(pos: THREE.Vector3, strength: number, eff: Effect) {
    if (reduced || rings.length >= MAX_RINGS) return;
    const mm = new THREE.Mesh(ringGeo, new THREE.MeshBasicMaterial({ color: eff === 'db-read' ? 0xffc48a : 0xff7a4d, transparent: true, opacity: strength, depthWrite: false, side: THREE.DoubleSide }));
    if (fe > 0.5) mm.rotation.x = Math.PI / 2; // the Focus view is seen from the front: face the camera
    mm.position.copy(pos); mm.scale.setScalar(NS); scene.add(mm); rings.push({ m: mm, t: 0, s: strength });
  }

  // ---------- state ----------
  let sel: FR | null = null, gaps = false, view: View = '3d', hovered: UN | null = null, tween: null | { fp: THREE.Vector3; tp: THREE.Vector3; ft: THREE.Vector3; tt: THREE.Vector3; t: number; d: number } = null;
  let userMoved = false;
  // The 2D view (Focus): the selected flow alone, its units pulled off the slabs into one
  // straight column, a row per unit in the order the flow first reaches it. Runs of rows on the
  // same layer are marked by a divider and the layer's name. The slabs and every other unit fade
  // away (fe: 0 = the stack, 1 = the column) and the camera faces it square on.
  interface FocusLayout {
    r: FR; at: Map<number, THREE.Vector3>; rows: Map<number, number[]>; // unit index → its place, its step numbers
    rowOf: Map<number, number>;                                          // unit index → its row
    bands: { name: string; y: number }[];                                // a layer's run of rows starts below y
    top: number; bottom: number; center: THREE.Vector3;
  }
  let focus = false, fe = 0, FX: FocusLayout | null = null;
  const lu ={ active: false, fi: 0, p: 0, pause: 0, lit: new Set<number>() };
  const PER_FLOW = FL.length <= 8 ? 2.3 : Math.max(0.2, 18 / FL.length);
  const BETWEEN = FL.length <= 8 ? 0.3 : 0.03;
  const total = m.units.length;

  // ---------- camera ----------
  const TARGET = new THREE.Vector3(L.center.x, L.center.y, L.center.z);
  const stackH = (L.layers.length - 1) * L.gap;
  const halfW = (L.extX + 1 + PW / 2) / 2;
  const fitPoints: THREE.Vector3[] = [];
  for (const l of L.layers) for (const sx of [-1, 1]) for (const sz of [-1, 1]) fitPoints.push(new THREE.Vector3(sx * PW / 2, l.y, sz * PD / 2));
  if (m.units.some(u => u.kind === 'external')) fitPoints.push(new THREE.Vector3(L.extX + 3, L.extY + 1.9, 0), new THREE.Vector3(L.extX + 3, L.extY - 1.9, 0));
  function presetFor(v: View) {
    const aspect = innerWidth / innerHeight, t = Math.tan((camera.fov * Math.PI / 180) / 2);
    const portrait = aspect < 0.8;
    let dir: THREE.Vector3, hw: number, hh: number;
    if (v === 'side') { dir = new THREE.Vector3(0, 0.07, 1); hw = (halfW + 2.75) * (portrait ? 0.74 : 1); hh = stackH / 2 + 3.5; }
    else if (v === 'top') { dir = new THREE.Vector3(0, 1, 0.015); hw = (halfW + 2.75) * (portrait ? 0.69 : 1); hh = PD / 2 + 1.5; }
    else { dir = portrait ? new THREE.Vector3(0.42, 0.3, 0.86) : new THREE.Vector3(0.62, 0.36, 0.74); hw = (halfW + 0.75) * (portrait ? 0.87 : 1); hh = stackH / 2 + 5; }
    dir.normalize();
    // The mockup's framing, tuned for its 24x15 slab...
    const tuned = Math.max(hw / (t * aspect), hh / t) * 1.08 + (v === '3d' ? 6 : 4) * Math.sqrt(L.scale);
    // ...and at least enough to see every corner of the stack and the outside services, which
    // a bigger, deeper slab seen at an angle needs. Portrait keeps the mockup's tighter crop.
    const crop = portrait ? (v === 'side' ? 0.74 : v === 'top' ? 0.69 : 0.87) : 1;
    const fwd = dir.clone().negate(), right = new THREE.Vector3().crossVectors(fwd, camera.up).normalize();
    const up = new THREE.Vector3().crossVectors(right, fwd);
    let fit = 0;
    for (const p of fitPoints) {
      const r = p.clone().sub(TARGET), z = r.dot(dir);
      fit = Math.max(fit, z + Math.abs(r.dot(right)) * crop / (t * aspect), z + Math.abs(r.dot(up)) / t);
    }
    const dist = Math.max(tuned, fit * 1.1 + 2);
    return { pos: TARGET.clone().addScaledVector(dir, dist), target: TARGET.clone() };
  }
  function moveCamera(p: { pos: THREE.Vector3; target: THREE.Vector3 }, instant: boolean) {
    if (instant || reduced) { camera.position.copy(p.pos); controls.target.copy(p.target); tween = null; return; }
    tween = { fp: camera.position.clone(), tp: p.pos, ft: controls.target.clone(), tt: p.target, t: 0, d: 1.2 };
  }
  function goView(v: View, instant = false) {
    view = v;
    document.querySelectorAll<HTMLButtonElement>('.seg button').forEach(b => b.classList.toggle('on', b.dataset.view === v));
    controls.autoRotate = v === '3d' && !reduced && !userMoved;
    moveCamera(presetFor(v), instant);
  }
  // A phone, or a tall narrow window: the Focus view keeps its labels on one side.
  const narrowScreen = () => innerWidth < 640 || innerWidth / innerHeight < 0.8;
  const ROW_GAP = 2.1;
  function focusLayout(r: FR): FocusLayout {
    const order: number[] = [], rows = new Map<number, number[]>(), rowOf = new Map<number, number>();
    r.f.steps.forEach((s, i) => { const k = s.unit.index; if (!rows.has(k)) { rows.set(k, []); rowOf.set(k, order.length); order.push(k); } rows.get(k)!.push(i); });
    const R = order.length, cx = TARGET.x, cy = TARGET.y;
    const yOf = (row: number) => cy + ((R - 1) / 2 - row) * ROW_GAP;
    const at = new Map<number, THREE.Vector3>();
    order.forEach((k, row) => at.set(k, new THREE.Vector3(cx, yOf(row), 0)));
    const bands: FocusLayout['bands'] = [];
    order.forEach((k, row) => {
      const name = un[k].u.layerName || 'Services';
      if (!bands.length || bands[bands.length - 1].name !== name) bands.push({ name, y: yOf(row) + ROW_GAP / 2 });
    });
    const half = (R - 1) / 2 * ROW_GAP;
    return { r, at, rows, rowOf, bands, top: cy + half, bottom: cy - half, center: new THREE.Vector3(cx, cy, 0) };
  }
  // Square on, in the clear band between the header and the controls at the bottom. Labels are
  // HTML of a fixed pixel size, so the framing is worked out in pixels: the column in the middle
  // of a wide screen (steps to its left, names to its right), near the left edge of a narrow one
  // (everything to its right).
  function focusPreset(F: FocusLayout) {
    const t = Math.tan((camera.fov * Math.PI / 180) / 2), narrow = narrowScreen();
    const top = ($('header')?.getBoundingClientRect().bottom ?? 0) - 6;
    const bottom = $('#sheet')?.getBoundingClientRect().top ?? innerHeight;
    const band = Math.max(0.35, (bottom - top) / innerHeight);
    const h = (F.top + 1.5 - F.bottom) / 2 + 1.2;
    // Long flows would shrink to nothing: keep rows at least ~34 px apart and let the user pan.
    const dist = Math.min(h / (t * band) * 1.04, (innerHeight / 2) * ROW_GAP / (t * 34));
    const ppu = (innerHeight / 2) / (t * dist); // screen px per world unit at the column
    const lanesAt = narrow ? 30 : innerWidth / 2;
    // The world y to put in the middle of the clear band: the column's middle, or for a flow
    // too long to fit, wherever puts its top at the top of the band.
    const fits = h / (t * band) * 1.04 <= dist;
    const bandMid = fits ? F.center.y + 0.75 : F.top + 1.5 - band * t * dist;
    const target = new THREE.Vector3(
      F.center.x + (innerWidth / 2 - lanesAt) / ppu,
      bandMid - ((innerHeight / 2 - (top + bottom) / 2) / (innerHeight / 2)) * t * dist,
      0);
    const dir = new THREE.Vector3(0, 0, 1);
    return { pos: target.clone().addScaledVector(dir, dist), target };
  }
  // Faint dividers where the column moves to another layer.
  const laneMat = new THREE.LineBasicMaterial({ color: 0x8aa6ff, transparent: true, opacity: 0, depthWrite: false });
  let laneLines: THREE.LineSegments | null = null;
  function buildLanes() {
    if (laneLines) { scene.remove(laneLines); laneLines.geometry.dispose(); laneLines = null; }
    if (!FX) return;
    const p: number[] = [];
    const x = FX.center.x, narrow = narrowScreen();
    for (const b of FX.bands.slice(1)) p.push(x - (narrow ? 0.8 : 6), b.y, -0.3, x + (narrow ? 9 : 6), b.y, -0.3);
    const g = new THREE.BufferGeometry(); g.setAttribute('position', new THREE.Float32BufferAttribute(p, 3));
    laneLines = new THREE.LineSegments(g, laneMat); scene.add(laneLines);
  }
  // Into or out of the Focus view (or onto another flow while in it): give every unit its new
  // place and let the frame loop carry it there, with the camera on the same 1.2 s ease.
  function setFocus(on: boolean, instant = false) {
    const was = FX;
    focus = on && !!sel;
    FX = focus ? focusLayout(sel!) : null;
    for (const n of un) {
      const to = FX?.at.get(n.u.index) ?? n.pos;
      if (to.equals(n.to)) continue;
      n.from.copy(n.show); n.to.copy(to); n.mt = 0;
      if (instant || reduced) { n.mt = 1; n.show.copy(to); }
      for (const f of n.u.flows) FL[f.index].dirty = true;
    }
    buildLanes();
    if (sel) rebuildPath(sel);
    $('#btnFocus').classList.toggle('on', focus);
    controls.enableRotate = !FX; // 2D: pan and zoom only
    if (FX) {
      document.querySelectorAll<HTMLButtonElement>('.seg button').forEach(b => b.classList.remove('on'));
      controls.autoRotate = false;
      moveCamera(focusPreset(FX), instant);
    } else if (was) goView(view, instant);
  }
  function setFov() { camera.fov = innerWidth / innerHeight < 0.8 ? 58 : 45; camera.updateProjectionMatrix(); }
  setFov();
  {
    const p = presetFor('3d');
    if (reduced) { camera.position.copy(p.pos); controls.target.copy(TARGET); }
    else {
      camera.position.copy(TARGET.clone().add(p.pos.clone().sub(TARGET).multiplyScalar(1.5))); controls.target.copy(TARGET);
      tween = { fp: camera.position.clone(), tp: p.pos, ft: TARGET.clone(), tt: TARGET.clone(), t: 0, d: 3 };
    }
  }
  controls.addEventListener('start', () => { userMoved = true; tween = null; controls.autoRotate = false; });

  // ---------- labels: HTML over the canvas, at most MAX_LABELS units at a time ----------
  const labelLayer = $('#labels');
  interface Lbl { el: HTMLDivElement; on: boolean; seen: number }
  const pool = new Map<string, Lbl>();
  let frameNo = 0;
  function label(key: string, text: string, cls: string): Lbl {
    let l = pool.get(key);
    if (!l) { const el = document.createElement('div'); el.className = 'lbl ' + cls; el.textContent = text; labelLayer.appendChild(el); l = { el, on: false, seen: 0 }; pool.set(key, l); }
    return l;
  }
  const pv = new THREE.Vector3();
  function place(l: Lbl, p: THREE.Vector3, anchor: string) {
    l.seen = frameNo;
    if (!l.on) { l.on = true; l.el.classList.add('on'); }
    pv.copy(p).project(camera);
    if (pv.z > 1 || pv.z < -1) { l.el.style.visibility = 'hidden'; return; }
    l.el.style.visibility = '';
    l.el.style.transform = `translate(${((pv.x * 0.5 + 0.5) * innerWidth).toFixed(1)}px,${((-pv.y * 0.5 + 0.5) * innerHeight).toFixed(1)}px) ${anchor}`;
  }
  const lp = new THREE.Vector3();
  function currentUnit(): number | null {
    return sel && sel.run.idx >= 0 && (sel.run.hold <= 0 || reduced) ? sel.f.steps[sel.run.idx].unit.index : null;
  }
  function updateLabels() {
    frameNo++;
    if (FX && fe > 0.5) for (const b of FX.bands) place(label('band:' + b.name + ':' + b.y.toFixed(2), b.name, 'layer'), lp.set(FX.center.x, b.y, 0), narrowScreen() ? 'translate(16px,-50%)' : 'translate(-100%,-50%) translateX(-16px)');
    else if (fe < 0.5) {
      for (const l of L.layers) place(label('layer:' + l.id, l.name, 'layer'), lp.set(-PW / 2, l.y, PD / 2), 'translate(6px,-140%)');
      if (ext.length && view !== 'top') place(label('layer:ext', 'external', 'layer'), lp.set(L.extX, L.extY + 1.9, -0.2), 'translate(-50%,-50%)');
    }
    if (!sel && !lu.active && !gaps && view !== 'side' && L.districts.length <= 60)
      for (const d of L.districts) place(label('d:' + d.layer + d.id, d.name, 'district'), lp.set(d.x + 0.35, d.y + 0.05, d.z + d.d - 0.35), 'translate(0,-50%)');
    // Units: pick the most important ones.
    const cur = currentUnit();
    const want: [number, number][] = []; // [priority, unit index]
    for (const n of un) {
      const u = n.u;
      if (FX && !FX.at.has(u.index)) continue;
      let p = 0;
      if (hovered === n) p = 100;
      else if (FX) p = cur === u.index ? 90 : 60;
      else if (cur === u.index) p = 90;
      else if (n.labelT > 0) p = 70;
      else if (lu.active) p = 0;
      else if (u.kind === 'external') p = 40;
      else if (gaps) p = u.flows.length ? 0 : 30;
      else if (sel && u.flows.includes(sel.f)) {
        if (reduced || u.kind !== 'code') p = 50;
        else if (sel.f.steps[0].unit === u) p = 80;
      }
      if (p > 0) want.push([p, u.index]);
    }
    want.sort((a, b) => b[0] - a[0] || a[1] - b[1]);
    for (const [, i] of want.slice(0, innerWidth < 560 && !FX ? Math.round(MAX_LABELS * 0.4) : MAX_LABELS)) { // fewer on a phone, except one flow in Focus
      const n = un[i];
      const l = label('u:' + i, n.u.name, n.u.kind === 'external' ? 'ext' : 'node');
      const isCur = cur === i, g = gaps && !n.u.flows.length && !lu.active;
      l.el.classList.toggle('gap', g); l.el.classList.toggle('cur', isCur);
      l.el.style.borderColor = isCur && sel ? sel.f.color : '';
      if (FX && fe > 0.3) place(l, n.show, `translate(${n.u.kind === 'code' ? 16 : 30}px,${narrowScreen() ? '2px' : '-50%'})`);
      else place(l, lp.copy(n.show).add(sv.set(0, (n.u.kind === 'code' ? 0.55 : 0.75) * NS, 0)), 'translate(-50%,-100%)');
    }
    // In the Focus view each row also says what happens there: "3. Snapshot the timeline".
    // Narrow screens have no room for floating branch labels, so the row names its branches.
    const narrow = narrowScreen();
    if (FX && fe > 0.3) for (const [k, idx] of FX.rows) {
      const f = FX.r.f;
      const text = idx.map(i => `${i + 1}. ${f.steps[i].label}` + (narrow ? f.branches.filter(b => b.at === i).map(b => `  ↳ ${b.label}`).join('') : '')).join(' · ');
      const l = label('fs:' + f.id + ':' + k + (narrow ? 'n' : ''), text, 'steplbl');
      l.el.classList.toggle('cur', cur === k);
      const gap = un[k].u.kind === 'code' ? 16 : 30;
      // Wide: left of the unit. Narrow: right of it, above its name.
      place(l, un[k].show, narrowScreen() ? `translate(${gap}px,-100%)` : `translate(-100%,-50%) translateX(-${gap}px)`);
    }
    if (sel && !lu.active && !(FX && narrow)) for (const s of sel.stubLabels) if (reduced || sel.run.hold > 0 || sel.run.idx >= s.origin) place(label('s:' + sel.f.id + s.text + s.origin, s.text, 'stub'), s.pos, FX ? 'translate(10px,-50%)' : 'translate(-50%,-100%)');
    for (const [key, l] of pool) {
      if (l.seen === frameNo) continue;
      if (l.on) { l.on = false; l.el.classList.remove('on'); l.seen = frameNo; }
      else if (frameNo - l.seen > 40) { l.el.remove(); pool.delete(key); }
    }
  }

  // ---------- UI ----------
  const chips = $('#chips'), cap = $('#caption'), card = $('#card');
  function chip(text: string, color: string | null, fn: () => void) {
    const b = document.createElement('button');
    b.innerHTML = (color ? `<span class="sw" style="background:${esc(color)}"></span>` : '') + esc(text);
    b.onclick = fn; chips.appendChild(b); return b;
  }
  // There's no "all flows" mode: drawn at once they tangle, and more so as a project grows.
  // Nothing runs until a flow is picked; picking it again lets it go.
  const flowChip = FL.map(r => chip(r.f.name, r.f.color, () => selectFlow(sel === r ? null : r)));
  function refreshButtons() {
    FL.forEach((r, i) => flowChip[i].classList.toggle('on', sel === r));
    $('#btnGaps').classList.toggle('on', gaps);
    $('#btnLight').classList.toggle('on', lu.active);
    const bf = $<HTMLButtonElement>('#btnFocus');
    bf.disabled = !sel; bf.classList.toggle('on', !!FX);
    bf.title = sel ? (FX ? 'Back to the layers' : 'Lay this flow out flat, top to bottom') : 'Pick a flow first';
  }
  function setHash(h: string) {
    try { history.replaceState(null, '', h ? '#' + h : location.pathname + location.search); } catch { /* sandboxed */ }
    H.hashChanged?.(h);
  }
  function stopLightUp() {
    if (!lu.active) return;
    lu.active = false; tubes.geometry.setDrawRange(0, Infinity);
    setCoverage(m.covered / total);
  }
  function flowName(f: Flow) {
    return f.docRef ? `<a class="fname" href="${esc(H.pageHref(f.docRef))}" data-ref="${esc(f.docRef)}">${esc(f.name)}</a>` : `<b>${esc(f.name)}</b>`;
  }
  function selectFlow(r: FR | null, fromHash = false) {
    stopLightUp(); sel = r; gaps = false; hideCard();
    if (r) {
      ensureSelected(r);
      r.run.t = 0; r.run.idx = reduced ? 0 : -1; r.run.hold = 0;
      if (reduced) staticCaption(r);
      flowChip[r.f.index].scrollIntoView({ block: 'nearest', inline: 'nearest' });
    } else cap.innerHTML = FL.length ? `<span class="st">Pick a flow below to follow it.</span>` : `<span class="st">No documented flows yet.</span>`;
    if (focus) setFocus(!!r); // another flow slides into the column, or the column goes back
    if (!fromHash) setHash(r ? (FX ? 'focus-' : 'flow-') + r.f.id : '');
    refreshButtons();
  }
  function toggleFocus(on: boolean, fromHash = false) {
    if (!sel) return;
    setFocus(on);
    if (!fromHash) setHash((FX ? 'focus-' : 'flow-') + sel.f.id);
    refreshButtons();
  }
  $('#btnFocus').onclick = () => toggleFocus(!focus);
  function setCaption(r: FR, i: number) {
    const s = r.f.steps[i];
    cap.innerHTML = `<i class="dot" style="background:${esc(r.f.color)}"></i>${flowName(r.f)}<span class="st">${i + 1}/${r.f.steps.length}</span><span>${esc(s.label)}</span>` +
      (s.effect ? `<span class="tag${s.effect === 'db-read' ? ' read' : ''}">${EFFECT_NAME[s.effect]}</span>` : '');
  }
  function staticCaption(r: FR) {
    const effects = r.f.steps.filter(s => s.effect).length;
    cap.innerHTML = `<i class="dot" style="background:${esc(r.f.color)}"></i>${flowName(r.f)}<span class="st">${r.f.steps.length} steps</span>` +
      `<span>${esc(r.f.steps[0].label)} → ${esc(r.f.steps[r.f.steps.length - 1].label)}</span>` + (effects ? `<span class="tag">${effects} side effect${effects > 1 ? 's' : ''}</span>` : '');
  }
  function gapsCaption() {
    const n = total - m.covered;
    cap.innerHTML = n
      ? `<span class="warn">${n} of ${total} code units sit on no documented flow.</span><span class="st">Undocumented paths, or dead code.</span>`
      : `<span class="st">Every unit sits on a documented flow.</span>`;
  }
  function setGaps(on: boolean, fromHash = false) {
    if (focus) setFocus(false);
    stopLightUp(); gaps = on; sel = null; hideCard();
    if (gaps) gapsCaption(); else { selectFlow(null, fromHash); return; }
    if (!fromHash) setHash('gaps');
    refreshButtons();
  }
  $('#btnGaps').onclick = () => setGaps(!gaps);
  $('#btnLight').onclick = () => startLightUp();
  document.querySelectorAll<HTMLButtonElement>('.seg button').forEach(b => b.onclick = () => {
    if (!focus) { goView(b.dataset.view as View); return; }
    view = b.dataset.view as View; toggleFocus(false); // leaving the Focus view lands on this camera
  });
  function setCoverage(r: number) { const p = Math.round(r * 100); $('#covTxt').textContent = p + '%'; $('#covBar').style.width = p + '%'; }
  function hideCard() { card.classList.remove('on'); }
  function showCard(n: UN) {
    const u = n.u, k = KIND_NAME[u.kind];
    let h = `<button class="x" aria-label="Close">×</button><div class="k">${esc(u.layerName)} · ${esc(u.districtName)}</div><h2>${esc(u.name)}</h2><div class="kind ${k[1]}">${k[0]}</div>`;
    if (u.flows.length) {
      h += `<div class="sub">On ${u.flows.length} documented flow${u.flows.length > 1 ? 's (an interchange)' : ''}</div><div class="fl">` +
        u.flows.map(f => `<button class="mini" data-f="${f.index}"><i style="background:${esc(f.color)}"></i>${esc(f.name)}</button>`).join('') + '</div>';
    } else h += `<div class="gapnote">No documented flow passes through this. Either a path codedocs hasn’t documented yet, or dead code.</div>`;
    const href = H.codeHref(u);
    if (href) h += `<a class="code" href="${esc(href)}" target="_blank" rel="noopener">Open in code viewer →<span>${esc(u.file)}${u.lines ? ':' + u.lines[0] + '-' + u.lines[1] : ''}</span></a>`;
    if (u.docRef) h += `<a class="code doc" href="${esc(H.pageHref(u.docRef))}" data-ref="${esc(u.docRef)}">Read about it →</a>`;
    card.innerHTML = h; card.classList.add('on');
    card.querySelector<HTMLButtonElement>('.x')!.onclick = hideCard;
    card.querySelectorAll<HTMLButtonElement>('.mini').forEach(b => b.onclick = () => selectFlow(FL[+b.dataset.f!]));
    const a = card.querySelector<HTMLAnchorElement>('a.code:not(.doc)');
    if (a) a.onclick = e => { if (H.openCode(u)) e.preventDefault(); };
  }
  document.addEventListener('click', e => {
    const a = (e.target as HTMLElement).closest<HTMLAnchorElement>('a[data-ref]');
    if (a && H.openPage(a.dataset.ref!)) e.preventDefault();
  });
  addEventListener('keydown', e => { if (e.key === 'Escape') hideCard(); });

  // ---------- light-up ----------
  function startLightUp() {
    if (focus) setFocus(false);
    sel = null; gaps = false; hideCard(); setHash('');
    if (reduced) { // no animation: show where it ends
      un.forEach(n => n.color.copy(n.u.kind === 'code' ? C.code : C.eff));
      setCoverage(m.covered / total); setGaps(true); return;
    }
    lu.active = true; lu.fi = 0; lu.p = 0; lu.pause = 0.5; lu.lit.clear();
    tubes.geometry.setDrawRange(0, 0); headVisible.fill(false);
    setCoverage(0); cap.innerHTML = '<span class="st">Running /codedocs…</span>'; refreshButtons();
  }
  function updateLightUp(dt: number) {
    if (!lu.active) return;
    if (lu.pause > 0) { lu.pause -= dt; return; }
    if (!FL.length) { lu.active = false; setGaps(true); return; }
    const r = FL[lu.fi];
    if (lu.p === 0) cap.innerHTML = `<i class="dot" style="background:${esc(r.f.color)}"></i><span class="st">documenting flow ${lu.fi + 1}/${FL.length}</span><b>${esc(r.f.name)}</b>`;
    lu.p = Math.min(1, lu.p + dt / PER_FLOW);
    tubes.geometry.setDrawRange(0, r.start + Math.floor(lu.p * r.segs) * 36);
    r.f.steps.forEach((s, i) => {
      if (r.stepT[i] <= lu.p + 1e-6 && !lu.lit.has(s.unit.index)) {
        lu.lit.add(s.unit.index); const n = un[s.unit.index]; n.pop = 1; n.labelT = 1.4;
        if (s.effect) spawnRing(n.pos, 0.9, s.effect);
      }
    });
    positionHead(r, lu.p); headVisible[r.f.index] = true; headFade[r.f.index] = 1;
    setCoverage(lu.lit.size / total);
    if (lu.p >= 1) {
      tubes.geometry.setDrawRange(0, r.start + r.count); headVisible[r.f.index] = false;
      lu.fi++; lu.p = 0; lu.pause = BETWEEN;
      if (lu.fi >= FL.length) { lu.active = false; tubes.geometry.setDrawRange(0, Infinity); setGaps(true); }
    }
  }

  // ---------- per frame ----------
  const hp = new THREE.Vector3();
  function positionHead(r: FR, t: number) {
    const fi = r.f.index;
    for (let k = 0; k < TRAIL; k++) {
      const tt = Math.max(0, t - k * 0.22 / r.total);
      r.path.getPoint(Math.min(1, tt), hp);
      const s = k === 0 ? 1 : 0.85 * (1 - k / 9);
      heads.setMatrixAt(fi * TRAIL + k, mtx.compose(hp, q.identity(), sv.setScalar(s)));
    }
  }
  function hideHead(fi: number) { for (let k = 0; k < TRAIL; k++) heads.setMatrixAt(fi * TRAIL + k, mtx.makeScale(0, 0, 0)); }
  function arrive(r: FR, i: number) {
    const s = r.f.steps[i], n = un[s.unit.index];
    n.pop = Math.max(n.pop, r === sel ? 1 : 0.6);
    if (s.effect) spawnRing(n.show, r === sel ? 1 : 0.45, s.effect);
    if (r === sel) setCaption(r, i);
  }
  let tubeOpacity = 0;
  function updateFlows(dt: number) {
    const target = lu.active ? 0.9 : gaps ? 0.1 : sel ? 0.05 * (1 - fe) : 0;
    tubeOpacity += (target - tubeOpacity) * Math.min(1, dt * 5);
    tubes.material.opacity = tubeOpacity;
    for (const r of FL) {
      if (r.tube) {
        const mt = r.tube.material, tgt = sel === r && !lu.active ? 0.95 : 0;
        mt.opacity += (tgt - mt.opacity) * Math.min(1, dt * 5);
        r.tube.visible = mt.opacity > 0.005;
      }
      if (r.stubs) r.stubs.visible = sel === r && !lu.active;
      const fi = r.f.index;
      if (lu.active) { if (r !== FL[lu.fi]) headVisible[fi] = false; continue; }
      const active = r === sel;
      if (!active || reduced) { headVisible[fi] = false; continue; }
      const run = r.run;
      if (run.hold > 0) { run.hold -= dt; headVisible[fi] = false; if (run.hold <= 0) { run.t = 0; run.idx = -1; } continue; }
      run.t = Math.min(1, run.t + dt * (r === sel ? 4.6 : 3.6) / r.total);
      while (run.idx + 1 < r.stepT.length && r.stepT[run.idx + 1] <= run.t + 1e-6) { run.idx++; arrive(r, run.idx); }
      if (run.t >= 1) run.hold = r === sel ? 1.4 : 0.5;
      headVisible[fi] = true; headFade[fi] = r === sel ? 1 : 0.7;
      positionHead(r, run.t);
    }
    // Trails dim when not the selected flow.
    const col = heads.instanceColor!;
    FL.forEach((r, fi) => {
      if (!headVisible[fi]) { hideHead(fi); return; }
      const c = tmpC.set(r.f.color), o = headFade[fi];
      for (let k = 1; k < TRAIL; k++) col.setXYZ(fi * TRAIL + k, c.r * 0.75 * (1 - k / TRAIL) * o, c.g * 0.75 * (1 - k / TRAIL) * o, c.b * 0.75 * (1 - k / TRAIL) * o);
    });
    col.needsUpdate = true;
    heads.instanceMatrix.needsUpdate = true;
  }
  const ease = (t: number) => t >= 1 ? 1 : t < 0.5 ? 4 * t ** 3 : 1 - Math.pow(-2 * t + 2, 3) / 2;
  // Units on their way between the slabs and the Focus column, the fade of everything the
  // Focus view leaves out, and the paths that follow the moving units.
  function updateMoves(dt: number) {
    let moving = 0, hitDirty = false;
    fe += ((FX ? 1 : 0) - fe) * (reduced ? 1 : Math.min(1, dt * 3.2));
    for (const n of un) {
      const tv = FX ? (FX.at.has(n.u.index) ? 1 : 0) : 1;
      if (n.mt >= 1 && n.vis === tv) continue;
      if (n.mt < 1) {
        n.mt = Math.min(1, n.mt + dt / 1.2);
        n.show.lerpVectors(n.from, n.to, ease(n.mt));
        moving++;
        for (const f of n.u.flows) FL[f.index].dirty = true;
      }
      n.vis = reduced || Math.abs(tv - n.vis) < 0.01 ? tv : n.vis + (tv - n.vis) * Math.min(1, dt * 4);
      hit.setMatrixAt(n.u.index, mtx.compose(n.show, q.identity(), sv.setScalar(n.vis > 0.5 ? 1 : 0)));
      hitDirty = true;
    }
    if (hitDirty) { hit.instanceMatrix.needsUpdate = true; hit.computeBoundingSphere(); }
    // The selected flow follows its units every frame; the rest catch up once they've landed.
    for (const r of FL) if (r.dirty && (r === sel || !moving)) rebuildPath(r);
    const k = 1 - fe;
    slabMat.opacity = 0.055 * k; edgeMat.opacity = 0.55 * k; gridMat.opacity = 0.07 * k;
    districtMat.opacity = 0.28 * k; sheetRingMat.opacity = 0.22 * k;
    laneMat.opacity = 0.16 * fe; dustMat.opacity = 0.7 * (1 - 0.75 * fe);
  }
  function updateNodes(dt: number, time: number) {
    const k = Math.min(1, dt * 6);
    const selF = sel?.f;
    for (const n of un) {
      const u = n.u, lit = u.kind === 'code' ? C.code : C.eff, covered = u.flows.length > 0;
      let tgt: THREE.Color;
      if (lu.active) tgt = lu.lit.has(u.index) ? lit : C.dark;
      else if (gaps) tgt = covered ? tmpC.copy(lit).lerp(C.dark, 0.72) : C.gap;
      else if (!covered) tgt = C.dark;
      else if (selF) tgt = u.flows.includes(selF) ? lit : tmpC.copy(lit).lerp(C.dark, 0.8);
      else tgt = lit;
      n.color.lerp(tgt, k);
      n.mesh.setColorAt(n.slot, n.color);
      n.pop = Math.max(0, n.pop - dt * 2.2); n.labelT = Math.max(0, n.labelT - dt);
      let s = n.base * NS * (1 + 0.55 * n.pop) * (hovered === n ? 1.25 : 1) * n.vis;
      if (gaps && !covered && !lu.active && !reduced) s *= 1 + 0.12 * Math.sin(time * 4);
      q.setFromEuler(eul.set(0, n.spin && !reduced ? time * 0.6 : 0, 0));
      n.mesh.setMatrixAt(n.slot, mtx.compose(n.show, q, sv.setScalar(s)));
    }
    for (const mesh of kindMesh.values()) { mesh.instanceMatrix.needsUpdate = true; if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true; }
    if (gates) {
      ext.forEach((u, i) => {
        q.setFromEuler(eul.set(reduced ? 0 : Math.sin(time * 0.3) * 0.4, reduced ? 0 : time * 0.5, 0));
        gates.setMatrixAt(i, mtx.compose(un[u.index].show, q, sv.setScalar(NS * un[u.index].vis)));
      });
      gates.instanceMatrix.needsUpdate = true;
    }
  }
  function updateRings(dt: number) {
    for (let i = rings.length - 1; i >= 0; i--) {
      const r = rings[i]; r.t += dt / 0.9;
      if (r.t >= 1) { scene.remove(r.m); r.m.material.dispose(); rings.splice(i, 1); continue; }
      r.m.scale.setScalar(NS * (1 + r.t * 2.6)); r.m.material.opacity = r.s * (1 - r.t);
    }
  }
  const clock = new THREE.Clock();
  let raf = 0;
  function frame() {
    const dt = Math.min(clock.getDelta(), 0.05), time = clock.elapsedTime;
    if (tween) {
      tween.t += dt / tween.d;
      const e = ease(tween.t);
      camera.position.lerpVectors(tween.fp, tween.tp, e); controls.target.lerpVectors(tween.ft, tween.tt, e);
      if (tween.t >= 1) tween = null;
    }
    controls.update();
    updateLightUp(dt);
    updateMoves(dt);
    updateFlows(dt);
    updateNodes(dt, time); updateRings(dt);
    if (composer) composer.render(); else renderer.render(scene, camera);
    updateLabels();
    raf = requestAnimationFrame(frame);
  }

  // ---------- picking ----------
  const ray = new THREE.Raycaster(), mp = new THREE.Vector2();
  function pick(cx: number, cy: number): UN | null {
    mp.set(cx / innerWidth * 2 - 1, -(cy / innerHeight) * 2 + 1);
    ray.setFromCamera(mp, camera);
    const h = ray.intersectObject(hit, false);
    return h.length && h[0].instanceId != null ? un[h[0].instanceId] : null;
  }
  let down: { x: number; y: number } | null = null;
  const cv = renderer.domElement;
  cv.addEventListener('pointerdown', e => { down = { x: e.clientX, y: e.clientY }; });
  cv.addEventListener('pointerup', e => {
    if (!down) return;
    const d = Math.hypot(e.clientX - down.x, e.clientY - down.y); down = null;
    if (d > 8) return;
    const n = pick(e.clientX, e.clientY);
    if (n) showCard(n); else hideCard();
  });
  let hoverQueued: PointerEvent | null = null;
  cv.addEventListener('pointermove', e => {
    if (e.pointerType !== 'mouse') return;
    if (!hoverQueued) requestAnimationFrame(() => { const ev = hoverQueued!; hoverQueued = null; hovered = pick(ev.clientX, ev.clientY); cv.style.cursor = hovered ? 'pointer' : 'grab'; });
    hoverQueued = e;
  });
  cv.addEventListener('pointerleave', () => { hovered = null; });
  addEventListener('resize', () => {
    renderer.setSize(innerWidth, innerHeight); setFov(); camera.aspect = innerWidth / innerHeight; camera.updateProjectionMatrix();
    composer?.setSize(innerWidth, innerHeight);
    if (FX) setFocus(true, true); // the column moves left on a narrow screen
    else if (!userMoved) goView(view, true);
  });

  // ---------- deep links: #flow-<id>, #focus-<id>, #gaps, #lightup (#all, from before, picks nothing) ----------
  function fromHash(): boolean {
    const h = decodeURIComponent(location.hash.replace(/^#/, ''));
    const fm = h.match(/^(flow|focus)-(.+)$/);
    if (fm) {
      const r = FL.find(r => r.f.id === fm[2]);
      if (r) { selectFlow(r, true); if ((fm[1] === 'focus') !== focus) toggleFocus(fm[1] === 'focus', true); return true; }
    }
    if (h === 'gaps') { setGaps(true, true); return true; }
    if (h === 'all') { selectFlow(null, true); return true; }
    if (h === 'lightup') { startLightUp(); return true; }
    return false;
  }
  addEventListener('hashchange', () => { fromHash(); });

  refreshButtons();
  if (!fromHash()) {
    let seen = false;
    try { seen = sessionStorage.getItem('codedocs-flowview-lit') === '1'; sessionStorage.setItem('codedocs-flowview-lit', '1'); } catch { /* storage blocked */ }
    // The first visit in a session opens with the light-up; reduced motion jumps to its end.
    if (seen) selectFlow(null, true);
    else { startLightUp(); if (!reduced) lu.pause = 1.4; }
  }
  if (!lu.active) setCoverage(m.covered / total);
  frame();

  // For tests, scripted recordings and debugging from the console.
  (window as any).__flowview = {
    selectFlow: (id: string | null) => selectFlow(id ? FL.find(r => r.f.id === id) ?? null : null),
    setGaps, startLightUp, goView, focus: (on = true) => toggleFocus(on), showCard: (id: string) => { const u = m.byId.get(id); if (u) showCard(un[u.index]); },
    state: () => ({ mode: lu.active ? 'lightup' : gaps ? 'gaps' : FX ? 'focus' : sel ? 'flow' : 'none', flow: sel?.f.id ?? null, view, covered: m.covered, total, labels: [...pool.values()].filter(l => l.on).length }),
    layout: L, bloom: () => bloomPass, renderer,
  };
  return { dispose() { cancelAnimationFrame(raf); renderer.dispose(); } };
}
