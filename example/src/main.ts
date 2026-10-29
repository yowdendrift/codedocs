import { Timeline } from './timeline/timeline';
import { RenderQueue } from './render/queue';
import { mountExportButton } from './ui/export-button';

const timeline = new Timeline();
timeline.tracks.push({ id: 'v1', kind: 'video', clips: [] });
const queue = new RenderQueue();
mountExportButton(document.querySelector('#export')!, timeline, queue);
