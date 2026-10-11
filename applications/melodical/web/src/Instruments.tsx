import { useState } from 'react';
import { pc, pointID, type Point, type Question } from './types';
export function Keyboard({
  q,
  selected,
  expected,
  labels,
  onSelect,
  disabled = false,
}: {
  q: Question;
  selected: Point[];
  expected?: Point[];
  labels: boolean;
  onSelect: (p: Point) => void;
  disabled?: boolean;
}) {
  const [octave, setOctave] = useState(0);
  const selectedSet = new Set(selected.map((p) => p.midi));
  const expectedSet = new Set(expected?.map((p) => p.midi));
  const displaySet = new Set(q.display?.map((p) => p.midi));
  const whites: number[] = [];
  for (let m = q.minMidi; m <= q.maxMidi; m++) {
    if (![1, 3, 6, 8, 10].includes(pc(m))) whites.push(m);
  }
  const pages = Math.ceil((q.maxMidi - q.minMidi + 1) / 12);
  const start = Math.min(octave, pages - 1) * 12;
  const key = (m: number, black: boolean, index: number) => {
    const active = selectedSet.has(m),
      answer = expectedSet.has(m),
      given = displaySet.has(m);
    return (
      <button
        key={m}
        type="button"
        data-midi={m}
        className={`piano-key ${black ? 'black' : 'white'} ${active ? 'selected' : ''} ${answer ? 'expected' : ''} ${given ? 'given' : ''}`}
        style={
          black
            ? {
                left: `${((index + 1) * 100) / whites.length}%`,
                width: `${65 / whites.length}%`,
              }
            : {
                left: `${(index * 100) / whites.length}%`,
                width: `${100 / whites.length}%`,
              }
        }
        disabled={disabled}
        aria-label={
          labels
            ? `${q.labels[pc(m)]}${Math.floor(m / 12) - 1}`
            : `${black ? 'Black' : 'White'} key ${m - q.minMidi + 1}`
        }
        aria-pressed={active}
        onClick={() => onSelect({ midi: m })}
      >
        {(labels || active || answer || given) && (
          <span>{labels || answer ? q.labels[pc(m)] : given ? '●' : '•'}</span>
        )}
        {(active || answer) && <i />}
      </button>
    );
  };
  return (
    <div className="keyboard-wrap">
      <div className="octave-controls">
        <button
          onClick={() => setOctave(Math.max(0, octave - 1))}
          disabled={octave === 0}
          aria-label="Previous octave"
        >
          ←
        </button>
        <span>
          C{Math.floor(q.minMidi / 12) - 1 + octave}–B
          {Math.floor(q.minMidi / 12) - 1 + octave}{' '}
          <small>
            {octave + 1} / {pages}
          </small>
        </span>
        <button
          onClick={() => setOctave(Math.min(pages - 1, octave + 1))}
          disabled={octave >= pages - 1}
          aria-label="Next octave"
        >
          →
        </button>
      </div>
      <div className="piano-viewport">
        <div
          className="piano"
          style={
            { '--page-offset': `${(start / 12) * 100}%` } as React.CSSProperties
          }
        >
          {whites.map((m, i) => key(m, false, i))}
          {whites.flatMap((m, i) =>
            m < q.maxMidi && [1, 3, 6, 8, 10].includes(pc(m + 1))
              ? [key(m + 1, true, i)]
              : [],
          )}
        </div>
      </div>
      <div className="instrument-caption">
        <span>
          C{Math.floor(q.minMidi / 12) - 1} — B{Math.floor(q.maxMidi / 12) - 1}
        </span>
        <span>Tap to select · tap again to remove</span>
      </div>
    </div>
  );
}
const tuning = [40, 45, 50, 55, 59, 64];
export function Fretboard({
  q,
  selected,
  expected,
  labels,
  onSelect,
  disabled = false,
}: {
  q: Question;
  selected: Point[];
  expected?: Point[];
  labels: boolean;
  onSelect: (p: Point) => void;
  disabled?: boolean;
}) {
  const selectedSet = new Set(selected.map(pointID)),
    expectedSet = new Set(expected?.map(pointID));
  const frets = Array.from(
    { length: q.maxFret - q.minFret + 1 },
    (_, i) => q.minFret + i,
  );
  return (
    <div className="fretboard-wrap">
      <div className="fretboard-scroll">
        <div
          className="fretboard"
          style={{ minWidth: Math.max(280, frets.length * 49 + 38) }}
        >
          <div className="fret-numbers">
            <span />{' '}
            {frets.map((f) => (
              <span key={f}>{f === 0 ? 'Open' : f}</span>
            ))}
          </div>
          {[5, 4, 3, 2, 1, 0].map((s) => (
            <div
              className={`guitar-row ${q.targetString && q.targetString !== s + 1 ? 'inactive' : ''}`}
              key={s}
            >
              <div className="string-name">
                <strong>{['E', 'A', 'D', 'G', 'B', 'e'][s]}</strong>
                <small>{6 - s}</small>
              </div>
              {frets.map((f) => {
                const point = { midi: tuning[s] + f, string: s + 1, fret: f },
                  id = pointID(point),
                  active = selectedSet.has(id),
                  answer = expectedSet.has(id);
                return (
                  <div className={`fret-cell ${f === 0 ? 'open' : ''}`} key={f}>
                    <button
                      type="button"
                      data-string={s + 1}
                      data-fret={f}
                      className={`${active ? 'selected' : ''} ${answer ? 'expected' : ''}`}
                      aria-pressed={active}
                      aria-label={
                        labels
                          ? `${q.labels[pc(point.midi)]}, string ${6 - s}, ${f === 0 ? 'open' : `fret ${f}`}`
                          : `String ${6 - s}, ${f === 0 ? 'open' : `fret ${f}`}`
                      }
                      disabled={
                        disabled ||
                        !!(q.targetString && q.targetString !== s + 1)
                      }
                      onClick={() => onSelect(point)}
                    >
                      {labels || answer
                        ? q.labels[pc(point.midi)]
                        : active
                          ? '●'
                          : ''}
                    </button>
                  </div>
                );
              })}
            </div>
          ))}
          <div className="fret-markers">
            <span />
            {frets.map((f) => (
              <span key={f}>
                {[3, 5, 7, 9].includes(f) ? '●' : f === 12 ? '● ●' : ''}
              </span>
            ))}
          </div>
        </div>
      </div>
      <div className="instrument-caption">
        <span>Standard tuning · right-handed</span>
        <span>Swipe to explore the region</span>
      </div>
    </div>
  );
}
