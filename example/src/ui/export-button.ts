import { Timeline } from '../timeline/timeline';
import { RenderQueue } from '../render/queue';

export function mountExportButton(el: HTMLButtonElement, timeline: Timeline, queue: RenderQueue) {
  el.addEventListener('click', () => {
    if (timeline.duration() === 0) {
      alert('Nothing to export');
      return;
    }
    const job = queue.enqueue(timeline.snapshot());
    el.textContent = `Exporting #${job.id}…`;
  });
}
