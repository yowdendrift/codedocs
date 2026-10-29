// The timeline: tracks of clips placed in time.
export interface Clip {
  id: string;
  mediaId: string;
  start: number;    // seconds on the timeline
  inPoint: number;  // seconds into the source media
  duration: number;
}

export interface Track {
  id: string;
  kind: 'video' | 'audio';
  clips: Clip[];
}

export class Timeline {
  tracks: Track[] = [];
  private listeners = new Set<() => void>();

  addClip(trackId: string, clip: Clip) {
    const track = this.tracks.find(t => t.id === trackId);
    if (!track) throw new Error(`No track ${trackId}`);
    track.clips.push(clip);
    track.clips.sort((a, b) => a.start - b.start);
    this.emit();
  }

  duration(): number {
    return Math.max(0, ...this.tracks.flatMap(t => t.clips.map(c => c.start + c.duration)));
  }

  snapshot(): Track[] {
    return structuredClone(this.tracks);
  }

  onChange(fn: () => void) {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  private emit() {
    this.listeners.forEach(fn => fn());
  }
}
