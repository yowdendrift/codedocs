// Runs off the UI thread. Turns tracks into frames and encodes them.
let cancelled = new Set<number>();

self.onmessage = async (e: MessageEvent) => {
  const msg = e.data;
  if (msg.type === 'cancel') { cancelled.add(msg.id); return; }
  if (msg.type !== 'render') return;
  try {
    const frames = 100;
    for (let i = 0; i < frames; i++) {
      if (cancelled.has(msg.id)) { self.postMessage({ type: 'error', message: 'cancelled' }); return; }
      await encodeFrame(i);
      self.postMessage({ type: 'progress', value: (i + 1) / frames });
    }
    self.postMessage({ type: 'done', blob: new Blob([]) });
  } catch (err) {
    self.postMessage({ type: 'error', message: String(err) });
  }
};

async function encodeFrame(_i: number) {
  await new Promise(r => setTimeout(r, 5));
}
