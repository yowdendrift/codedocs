// flowgraph.json, version 1. schema/flowgraph.v1.json at the repo root is the definition;
// these types mirror it.

export type UnitKind = 'code' | 'store' | 'file' | 'external' | 'queue' | 'cache';
export type Effect = 'db-read' | 'db-write' | 'external' | 'disk-write' | 'queue' | 'cache';

export interface FlowGraph {
  version: 1;
  generatedAt: string;
  repo: { name: string; commit?: string | null; web?: string; root?: string };
  layers: { id: string; name: string; order: number }[];
  districts: { id: string; layer: string; name: string; path?: string }[];
  units: {
    id: string; name: string; kind: UnitKind;
    layer?: string; district?: string;
    file?: string; lines?: [number, number]; docRef?: string;
  }[];
  flows: {
    id: string; name: string; lens?: 'system' | 'ui'; color?: string | null;
    docRef?: string; parent?: string;
    steps: { unit: string; label: string; effect?: Effect }[];
    branches?: { atStep: number; label: string; target?: string }[];
  }[];
}

declare global {
  interface Window { __FLOWGRAPH__?: FlowGraph }
}
