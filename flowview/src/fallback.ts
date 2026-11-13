// Without WebGL: the Side view as a static SVG, drawn from the same layout. Layers become
// swimlanes, so it doubles as a 2D process diagram.

import type { Model } from './graph';
import type { Layout } from './layout';

const esc = (s: unknown) => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!));

export function renderFallback(m: Model, L: Layout, host: HTMLElement, reason: string) {
  // Oblique side view: x across, layers down, depth nudged so units behind don't hide.
  const sx = (x: number, z: number) => x + z * 0.32;
  const sy = (y: number, z: number) => -y * 1.4 + z * 0.1;
  const pts = m.units.map(u => { const p = L.pos[u.index]; return { x: sx(p.x, p.z), y: sy(p.y, p.z) }; });
  const xs = pts.map(p => p.x).concat([sx(-L.slabW / 2, -L.slabD / 2) - 2, sx(L.slabW / 2, L.slabD / 2) + 2]);
  const ys = L.layers.map(l => sy(l.y, 0));
  const minX = Math.min(...xs) - 6, maxX = Math.max(...xs) + 3;
  const minY = Math.min(...ys) - L.gap * 0.9, maxY = Math.max(...ys) + L.gap * 0.9;
  const k = 22; // px per unit
  const W = (maxX - minX) * k, Hh = (maxY - minY) * k;
  const X = (x: number) => ((x - minX) * k).toFixed(1), Y = (y: number) => ((y - minY) * k).toFixed(1);
  let s = `<svg viewBox="0 0 ${W.toFixed(0)} ${Hh.toFixed(0)}" role="img" aria-label="Side view of the documented flows" xmlns="http://www.w3.org/2000/svg">`;
  L.layers.forEach(l => {
    const y = sy(l.y, 0);
    s += `<rect x="0" y="${Y(y - L.gap * 0.42)}" width="${W.toFixed(0)}" height="${(L.gap * 0.84 * k).toFixed(0)}" rx="10" class="lane"/>`;
    s += `<text x="10" y="${Y(y - L.gap * 0.42 + 0.9)}" class="lane-name">${esc(l.name)}</text>`;
  });
  m.flows.forEach((f, fi) => {
    const o = ((fi % 8) - (Math.min(m.flows.length, 8) - 1) / 2) * 0.2;
    const d = f.steps.map((st, i) => { const p = pts[st.unit.index]; return (i ? 'L' : 'M') + X(p.x + o) + ' ' + Y(p.y + o); }).join(' ');
    s += `<path d="${d}" class="flow" stroke="${esc(f.color)}"><title>${esc(f.name)}</title></path>`;
  });
  m.units.forEach(u => {
    const p = pts[u.index], on = u.flows.length > 0, eff = u.kind !== 'code';
    const cls = !on ? 'dark' : eff ? 'eff' : 'lit';
    s += u.kind === 'code'
      ? `<circle cx="${X(p.x)}" cy="${Y(p.y)}" r="5" class="${cls}"><title>${esc(u.name)}</title></circle>`
      : `<rect x="${(+X(p.x) - 6).toFixed(1)}" y="${(+Y(p.y) - 6).toFixed(1)}" width="12" height="12" rx="${u.kind === 'external' ? 1 : 4}" class="${cls}" transform="rotate(${u.kind === 'external' ? 45 : 0} ${X(p.x)} ${Y(p.y)})"><title>${esc(u.name)}</title></rect>`;
    if (!eff && on && u.flows[0].steps[0].unit === u || u.kind !== 'code') s += `<text x="${X(p.x)}" y="${(+Y(p.y) - 10).toFixed(1)}" class="name">${esc(u.name)}</text>`;
  });
  s += '</svg>';
  const pct = m.units.length ? Math.round(m.covered / m.units.length * 100) : 0;
  host.innerHTML = `<div class="fb-head"><b>${esc(m.graph.repo.name)}</b> · ${m.flows.length} flows · ${m.covered} of ${m.units.length} units on a documented flow (${pct}%)` +
    `<span>${esc(reason)} This is the side view as a still picture.</span></div><div class="fb-svg">${s}</div>` +
    `<div class="fb-legend">${m.flows.map(f => `<span><i style="background:${esc(f.color)}"></i>${esc(f.name)}</span>`).join('')}</div>`;
  host.hidden = false;
}
