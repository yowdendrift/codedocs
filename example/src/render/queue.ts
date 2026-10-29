import type { Track } from '../timeline/timeline';

export type JobState = 'queued' | 'rendering' | 'done' | 'failed' | 'cancelled';

export interface RenderJob {
  id: number;
  tracks: Track[];
  state: JobState;
  progress: number;
  output?: Blob;
  error?: string;
}

// One worker, created once and reused for every job.
const worker = new Worker(new URL('./encode.worker.ts', import.meta.url), { type: 'module' });

export class RenderQueue {
  private jobs: RenderJob[] = [];
  private nextId = 1;
  private running = false;

  enqueue(tracks: Track[]): RenderJob {
    const job: RenderJob = { id: this.nextId++, tracks, state: 'queued', progress: 0 };
    this.jobs.push(job);
    this.pump();
    return job;
  }

  cancel(id: number) {
    const job = this.jobs.find(j => j.id === id);
    if (job && job.state === 'queued') job.state = 'cancelled';
    if (job && job.state === 'rendering') worker.postMessage({ type: 'cancel', id });
  }

  private pump() {
    if (this.running) return;
    const job = this.jobs.find(j => j.state === 'queued');
    if (!job) return;
    this.running = true;
    job.state = 'rendering';
    worker.onmessage = (e: MessageEvent) => {
      const msg = e.data;
      if (msg.type === 'progress') job.progress = msg.value;
      if (msg.type === 'done') { job.state = 'done'; job.output = msg.blob; this.finish(); }
      if (msg.type === 'error') { job.state = 'failed'; job.error = msg.message; this.finish(); }
    };
    worker.postMessage({ type: 'render', id: job.id, tracks: job.tracks });
  }

  private finish() {
    this.running = false;
    this.pump();
  }
}
