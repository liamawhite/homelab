import React, { useCallback, useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Keyboard, Fretboard } from './Instruments';
import {
  api,
  defaults,
  keyName,
  keys,
  pc,
  play,
  pointID,
  topics,
  type Feedback,
  type Point,
  type Question,
  type Session,
  type Settings,
  type Stats,
} from './types';
import './style.css';
const EMPTY: Stats = { reviews: [], recent: [] };
const SESSION_KEY = 'melodical.session.v1';
const SETTINGS_KEY = 'melodical.settings.v1';
function local<T>(key: string, fallback: T): T {
  try {
    return JSON.parse(localStorage.getItem(key) || 'null') ?? fallback;
  } catch {
    return fallback;
  }
}
function saveLocal(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* Storage is optional; completed answers still save on the server. */
  }
}
function Icon({ name, size = 20 }: { name: string; size?: number }) {
  const paths: Record<string, React.ReactNode> = {
    note: (
      <>
        <path d="M9 18V5l11-2v13M9 8l11-2" />
        <ellipse cx="6" cy="18" rx="3" ry="2" />
        <ellipse cx="17" cy="16" rx="3" ry="2" />
      </>
    ),
    piano: (
      <>
        <rect x="3" y="5" width="18" height="14" rx="2" />
        <path d="M8 5v14M13 5v14M18 5v14M6 5v7M11 5v7M16 5v7" />
      </>
    ),
    guitar: (
      <>
        <path d="m14 10 6-7 2 2-7 7M14 10c-1-3-5-3-6 0-1 1-5 0-5 5 0 3 3 6 6 6 5 0 4-4 5-5 3-1 3-5 0-6Z" />
        <circle cx="10" cy="14" r="2" />
      </>
    ),
    arrow: <path d="M4 12h16m-6-6 6 6-6 6" />,
    chart: (
      <>
        <path d="M4 4v16h16M8 15l4-5 4 2 4-7" />
      </>
    ),
    book: (
      <>
        <path d="M12 6c-3-2-6-2-9-1v14c3-1 6-1 9 1 3-2 6-2 9-1V5c-3-1-6-1-9 1Zm0 0v14" />
      </>
    ),
    sound: (
      <>
        <path d="m3 9 4 0 5-4v14l-5-4H3Zm13-2c3 3 3 7 0 10M19 4c5 5 5 11 0 16" />
      </>
    ),
    muted: (
      <>
        <path d="m3 9 4 0 5-4v14l-5-4H3ZM17 9l5 6m0-6-5 6" />
      </>
    ),
    sun: (
      <>
        <circle cx="12" cy="12" r="4" />
        <path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2" />
      </>
    ),
    check: <path d="m5 12 4 4L19 6" />,
    close: <path d="m6 6 12 12M6 18 18 6" />,
    clock: (
      <>
        <circle cx="12" cy="12" r="9" />
        <path d="M12 7v5l3 2" />
      </>
    ),
    spark: (
      <>
        <path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5Z" />
      </>
    ),
  };
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name] ?? paths.note}
    </svg>
  );
}
function App() {
  const [settings, setSettings] = useState<Settings>(() => ({
    ...defaults,
    ...local<Partial<Settings>>(SETTINGS_KEY, {}),
  }));
  const [stats, setStats] = useState<Stats>(EMPTY),
    [page, setPage] = useState('practice'),
    [session, setSession] = useState<Session | null>(() =>
      local(SESSION_KEY, null),
    ),
    [active, setActive] = useState(false),
    [summary, setSummary] = useState<Session | null>(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [ready, setReady] = useState(false),
    [advanced, setAdvanced] = useState(false);
  const sessionRef = useRef(session);
  sessionRef.current = session;
  const settingsRef = useRef(settings);
  settingsRef.current = settings;
  const hiddenRef = useRef(document.hidden),
    lastTick = useRef(performance.now()),
    retrySubmit = useRef(false);
  const refresh = useCallback(
    async () => setStats(await api<Stats>('stats')),
    [],
  );
  useEffect(() => {
    Promise.all([api<Settings>('settings'), api<Stats>('stats')])
      .then(([s, p]) => {
        setSettings(s);
        saveLocal(SETTINGS_KEY, s);
        setStats(p);
        setReady(true);
      })
      .catch((e) => {
        setError(e.message);
        setReady(false);
      });
  }, []);
  useEffect(() => {
    saveLocal(SETTINGS_KEY, settings);
    const system = window.matchMedia('(prefers-color-scheme: dark)');
    const apply = () =>
      (document.documentElement.dataset.theme =
        settings.theme === 'system'
          ? system.matches
            ? 'dark'
            : 'light'
          : settings.theme);
    apply();
    system.addEventListener('change', apply);
    return () => system.removeEventListener('change', apply);
  }, [settings]);
  useEffect(() => {
    if (session) saveLocal(SESSION_KEY, session);
    else {
      try {
        localStorage.removeItem(SESSION_KEY);
      } catch {
        /* optional */
      }
    }
  }, [session]);
  useEffect(() => {
    const visibility = () => {
      hiddenRef.current = document.hidden;
      lastTick.current = performance.now();
      if (document.hidden && active)
        setSession((s) =>
          s ? { ...s, interrupted: true, updatedAt: Date.now() } : s,
        );
    };
    const pagehide = () => {
      const s = sessionRef.current;
      if (s)
        saveLocal(SESSION_KEY, {
          ...s,
          interrupted: true,
          updatedAt: Date.now(),
        });
    };
    document.addEventListener('visibilitychange', visibility);
    window.addEventListener('pagehide', pagehide);
    return () => {
      document.removeEventListener('visibilitychange', visibility);
      window.removeEventListener('pagehide', pagehide);
    };
  }, [active]);
  useEffect(() => {
    lastTick.current = performance.now();
    if (!active) return;
    const timer = setInterval(() => {
      const now = performance.now(),
        delta = Math.min(1500, now - lastTick.current);
      lastTick.current = now;
      if (hiddenRef.current || busy) return;
      setSession((s) =>
        s
          ? {
              ...s,
              activeMs: s.activeMs + delta,
              answerMs:
                s.question && !s.feedback ? s.answerMs + delta : s.answerMs,
              updatedAt: Date.now(),
            }
          : s,
      );
    }, 250);
    return () => clearInterval(timer);
  }, [active, busy]);
  const update = async (patch: Partial<Settings>) => {
    const next = { ...settingsRef.current, ...patch };
    setSettings(next);
    try {
      await api('settings', next, 'PUT');
    } catch (e) {
      setError((e as Error).message);
    }
  };
  const nextQuestion = async (base: Session) => {
    const retry = base.retries.find((r) => r.after <= base.answered);
    const q = await api<Question>('questions', {
      instrument: base.settings.instrument,
      topic: base.settings.topic,
      key: base.settings.key,
      difficulty: base.settings.difficulty,
      ...(retry ? { retryId: retry.id } : {}),
    });
    return {
      ...base,
      question: q,
      points: [],
      choice: '',
      feedback: null,
      assisted: false,
      interrupted: false,
      answerMs: 0,
      retries: retry ? base.retries.filter((r) => r !== retry) : base.retries,
      updatedAt: Date.now(),
    };
  };
  const start = async (patch: Partial<Settings> = {}) => {
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      const chosen = { ...settingsRef.current, ...patch };
      await api('settings', chosen, 'PUT');
      setSettings(chosen);
      const base: Session = {
        settings: chosen,
        question: null,
        points: [],
        choice: '',
        feedback: null,
        assisted: false,
        interrupted: false,
        answerMs: 0,
        activeMs: 0,
        answered: 0,
        correct: 0,
        unaided: 0,
        speedTotal: 0,
        speedCount: 0,
        retries: [],
        updatedAt: Date.now(),
      };
      const s = await nextQuestion(base);
      setSession(s);
      setSummary(null);
      setActive(true);
      setPage('practice');
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const finish = () => {
    const s = sessionRef.current;
    setSummary(s);
    setActive(false);
    setSession(null);
    void refresh().catch((e) => setError(e.message));
  };
  const submit = async (points: Point[], choice = '') => {
    const s = sessionRef.current;
    if (!s?.question || s.feedback || retrySubmit.current) return;
    retrySubmit.current = true;
    setBusy(true);
    setError('');
    setSession({ ...s, points, choice });
    try {
      const elapsedMs =
        s.answerMs +
        (!hiddenRef.current
          ? Math.min(1500, performance.now() - lastTick.current)
          : 0);
      const feedback = await api<Feedback>(
        `questions/${s.question.id}/answers`,
        {
          points,
          choice,
          assisted: s.assisted,
          interrupted: s.interrupted,
          elapsedMs: Math.round(elapsedMs),
        },
      );
      const speed = feedback.correct && !s.assisted && !s.interrupted;
      setSession({
        ...s,
        points,
        choice,
        feedback,
        answered: s.answered + 1,
        correct: s.correct + Number(feedback.correct),
        unaided: s.unaided + Number(feedback.correct && !s.assisted),
        speedTotal: s.speedTotal + (speed ? elapsedMs : 0),
        speedCount: s.speedCount + Number(speed),
        retries: !feedback.correct
          ? [...s.retries, { id: s.question.id, after: s.answered + 3 }]
          : s.retries,
        updatedAt: Date.now(),
      });
    } catch (e) {
      setError((e as Error).message);
    } finally {
      retrySubmit.current = false;
      setBusy(false);
    }
  };
  const select = (point: Point) => {
    const s = sessionRef.current;
    if (!s?.question || s.feedback || busy) return;
    if (settingsRef.current.sound) play([point]);
    if (s.question.single) {
      void submit([point]);
      return;
    }
    const selected = s.points.some((p) => pointID(p) === pointID(point));
    setSession({
      ...s,
      points: selected
        ? s.points.filter((p) => pointID(p) !== pointID(point))
        : [...s.points, point],
    });
  };
  const advance = async () => {
    const s = sessionRef.current;
    if (!s || busy) return;
    if (
      (s.settings.mode === 'quick' && s.answered >= s.settings.count) ||
      (s.settings.mode === 'timed' && s.activeMs >= s.settings.minutes * 60000)
    ) {
      finish();
      return;
    }
    setBusy(true);
    setError('');
    try {
      setSession(await nextQuestion(s));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const total = stats.reviews.reduce((a, r) => a + r.attempts, 0),
    correct = stats.reviews.reduce((a, r) => a + r.correct, 0),
    unaided = stats.reviews.reduce((a, r) => a + r.unaided, 0),
    speedTotal = stats.reviews.reduce((a, r) => a + r.speedTotal, 0),
    speedCount = stats.reviews.reduce((a, r) => a + r.speedCount, 0),
    due = stats.reviews.filter((r) => r.dueAt <= Date.now() / 1000).length;
  const q = active ? session?.question : null;
  const practiceInstrument = q?.instrument ?? settings.instrument;
  const errorBox = error ? (
    <div className="error" role="alert">
      <span>{error}</span>
      <button aria-label="Dismiss error" onClick={() => setError('')}>
        <Icon name="close" />
      </button>
      {!ready && (
        <button
          onClick={() => {
            setBusy(true);
            Promise.all([api<Settings>('settings'), api<Stats>('stats')])
              .then(([s, p]) => {
                setSettings(s);
                setStats(p);
                setReady(true);
                setError('');
              })
              .catch((e) => setError(e.message))
              .finally(() => setBusy(false));
          }}
        >
          Retry connection
        </button>
      )}
    </div>
  ) : null;
  return (
    <div className="app-shell">
      <header className="topbar">
        <button
          className="brand"
          onClick={() => {
            if (active) {
              setActive(false);
              setSession((s) => (s ? { ...s, interrupted: true } : s));
            }
            setPage('practice');
          }}
          aria-label="Melodical home"
        >
          <span className="brand-mark">
            <Icon name="note" size={24} />
          </span>
          <span>
            melodical<span className="brand-dot">.</span>
          </span>
        </button>
        <nav aria-label="Main navigation">
          {[
            ['practice', 'Practice', 'note'],
            ['progress', 'Progress', 'chart'],
            ['notes', 'Field notes', 'book'],
          ].map(([id, label, icon]) => (
            <button
              key={id}
              className={page === id ? 'current' : ''}
              onClick={() => {
                if (active) {
                  setActive(false);
                  setSession((s) => (s ? { ...s, interrupted: true } : s));
                }
                setPage(id);
                setSummary(null);
              }}
            >
              <Icon name={icon} size={17} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
        <div className="header-actions">
          <button
            className="icon-button"
            aria-label={settings.sound ? 'Mute sound' : 'Enable sound'}
            title={settings.sound ? 'Mute sound' : 'Enable sound'}
            onClick={() => void update({ sound: !settings.sound })}
          >
            <Icon name={settings.sound ? 'sound' : 'muted'} />
          </button>
          <button
            className="icon-button"
            aria-label={`Theme: ${settings.theme}. Change theme`}
            title={`Theme: ${settings.theme}`}
            onClick={() =>
              void update({
                theme:
                  settings.theme === 'system'
                    ? 'light'
                    : settings.theme === 'light'
                      ? 'dark'
                      : 'system',
              })
            }
          >
            <Icon name="sun" />
          </button>
        </div>
      </header>
      <main>
        {errorBox}
        {page === 'practice' && !active && !summary && (
          <>
            <section className="hero">
              <div>
                <div className="eyebrow">
                  <span className="tiny-line" /> A LITTLE PRACTICE, EVERY DAY
                </div>
                <h1>
                  Make music
                  <br />
                  <em>make sense.</em>
                </h1>
                <p>
                  Less memorizing. More recognizing.
                  <br />
                  Build your musical intuition, one note at a time.
                </p>
              </div>
              <div className="hero-art" aria-hidden="true">
                <span className="art-label">FROM THEORY TO INSTINCT</span>
                <div className="art-orbit orbit-one" />
                <div className="art-orbit orbit-two" />
                <div className="art-note n1">C</div>
                <div className="art-note n2">E</div>
                <div className="art-note n3">G</div>
                <div className="art-center">
                  <Icon name="note" size={38} />
                </div>
                <div className="art-keys">
                  {Array.from({ length: 10 }, (_, i) => (
                    <span
                      key={i}
                      className={
                        [1, 2, 4, 5, 6, 8, 9].includes(i) ? 'has-black' : ''
                      }
                    />
                  ))}
                </div>
                <span className="art-caption">
                  A few notes. A world of possibility.
                </span>
              </div>
            </section>
            {session && (
              <div className="resume-card">
                <div>
                  <strong>Your practice is right here.</strong>
                  <span>
                    {session.answered} answered ·{' '}
                    {Date.now() - session.updatedAt > 30 * 60000
                      ? 'Welcome back. Resume or start fresh.'
                      : 'Pick up where you left off.'}
                  </span>
                </div>
                <button
                  onClick={() => {
                    setActive(true);
                    setSession((s) =>
                      s
                        ? { ...s, interrupted: true, updatedAt: Date.now() }
                        : s,
                    );
                  }}
                  disabled={busy || !ready}
                >
                  Resume <Icon name="arrow" size={16} />
                </button>
                <button
                  className="text-button"
                  onClick={() => void start()}
                  disabled={busy || !ready}
                >
                  Start fresh
                </button>
              </div>
            )}
            <div className="home-grid">
              <section className="practice-card">
                <div className="card-heading">
                  <div>
                    <span className="eyebrow">YOUR NEXT SESSION</span>
                    <h2>A little room to grow.</h2>
                  </div>
                  <span className="small-badge">
                    <Icon name="spark" size={14} /> Adaptive practice
                  </span>
                </div>
                <div className="field-label">Pick your instrument</div>
                <div className="instrument-choice">
                  {[
                    ['piano', 'Piano', 'piano'],
                    ['guitar', 'Guitar', 'guitar'],
                    ['both', 'Both', 'note'],
                  ].map(([id, label, icon]) => (
                    <button
                      key={id}
                      className={settings.instrument === id ? 'chosen' : ''}
                      aria-pressed={settings.instrument === id}
                      onClick={() => void update({ instrument: id })}
                    >
                      <Icon name={icon} size={23} />
                      {label}
                    </button>
                  ))}
                </div>
                <div className="field-label">Find your rhythm</div>
                <div className="mode-choice">
                  {[
                    ['quick', 'Quick round', 'A small, satisfying set'],
                    ['timed', 'Timed practice', 'Make a little time'],
                    ['free', 'Free practice', 'Follow your curiosity'],
                  ].map(([id, label, description]) => (
                    <button
                      key={id}
                      className={settings.mode === id ? 'chosen' : ''}
                      aria-pressed={settings.mode === id}
                      onClick={() => void update({ mode: id })}
                    >
                      <span className="radio-dot" />
                      <span>
                        <strong>{label}</strong>
                        <small>{description}</small>
                      </span>
                    </button>
                  ))}
                </div>
                <div className="session-fields">
                  <label>
                    Focus
                    <select
                      value={settings.topic}
                      onChange={(e) => void update({ topic: e.target.value })}
                    >
                      {Object.entries(topics).map(([id, label]) => (
                        <option key={id} value={id}>
                          {label}
                        </option>
                      ))}
                    </select>
                  </label>
                  {settings.mode === 'quick' ? (
                    <label>
                      Round length
                      <select
                        value={settings.count}
                        onChange={(e) =>
                          void update({ count: Number(e.target.value) })
                        }
                      >
                        {[5, 10, 15, 20].map((n) => (
                          <option key={n} value={n}>
                            {n} questions
                          </option>
                        ))}
                      </select>
                    </label>
                  ) : settings.mode === 'timed' ? (
                    <label>
                      Session length
                      <select
                        value={settings.minutes}
                        onChange={(e) =>
                          void update({ minutes: Number(e.target.value) })
                        }
                      >
                        {[2, 3, 5, 10, 15].map((n) => (
                          <option key={n} value={n}>
                            {n} minutes
                          </option>
                        ))}
                      </select>
                    </label>
                  ) : (
                    <div className="free-description">
                      No finish line.
                      <br />
                      Stop whenever you like.
                    </div>
                  )}
                </div>
                <button
                  className="advanced-toggle"
                  aria-expanded={advanced}
                  onClick={() => setAdvanced(!advanced)}
                >
                  {advanced ? '−' : '+'} Key & difficulty{' '}
                  <span>
                    {settings.key === 'auto'
                      ? 'All keys'
                      : keyName(settings.key)}{' '}
                    ·{' '}
                    {settings.difficulty === 'auto'
                      ? 'Adaptive'
                      : `Level ${settings.difficulty}`}
                  </span>
                </button>
                {advanced && (
                  <div className="session-fields advanced">
                    <label>
                      Musical key
                      <select
                        value={settings.key}
                        onChange={(e) => void update({ key: e.target.value })}
                      >
                        <option value="auto">Choose for me</option>
                        {keys.map((k) => (
                          <option key={k} value={k}>
                            {keyName(k)}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label>
                      Difficulty
                      <select
                        value={settings.difficulty}
                        onChange={(e) =>
                          void update({ difficulty: e.target.value })
                        }
                      >
                        <option value="auto">Adaptive</option>
                        <option value="1">1 · Find your bearings</option>
                        <option value="2">2 · Connect the dots</option>
                        <option value="3">3 · Build fluency</option>
                      </select>
                    </label>
                  </div>
                )}
                <button
                  className="primary start-button"
                  onClick={() => void start()}
                  disabled={busy || !ready}
                >
                  <span>{busy ? 'Getting ready…' : 'Start practicing'}</span>
                  <Icon name="arrow" />
                </button>
                <p className="quiet-note">
                  No pressure. No streak to protect. Just a little progress.
                </p>
              </section>
              <aside className="home-aside">
                <section className="progress-glance">
                  <span className="eyebrow">THE BIGGER PICTURE</span>
                  <h2>Every note adds up.</h2>
                  <div className="glance-number">
                    {total}
                    <span>questions explored</span>
                  </div>
                  <div className="glance-row">
                    <span>Unaided accuracy</span>
                    <strong>
                      {total ? `${Math.round((unaided / total) * 100)}%` : '—'}
                    </strong>
                  </div>
                  <div className="glance-row">
                    <span>Recognition time</span>
                    <strong>
                      {speedCount
                        ? `${(speedTotal / speedCount / 1000).toFixed(1)}s`
                        : '—'}
                    </strong>
                  </div>
                  <div className="glance-row">
                    <span>Areas due for review</span>
                    <strong>{due}</strong>
                  </div>
                  <button
                    className="text-link"
                    onClick={() => setPage('progress')}
                  >
                    See your progress <Icon name="arrow" size={17} />
                  </button>
                </section>
                <section className="insight-card">
                  <span className="eyebrow">A NOTE TO TAKE WITH YOU</span>
                  <Icon name="book" size={26} />
                  <h3>
                    A key is a home,
                    <br />
                    not a fence.
                  </h3>
                  <p>
                    A key gives you a family of notes to recognize. Songs can
                    borrow from outside that family, too.
                  </p>
                  <button
                    className="text-link"
                    onClick={() => setPage('notes')}
                  >
                    Explore field notes <Icon name="arrow" size={17} />
                  </button>
                </section>
              </aside>
            </div>
            <section className="focus-section">
              <div className="section-title">
                <h2>One thing at a time.</h2>
                <span>Or choose a focus and go a little deeper.</span>
              </div>
              <div className="focus-cards">
                {[
                  [
                    'notes',
                    'guitar',
                    'Know your way around.',
                    'Find notes on the guitar neck.',
                    'guitar',
                  ],
                  [
                    'keys',
                    'both',
                    'Find the family.',
                    'Connect notes to their musical key.',
                    'note',
                  ],
                  [
                    'inversions',
                    'piano',
                    'See the shape.',
                    'Recognize chords in new arrangements.',
                    'piano',
                  ],
                ].map(([topic, instrument, title, description, icon]) => (
                  <button
                    key={topic}
                    className="focus-card"
                    disabled={!ready || busy}
                    onClick={() => void start({ topic, instrument })}
                  >
                    <Icon name={icon} size={26} />
                    <h3>{title}</h3>
                    <p>{description}</p>
                    <span>
                      Practice {topics[topic].toLowerCase()}{' '}
                      <Icon name="arrow" size={16} />
                    </span>
                  </button>
                ))}
              </div>
            </section>
          </>
        )}
        {page === 'practice' && active && session && q && (
          <section className="exercise-page">
            <div className="session-top">
              <button
                className="text-button"
                onClick={() => {
                  setActive(false);
                  setSession((s) => (s ? { ...s, interrupted: true } : s));
                }}
              >
                ← Pause practice
              </button>
              <span>
                {session.settings.mode === 'quick'
                  ? `${session.answered} / ${session.settings.count}`
                  : session.settings.mode === 'timed'
                    ? `${Math.floor(session.activeMs / 60000)}:${String(Math.floor(session.activeMs / 1000) % 60).padStart(2, '0')} / ${session.settings.minutes}:00`
                    : `${session.answered} explored`}
              </span>
              <button className="text-button" onClick={finish} disabled={busy}>
                Finish
              </button>
            </div>
            {session.settings.mode === 'quick' && (
              <div className="round-progress">
                <span
                  style={{
                    width: `${Math.min(100, (session.answered / session.settings.count) * 100)}%`,
                  }}
                />
              </div>
            )}
            <div className="question-heading">
              <span className="exercise-tag">
                <Icon name={practiceInstrument} size={17} />
                {practiceInstrument} <span>·</span> {topics[q.topic]}{' '}
                <span>·</span> Level {q.level}
              </span>
              <h1>{q.prompt}</h1>
              <p>{q.detail}</p>
            </div>
            <div className="instrument-panel">
              <div className="panel-tools">
                <span>
                  {keyName(q.key)} {q.topic === 'keys' ? 'major' : ''}
                </span>
                <div>
                  {(q.display || session.feedback) && (
                    <button
                      className="tool-button"
                      onClick={() =>
                        play(
                          session.feedback?.expected ?? q.display ?? [],
                          q.kind === 'scale',
                        )
                      }
                    >
                      Listen <Icon name="sound" size={16} />
                    </button>
                  )}
                  <button
                    className="tool-button"
                    disabled={!!session.feedback || session.assisted || busy}
                    onClick={() =>
                      setSession((s) => (s ? { ...s, assisted: true } : s))
                    }
                  >
                    {session.assisted
                      ? 'Note labels shown'
                      : 'Show note labels'}{' '}
                    <span className="hint-tag">Hint</span>
                  </button>
                </div>
              </div>
              {q.instrument === 'piano' ? (
                <Keyboard
                  key={q.id}
                  q={q}
                  selected={session.points}
                  expected={session.feedback?.expected}
                  labels={
                    q.level === 1 || session.assisted || !!session.feedback
                  }
                  onSelect={select}
                  disabled={busy || !!session.feedback || q.kind === 'identify'}
                />
              ) : (
                <Fretboard
                  key={q.id}
                  q={q}
                  selected={session.points}
                  expected={session.feedback?.expected}
                  labels={session.assisted || !!session.feedback}
                  onSelect={select}
                  disabled={busy || !!session.feedback}
                />
              )}
              <div className="selection-readout">
                {q.kind === 'identify' ? (
                  <span>Look at the highlighted notes, then choose below.</span>
                ) : session.points.length ? (
                  <>
                    <span>
                      {session.points.length}{' '}
                      {session.points.length === 1 ? 'note' : 'notes'} selected
                    </span>
                    {(session.assisted ||
                      q.level === 1 ||
                      session.feedback) && (
                      <strong>
                        {session.points
                          .map((p) => q.labels[pc(p.midi)])
                          .join(' · ')}
                      </strong>
                    )}
                    <button
                      className="text-button"
                      disabled={!!session.feedback || busy}
                      onClick={() =>
                        setSession((s) => (s ? { ...s, points: [] } : s))
                      }
                    >
                      Clear
                    </button>
                  </>
                ) : (
                  <span>
                    {q.single
                      ? 'Tap a note to answer.'
                      : 'Select your notes, then check your answer.'}
                  </span>
                )}
              </div>
            </div>
            {q.choices && !session.feedback && (
              <div className="answer-choices">
                {q.choices.map((choice) => (
                  <button
                    key={choice}
                    disabled={busy}
                    onClick={() => void submit([], choice)}
                  >
                    {choice}
                  </button>
                ))}
              </div>
            )}
            {!session.feedback && (
              <div className="exercise-bottom">
                <span>
                  {session.assisted
                    ? 'Assisted attempt · learning still counts.'
                    : 'Take your time. There’s no countdown.'}
                </span>
                {!q.single && q.kind !== 'identify' && (
                  <button
                    className="primary"
                    disabled={busy || !session.points.length}
                    onClick={() => void submit(session.points)}
                  >
                    {busy ? 'Saving…' : 'Check answer'}{' '}
                    <Icon name="arrow" size={17} />
                  </button>
                )}
                {q.single && session.points.length > 0 && error && (
                  <button
                    className="primary"
                    disabled={busy}
                    onClick={() => void submit(session.points)}
                  >
                    Retry saving answer
                  </button>
                )}
              </div>
            )}
            {session.feedback && (
              <div
                className={`feedback ${session.feedback.correct ? 'correct' : 'incorrect'}`}
                role="status"
              >
                <div className="feedback-icon">
                  <Icon
                    name={session.feedback.correct ? 'check' : 'book'}
                    size={24}
                  />
                </div>
                <div>
                  <h2>
                    {session.feedback.correct
                      ? session.assisted
                        ? 'You found it.'
                        : 'That’s it. Nicely connected.'
                      : 'A good moment to connect the dots.'}
                  </h2>
                  {session.feedback.choice && (
                    <strong>{session.feedback.choice}</strong>
                  )}
                  <p>{session.feedback.explanation}</p>
                  {!session.feedback.correct && (
                    <small>
                      We’ll revisit this after a couple of questions.
                    </small>
                  )}
                </div>
                <button
                  className="primary"
                  onClick={() => void advance()}
                  disabled={busy}
                >
                  {busy
                    ? 'Loading…'
                    : (session.settings.mode === 'quick' &&
                          session.answered >= session.settings.count) ||
                        (session.settings.mode === 'timed' &&
                          session.activeMs >= session.settings.minutes * 60000)
                      ? 'Finish round'
                      : 'Next question'}
                  <Icon name="arrow" size={17} />
                </button>
              </div>
            )}
          </section>
        )}
        {page === 'practice' && summary && (
          <section className="summary">
            <span className="summary-icon">
              <Icon name="spark" size={36} />
            </span>
            <div className="eyebrow">A LITTLE FURTHER THAN BEFORE</div>
            <h1>Practice, made progress.</h1>
            <p>
              {summary.answered
                ? 'You made a little space for music today.'
                : 'Your next little step is waiting when you’re ready.'}
            </p>
            <div className="summary-stats">
              <div>
                <strong>{summary.answered}</strong>
                <span>questions explored</span>
              </div>
              <div>
                <strong>
                  {summary.answered
                    ? `${Math.round((summary.correct / summary.answered) * 100)}%`
                    : '—'}
                </strong>
                <span>correct answers</span>
              </div>
              <div>
                <strong>
                  {summary.speedCount
                    ? `${(summary.speedTotal / summary.speedCount / 1000).toFixed(1)}s`
                    : '—'}
                </strong>
                <span>unaided recognition</span>
              </div>
            </div>
            <p className="quiet-note">
              {summary.unaided} unaided correct ·{' '}
              {summary.correct - summary.unaided} assisted correct
            </p>
            <div className="summary-actions">
              <button
                className="primary"
                onClick={() => void start(summary.settings)}
                disabled={busy || !ready}
              >
                Another little round <Icon name="arrow" />
              </button>
              <button className="secondary" onClick={() => setSummary(null)}>
                Back to practice
              </button>
            </div>
          </section>
        )}
        {page === 'progress' && (
          <section className="progress-page">
            <div className="eyebrow">YOUR MUSICAL INTUITION, GROWING</div>
            <h1>See what’s clicking.</h1>
            <p className="page-description">
              Accuracy first. Fluency follows. Your piano and guitar skills grow
              at their own pace.
            </p>
            <div className="metric-grid">
              <div>
                <span>Questions explored</span>
                <strong>{total}</strong>
              </div>
              <div>
                <span>Overall accuracy</span>
                <strong>
                  {total ? `${Math.round((correct / total) * 100)}%` : '—'}
                </strong>
              </div>
              <div>
                <span>Unaided accuracy</span>
                <strong>
                  {total ? `${Math.round((unaided / total) * 100)}%` : '—'}
                </strong>
              </div>
              <div>
                <span>Recognition time</span>
                <strong>
                  {speedCount
                    ? `${(speedTotal / speedCount / 1000).toFixed(1)}s`
                    : '—'}
                </strong>
              </div>
            </div>
            <div className="section-title">
              <h2>Your practice map.</h2>
              <button
                className="text-button"
                onClick={() => void refresh().catch((e) => setError(e.message))}
              >
                Refresh
              </button>
            </div>
            {!total ? (
              <div className="empty-state">
                <Icon name="note" size={35} />
                <h3>Every musician starts somewhere.</h3>
                <p>
                  Finish a few questions to see your strengths and the things to
                  revisit.
                </p>
                <button
                  className="primary"
                  onClick={() => void start()}
                  disabled={!ready || busy}
                >
                  Start practicing <Icon name="arrow" />
                </button>
              </div>
            ) : (
              <div className="skill-grid">
                {['piano', 'guitar'].flatMap((instrument) =>
                  Object.entries(topics)
                    .filter(([topic]) => topic !== 'mixed')
                    .map(([topic, label]) => {
                      const reviews = stats.reviews.filter(
                        (r) => r.instrument === instrument && r.topic === topic,
                      );
                      if (!reviews.length) return null;
                      const attempts = reviews.reduce(
                          (a, r) => a + r.attempts,
                          0,
                        ),
                        good = reviews.reduce((a, r) => a + r.unaided, 0),
                        time = reviews.reduce((a, r) => a + r.speedTotal, 0),
                        count = reviews.reduce((a, r) => a + r.speedCount, 0),
                        percentage = Math.round((good / attempts) * 100);
                      return (
                        <article
                          className="skill-card"
                          key={`${instrument}:${topic}`}
                        >
                          <div className="skill-title">
                            <Icon name={instrument} />
                            <span>{instrument}</span>
                            <strong>{label}</strong>
                          </div>
                          <div className="skill-score">
                            <strong>{percentage}%</strong>
                            <span>unaided accuracy</span>
                          </div>
                          <div className="skill-bar">
                            <span style={{ width: `${percentage}%` }} />
                          </div>
                          <p>
                            {attempts} attempts ·{' '}
                            {count
                              ? `${(time / count / 1000).toFixed(1)}s recognition`
                              : 'Recognition time still forming'}
                          </p>
                          <div className="key-chips">
                            {reviews.map((r) => (
                              <span
                                key={r.skill}
                                title={`${r.attempts} attempts`}
                              >
                                {keyName(r.key)}{' '}
                                {r.dueAt <= Date.now() / 1000 ? '· review' : ''}
                              </span>
                            ))}
                          </div>
                          <button
                            className="text-link"
                            disabled={busy}
                            onClick={() => void start({ instrument, topic })}
                          >
                            Practice this <Icon name="arrow" size={16} />
                          </button>
                        </article>
                      );
                    }),
                )}
              </div>
            )}
            <p className="quiet-note">
              Recognition time includes correct, unaided, uninterrupted answers
              only. Assisted answers help you learn, but don’t establish
              proficiency.
            </p>
            {stats.recent.length > 0 && (
              <section className="recent">
                <h2>Recent discoveries.</h2>
                {stats.recent.map((r, i) => (
                  <div key={i}>
                    <span
                      className={r.correct ? 'recent-good' : 'recent-review'}
                    >
                      <Icon name={r.correct ? 'check' : 'book'} size={16} />
                    </span>
                    <strong>{topics[r.topic]}</strong>
                    <span>
                      {r.instrument} · {keyName(r.key)}
                    </span>
                    <small>
                      {r.assisted
                        ? 'Assisted'
                        : r.correct
                          ? 'Unaided correct'
                          : 'To revisit'}
                    </small>
                  </div>
                ))}
              </section>
            )}
          </section>
        )}
        {page === 'notes' && (
          <section className="notes-page">
            <div className="eyebrow">A FEW IDEAS TO KEEP CLOSE</div>
            <h1>Little field notes.</h1>
            <p className="page-description">
              The why behind the notes. Take an idea to your instrument, or
              explore it right here.
            </p>
            <div className="field-notes">
              {[
                [
                  'keys',
                  'A key is a family of notes.',
                  'A major scale follows the pattern whole, whole, half, whole, whole, whole, half steps. In C major that’s C, D, E, F, G, A, B. In D major it’s D, E, F♯, G, A, B, C♯. A key has a tonic: the note that feels like home. Songs can borrow notes from outside their key.',
                  'At your instrument: play a major scale slowly. Return to the tonic and listen to the sense of arrival.',
                ],
                [
                  'intervals',
                  'Distance gives notes a relationship.',
                  'An interval measures the distance between two notes. One semitone is one piano key (including black keys) or one guitar fret. A major third is four semitones; a minor third is three; a perfect fifth is seven. The same relationship can move to any starting note.',
                  'At your instrument: play a note, then a note seven semitones above it. Move the pair somewhere else.',
                ],
                [
                  'chords',
                  'Three notes can tell a story.',
                  'A major triad contains a root, major third, and perfect fifth (1–3–5). A minor triad lowers the third by one semitone (1–♭3–5). C major is C–E–G; C minor is C–E♭–G. Spacing and doubling change the voicing, without changing the chord’s identity.',
                  'At your instrument: play a major triad. Lower only the third and hear the change in character.',
                ],
                [
                  'inversions',
                  'Same chord. A different foundation.',
                  'Root position has the root in the bass. First inversion puts the third lowest; second inversion puts the fifth lowest. C–E–G, E–G–C, and G–C–E are all C major. An inversion describes the bass note; a voicing also describes spacing and doubling.',
                  'At your piano: move the lowest note of C–E–G up an octave. Repeat, listening to the same chord in a new arrangement.',
                ],
                [
                  'notes',
                  'The neck has landmarks.',
                  'Standard guitar tuning, low to high, is E–A–D–G–B–E. Every fret adds one semitone. The twelfth fret repeats the open string an octave higher. Conventional string numbers run the other way: high E is string 1, low E is string 6.',
                  'At your guitar: find every E in frets 0–12. Then choose one string and name each note slowly.',
                ],
              ].map(([topic, title, body, prompt], i) => (
                <article key={topic}>
                  <span className="note-number">0{i + 1}</span>
                  <div>
                    <span className="eyebrow">{topics[topic]}</span>
                    <h2>{title}</h2>
                    <p>{body}</p>
                    <blockquote>{prompt}</blockquote>
                    <button
                      className="text-link"
                      disabled={!ready || busy}
                      onClick={() =>
                        void start({
                          topic,
                          instrument:
                            topic === 'notes'
                              ? 'guitar'
                              : topic === 'inversions'
                                ? 'piano'
                                : settings.instrument,
                        })
                      }
                    >
                      Put it into practice <Icon name="arrow" size={16} />
                    </button>
                  </div>
                </article>
              ))}
            </div>
            <article className="roman-note">
              <h2>From names to roles.</h2>
              <p>
                Scale degrees describe a note’s role in a key: D is degree 1 in
                D major, F♯ is degree 3, and A is degree 5. Roman numerals
                describe chords built on those degrees. In a major key, the
                diatonic triads are I, ii, iii, IV, V, vi, vii°. Capitals
                indicate major, lowercase minor, and ° diminished. In C major,
                C–F–G is I–IV–V; in D major, D–G–A has the same roles.
              </p>
            </article>
          </section>
        )}
        {page === 'practice' && !active && !summary && (
          <div className="mobile-start">
            <button
              className="primary"
              disabled={busy || !ready}
              onClick={() => void start()}
              aria-label="Start a session with current settings"
            >
              <span>
                {busy ? 'Getting ready…' : 'Start practicing'}
                <small>
                  {settings.instrument === 'both'
                    ? 'Piano & guitar'
                    : settings.instrument === 'piano'
                      ? 'Piano'
                      : 'Guitar'}{' '}
                  ·{' '}
                  {settings.mode === 'quick'
                    ? `${settings.count} questions`
                    : settings.mode === 'timed'
                      ? `${settings.minutes} minutes`
                      : 'Your own pace'}
                </small>
              </span>
              <Icon name="arrow" />
            </button>
          </div>
        )}
      </main>
      <footer>
        <span>
          <Icon name="note" size={15} /> A little practice goes a long way.
        </span>
        <span>Piano & guitar · Built for your own pace</span>
      </footer>
    </div>
  );
}
createRoot(document.getElementById('root')!).render(<App />);
