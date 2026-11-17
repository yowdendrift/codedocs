// The model the view works from: flowgraph.json with references resolved and the derived
// facts (which flows pass each unit, coverage, colours) worked out once.

import type { Effect, FlowGraph, UnitKind } from './types';

// At least 8 distinguishable flow colours. The first four are the mockup's.
export const PALETTE = ['#a78bfa', '#2dd4bf', '#fbbf24', '#f472b6', '#60a5fa', '#a3e635', '#e879f9', '#22d3ee', '#fda4af', '#bef264'];

export interface Unit {
  index: number;
  id: string;
  name: string;
  kind: UnitKind;
  layer: string | null;      // null for externals
  district: string | null;
  districtName: string;
  layerName: string;
  file?: string;
  lines?: [number, number];
  docRef?: string;
  flows: Flow[];             // distinct flows through it, in flow order
  // On no flow, but called from one: the flow (null if the view leaves that flow out) and the
  // unit that calls it. The builder works it out by name matching, so it's approximate.
  reached: { flow: Flow | null; flowName: string; via: Unit } | null;
}

export interface Step { unit: Unit; label: string; effect?: Effect }
export interface Branch { at: number; label: string; target?: string }

export interface Flow {
  index: number;
  id: string;
  name: string;
  color: string;
  docRef?: string;
  steps: Step[];
  branches: Branch[];
}

export interface Model {
  graph: FlowGraph;
  units: Unit[];
  byId: Map<string, Unit>;
  flows: Flow[];
  covered: number;           // units on at least one flow
  reached: number;           // units on none, but called from one
}

export class GraphError extends Error {}

export function buildModel(g: FlowGraph): Model {
  if (!g || g.version !== 1) throw new GraphError(`This view reads flowgraph version 1, but the file says ${g ? g.version : 'nothing'}.`);
  const layerName = new Map(g.layers.map(l => [l.id, l.name]));
  const districtName = new Map(g.districts.map(d => [d.id, d.name]));
  const problems: string[] = [];
  const units: Unit[] = g.units.map((u, index) => {
    if (u.kind !== 'external' && (!u.layer || !layerName.has(u.layer))) problems.push(`unit ${u.id}: unknown layer ${u.layer}`);
    if (u.kind !== 'external' && (!u.district || !districtName.has(u.district))) problems.push(`unit ${u.id}: unknown district ${u.district}`);
    return {
      index, id: u.id, name: u.name, kind: u.kind,
      layer: u.kind === 'external' ? null : u.layer ?? null,
      district: u.kind === 'external' ? null : u.district ?? null,
      districtName: u.kind === 'external' ? 'outside the system' : districtName.get(u.district!) ?? '',
      layerName: u.kind === 'external' ? 'External' : layerName.get(u.layer!) ?? '',
      file: u.file, lines: u.lines, docRef: u.docRef, flows: [], reached: null,
    };
  });
  const byId = new Map(units.map(u => [u.id, u]));
  // System lens only in v1: ui-lens flows and sub-flows are skipped.
  const flows: Flow[] = [];
  for (const f of g.flows) {
    if ((f.lens ?? 'system') !== 'system' || f.parent) continue;
    const steps: Step[] = [];
    for (const s of f.steps) {
      const u = byId.get(s.unit);
      if (!u) { problems.push(`flow ${f.id}: unknown unit ${s.unit}`); continue; }
      steps.push({ unit: u, label: s.label, effect: s.effect });
    }
    if (!steps.length) continue;
    const index = flows.length;
    flows.push({
      index, id: f.id, name: f.name, docRef: f.docRef,
      color: f.color || PALETTE[index % PALETTE.length],
      steps,
      branches: (f.branches ?? []).filter(b => b.atStep >= 0 && b.atStep < steps.length).map(b => ({ at: b.atStep, label: b.label, target: b.target })),
    });
  }
  if (problems.length) throw new GraphError('flowgraph.json has references that don’t resolve:\n' + problems.slice(0, 8).join('\n') + (problems.length > 8 ? `\n…and ${problems.length - 8} more` : ''));
  for (const f of flows) for (const s of f.steps) if (!s.unit.flows.includes(f)) s.unit.flows.push(f);
  // Reached: kept only for units the view shows on no flow, with a caller that resolves.
  const flowById = new Map(flows.map(f => [f.id, f]));
  const flowName = new Map(g.flows.map(f => [f.id, f.name]));
  g.units.forEach((r, i) => {
    const via = r.reached && byId.get(r.reached.via);
    if (r.reached && via && !units[i].flows.length)
      units[i].reached = { flow: flowById.get(r.reached.flow) ?? null, flowName: flowName.get(r.reached.flow) ?? r.reached.flow, via };
  });
  return { graph: g, units, byId, flows, covered: units.filter(u => u.flows.length).length, reached: units.filter(u => u.reached).length };
}

// Deterministic 0..1 from a string: flow stagger, dust, anything that must repeat run to run.
export function hash01(s: string): number {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); }
  return (h >>> 0) / 4294967296;
}

export function rng(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
