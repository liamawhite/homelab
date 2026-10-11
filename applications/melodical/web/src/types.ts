export type Point = { midi: number; string?: number; fret?: number };
export type Settings = {
  instrument: string;
  topic: string;
  key: string;
  difficulty: string;
  mode: string;
  count: number;
  minutes: number;
  sound: boolean;
  theme: string;
};
export type Question = {
  id: string;
  instrument: string;
  topic: string;
  kind: string;
  prompt: string;
  detail: string;
  key: string;
  root: number;
  level: number;
  single: boolean;
  minMidi: number;
  maxMidi: number;
  minFret: number;
  maxFret: number;
  targetString?: number;
  labels: string[];
  display?: Point[];
  choices?: string[];
  degrees: boolean;
};
export type Feedback = {
  correct: boolean;
  expected: Point[];
  choice?: string;
  explanation: string;
};
export type Review = {
  skill: string;
  instrument: string;
  topic: string;
  key: string;
  attempts: number;
  correct: number;
  unaided: number;
  speedTotal: number;
  speedCount: number;
  streak: number;
  dueAt: number;
  lastSeen: number;
};
export type Stats = {
  reviews: Review[];
  recent: {
    instrument: string;
    topic: string;
    key: string;
    correct: boolean;
    assisted: boolean;
    createdAt: number;
  }[];
};
export type Session = {
  settings: Settings;
  question: Question | null;
  points: Point[];
  choice: string;
  feedback: Feedback | null;
  assisted: boolean;
  interrupted: boolean;
  answerMs: number;
  activeMs: number;
  answered: number;
  correct: number;
  unaided: number;
  speedTotal: number;
  speedCount: number;
  retries: { id: string; after: number }[];
  updatedAt: number;
};
export const defaults: Settings = {
  instrument: 'both',
  topic: 'mixed',
  key: 'auto',
  difficulty: 'auto',
  mode: 'quick',
  count: 5,
  minutes: 3,
  sound: false,
  theme: 'system',
};
export const topics: Record<string, string> = {
  mixed: 'A little of everything',
  notes: 'Note finding',
  keys: 'Notes in a key',
  intervals: 'Intervals',
  chords: 'Triads',
  inversions: 'Chord inversions',
};
export const keys = [
  'C',
  'G',
  'D',
  'A',
  'E',
  'B',
  'F#',
  'F',
  'Bb',
  'Eb',
  'Ab',
  'Db',
];
export const keyName = (k: string) => k.replace('#', '♯').replace('b', '♭');
export const pc = (m: number) => ((m % 12) + 12) % 12;
export const pointID = (p: Point) =>
  `${p.midi}:${p.string ?? 0}:${p.fret ?? 0}`;
export async function api<T>(
  path: string,
  data?: unknown,
  method = 'POST',
): Promise<T> {
  const response = await fetch(
    `/api/${path}`,
    data === undefined
      ? undefined
      : {
          method,
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(data),
        },
  );
  if (!response.ok) {
    const result = await response.json().catch(() => null);
    throw new Error(
      result?.error ||
        'Could not reach your practice server. Please try again.',
    );
  }
  return response.json();
}
let context: AudioContext | null = null;
export function play(points: Point[], arpeggio = false) {
  try {
    context ??= new AudioContext();
    void context.resume();
    const now = context.currentTime;
    [...new Set(points.map((p) => p.midi))]
      .sort((a, b) => a - b)
      .forEach((m, i) => {
        const oscillator = context!.createOscillator(),
          gain = context!.createGain(),
          start = now + (arpeggio ? i * 0.14 : 0);
        oscillator.type = 'triangle';
        oscillator.frequency.value = 440 * 2 ** ((m - 69) / 12);
        gain.gain.setValueAtTime(0, start);
        gain.gain.linearRampToValueAtTime(
          0.12 / Math.max(1, points.length),
          start + 0.015,
        );
        gain.gain.exponentialRampToValueAtTime(0.001, start + 0.85);
        oscillator.connect(gain);
        gain.connect(context!.destination);
        oscillator.start(start);
        oscillator.stop(start + 0.9);
      });
  } catch {
    /* Visual practice remains available when audio is unsupported. */
  }
}
