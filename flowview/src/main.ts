// Entry point of flowview.js: load the graph, lay it out, start the 3D view (or the SVG
// fallback without WebGL), and connect to the docs viewer when embedded in it.
//
// The graph comes from window.__FLOWGRAPH__ (flowgraph.js, loaded by a script tag, which
// works from file://) or else from flowgraph.json. ?src=<path> loads another graph file from
// the same site, e.g. ?src=design/flowgraph.sample.json.

import { buildModel, GraphError, type Unit } from './graph';
import { computeLayout } from './layout';
import { start, type Hooks } from './app';
import { renderFallback } from './fallback';
import type { FlowGraph } from './types';

const params = new URLSearchParams(location.search);
// Embedded in the docs viewer (index.html opens flowview.html?embed in a frame).
const embedded = params.has('embed') && window.parent !== window;
// ...and the docs viewer has a code panel to open files in (&code=1).
const codePanel = embedded && params.get('code') === '1';

async function loadGraph(): Promise<FlowGraph> {
  const src = params.get('src');
  if (src && /^[\w./-]+\.json$/.test(src) && !src.includes('..')) {
    const r = await fetch(src, { cache: 'no-store' });
    if (!r.ok) throw new Error(`Couldn’t load ${src} (${r.status}).`);
    return r.json();
  }
  if (window.__FLOWGRAPH__) return window.__FLOWGRAPH__;
  if (location.protocol === 'file:') throw new Error('No flow graph next to this page. Run /codedocs flows to make one.');
  const r = await fetch('flowgraph.json', { cache: 'no-store' });
  if (!r.ok) throw new Error('No flow graph yet. Run /codedocs flows to make one.');
  return r.json();
}

function hooks(g: FlowGraph): Hooks {
  const post = (msg: object) => { window.parent.postMessage({ codedocs: 1, ...msg }, location.origin === 'null' ? '*' : location.origin); return true; };
  const root = (g.repo.root ?? '..').replace(/\/$/, '');
  return {
    codeHref(u: Unit) {
      if (!u.file) return null;
      if (g.repo.web) return g.repo.web + u.file.split('/').map(encodeURIComponent).join('/') + (u.lines ? `#L${u.lines[0]}-L${u.lines[1]}` : '');
      return root + '/' + u.file;
    },
    openCode(u: Unit) { return codePanel && !!u.file && post({ type: 'open-code', file: u.file, line: u.lines?.[0] ?? 0, end: u.lines?.[1] ?? 0 }); },
    pageHref(ref: string) { return 'index.html#/' + ref; },
    openPage(ref: string) { return embedded && post({ type: 'open-page', ref }); },
    hashChanged(h: string) { if (embedded) post({ type: 'hash', hash: h }); },
  };
}

function webgl(): boolean {
  if (params.has('nowebgl')) return false;
  try { const c = document.createElement('canvas'); return !!(c.getContext('webgl2') || c.getContext('webgl')); } catch { return false; }
}

function fail(msg: string) {
  const el = document.getElementById('fallback')!;
  el.innerHTML = `<p class="fb-msg">${msg.replace(/[&<>]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]!)).replace(/\n/g, '<br>')}</p>`;
  el.hidden = false;
  document.getElementById('sheet')!.style.display = 'none';
}

async function main() {
  let g: FlowGraph;
  try { g = await loadGraph(); } catch (e) { return fail((e as Error).message); }
  let model;
  try { model = buildModel(g); } catch (e) { return fail(e instanceof GraphError ? e.message : 'flowgraph.json couldn’t be read: ' + (e as Error).message); }
  const layout = computeLayout(model);
  document.title = `${g.repo.name} · flow view`;
  const brand = document.querySelector('.brand h1');
  if (brand) brand.textContent = `${g.repo.name} · flow view`;
  if (embedded) document.body.classList.add('embedded');
  const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches || params.has('still');
  if (!webgl()) {
    document.getElementById('sheet')!.style.display = 'none';
    document.querySelector('header .pill')?.remove();
    return renderFallback(model, layout, document.getElementById('fallback')!, 'This browser can’t draw 3D (no WebGL).');
  }
  try { start(model, layout, { reduced, hooks: hooks(g) }); }
  catch (e) {
    document.getElementById('stage')!.replaceChildren();
    document.getElementById('sheet')!.style.display = 'none';
    renderFallback(model, layout, document.getElementById('fallback')!, 'The 3D view couldn’t start (' + (e as Error).message + ').');
  }
}

main();
