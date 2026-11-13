// Dev server for dist/ (npm run build first): http://localhost:8790/flowview.html
// Files not in dist/ are looked up in ../docs, so ?src=design/flowgraph.sample.json works.
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { extname, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = fileURLToPath(new URL('..', import.meta.url));
const roots = [join(here, 'dist'), join(here, '..', 'docs')];
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.json': 'application/json' };
const port = +(process.env.PORT || 8790);
createServer(async (req, res) => {
  const p = normalize(decodeURIComponent(new URL(req.url, 'http://x').pathname)).replace(/^[\\/]+/, '') || 'flowview.html';
  if (p.startsWith('..')) { res.writeHead(403).end(); return; }
  for (const r of roots) {
    try {
      const b = await readFile(join(r, p));
      res.writeHead(200, { 'Content-Type': (types[extname(p)] || 'application/octet-stream') + '; charset=utf-8', 'Cache-Control': 'no-store' });
      res.end(b); return;
    } catch {}
  }
  res.writeHead(404).end('not found');
}).listen(port, '127.0.0.1', () => console.log(`http://localhost:${port}/flowview.html`));
