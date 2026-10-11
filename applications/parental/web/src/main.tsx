import React, {
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from 'react';
import { createRoot } from 'react-dom/client';
import {
  QueryClient,
  QueryClientProvider,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';
import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import {
  Baby,
  Plus,
  ArrowLeft,
  ArrowRight,
  Pencil,
  Trash2,
  X,
  CalendarDays,
  Check,
  AlertCircle,
  Sun,
  Moon,
} from 'lucide-react';
import {
  PlannerService,
  type Person,
  type Assignment,
  type Vacation,
  type GetPlanResponse,
} from './gen/parental/v1/planner_pb';
import '@fontsource-variable/geist';
import './style.css';

const api = createClient(
  PlannerService,
  createConnectTransport({ baseUrl: '/' }),
);
const queryClient = new QueryClient();
const colors = [
  '#377c65',
  '#527ec2',
  '#b77645',
  '#9a68ad',
  '#ba5d75',
  '#708840',
];
const parse = (date: string) => new Date(`${date}T12:00:00Z`);
const fmt = (
  date: string,
  options: Intl.DateTimeFormatOptions = {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  },
) => parse(date).toLocaleDateString('en-US', { ...options, timeZone: 'UTC' });
const dateKey = (d: Date) => d.toISOString().slice(0, 10);
const monthKey = (date: string) => date.slice(0, 7);
const countLabel = (n: number) => `${n} ${n === 1 ? 'day' : 'days'}`;

type Editor =
  | { kind: 'plan' }
  | { kind: 'vacation'; vacation?: Vacation }
  | { kind: 'person'; person?: Person }
  | {
      kind: 'assignment';
      assignment?: Assignment;
      start?: string;
      end?: string;
    };

function Modal({
  title,
  children,
  onClose,
  busy,
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
  busy: boolean;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  return (
    <dialog
      ref={ref}
      onCancel={(e) => {
        e.preventDefault();
        if (!busy) onClose();
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <div className="modal-head">
        <h2>{title}</h2>
        <button
          type="button"
          className="icon"
          aria-label="Close dialog"
          disabled={busy}
          onClick={onClose}
        >
          <X size={20} />
        </button>
      </div>
      {children}
    </dialog>
  );
}

function App() {
  const cache = useQueryClient();
  const query = useQuery({
    queryKey: ['plan'],
    queryFn: () => api.getPlan({}),
    refetchInterval: 30000,
  });
  const data = query.data;
  const [editor, setEditor] = useState<Editor | null>(null);
  const [month, setMonth] = useState(() => {
    const saved = localStorage.getItem('parental-month') || '';
    return /^\d{4}-(0[1-9]|1[0-2])$/.test(saved) ? saved : '';
  });
  const [error, setError] = useState('');
  const [theme, setTheme] = useState(
    () =>
      localStorage.getItem('parental-theme') ||
      (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'),
  );
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem('parental-theme', theme);
  }, [theme]);
  useEffect(() => {
    if (
      data?.plan &&
      (!month ||
        month < data.days[0].date.slice(0, 7) ||
        month > data.days.at(-1)!.date.slice(0, 7))
    )
      setMonth(monthKey(data.days[0].date));
  }, [data, month]);
  useEffect(() => {
    if (month) localStorage.setItem('parental-month', month);
  }, [month]);
  const mutation = useMutation({
    mutationFn: async (fn: () => Promise<unknown>) => fn(),
    onSuccess: async () => {
      setEditor(null);
      setError('');
      await cache.invalidateQueries({ queryKey: ['plan'] });
    },
    onError: (e: Error) => setError(e.message),
  });
  const open = (value: Editor) => {
    setError('');
    setEditor(value);
  };
  const submit = (fn: () => Promise<unknown>) => {
    setError('');
    mutation.mutate(fn);
  };
  const busy = mutation.isPending;
  const people = new Map(data?.people.map((p) => [p.id, p]));
  const used = new Map(data?.budgets.map((b) => [b.personId, b.usedDays]));
  const overdrawn =
    data?.people.filter(
      (p) => p.budgetDays !== undefined && (used.get(p.id) ?? 0) > p.budgetDays,
    ) ?? [];

  return (
    <>
      <header className="topbar">
        <a className="brand" href="/" aria-label="Parental home">
          <span className="brand-icon">
            <Baby size={23} />
          </span>
          parental<span className="brand-caption">A year of care</span>
        </a>
        <button
          className="icon"
          aria-label={theme === 'dark' ? 'Use light theme' : 'Use dark theme'}
          onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
        >
          {theme === 'dark' ? <Sun size={19} /> : <Moon size={19} />}
        </button>
      </header>
      <main>
        {query.isPending ? (
          <p role="status" className="empty">
            Loading your plan…
          </p>
        ) : query.isError && !data ? (
          <div className="empty" role="alert">
            <h2>Couldn’t load the planner</h2>
            <p>{query.error.message}</p>
            <button onClick={() => query.refetch()}>Try again</button>
          </div>
        ) : (
          data && (
            <>
              {query.isError && (
                <div className="warning" role="alert">
                  Couldn’t refresh. Displaying the last loaded plan.{' '}
                  <button onClick={() => query.refetch()}>Retry</button>
                </div>
              )}
              {error && !editor && (
                <div className="warning" role="alert">
                  {error}
                </div>
              )}
              {!data.plan ? (
                <section className="welcome">
                  <span className="eyebrow">THE FIRST YEAR, TOGETHER</span>
                  <h1>
                    A little planning.
                    <br />A lot of care.
                  </h1>
                  <p>
                    Bring everyone’s leave into one shared plan.
                    <br />
                    See who’s there, what’s covered, and where you need a hand.
                  </p>
                  <button
                    className="primary"
                    onClick={() => open({ kind: 'plan' })}
                  >
                    <CalendarDays size={18} /> Set expected birth date
                  </button>
                  <small>
                    You can change it anytime. Your assignments will keep their
                    dates.
                  </small>
                </section>
              ) : (
                <>
                  <section className="page-heading">
                    <div>
                      <span className="eyebrow">THE FIRST YEAR, TOGETHER</span>
                      <h1>Your care plan</h1>
                      <button
                        className="date-edit"
                        onClick={() => open({ kind: 'plan' })}
                      >
                        {data.plan.actual ? 'Born' : 'Expected'}{' '}
                        {fmt(data.plan.birthDate)} <Pencil size={13} />
                      </button>
                      <span className="date-end">
                        Plan: {fmt(data.days[0].date)} –{' '}
                        {fmt(data.days.at(-1)!.date)}
                      </span>
                    </div>
                    <button
                      className="primary"
                      disabled={!data.people.length}
                      onClick={() => open({ kind: 'assignment' })}
                    >
                      <Plus size={18} /> Add coverage
                    </button>
                  </section>
                  <section className="stats" aria-label="Coverage summary">
                    <div>
                      <span>Weekdays covered</span>
                      <strong>
                        {data.coveredDays}
                        <small> / {data.requiredDays}</small>
                      </strong>
                      <div className="progress">
                        <i
                          style={{
                            width: `${data.requiredDays ? (data.coveredDays / data.requiredDays) * 100 : 0}%`,
                          }}
                        />
                      </div>
                    </div>
                    <div>
                      <span>Still to cover</span>
                      <strong
                        className={
                          data.requiredDays > data.coveredDays
                            ? 'gap-number'
                            : ''
                        }
                      >
                        {data.requiredDays - data.coveredDays}
                        <small> days</small>
                      </strong>
                      <p>
                        {data.requiredDays === data.coveredDays
                          ? 'Every required day has a helping hand.'
                          : 'A little space for a helping hand.'}
                      </p>
                    </div>
                    <div>
                      <span>Shared coverage</span>
                      <strong>
                        {data.multipleDays}
                        <small> days</small>
                      </strong>
                      <p>Two or more people, together.</p>
                    </div>
                  </section>
                  {overdrawn.length > 0 && (
                    <div className="warning" role="status">
                      <AlertCircle size={18} />
                      <span>
                        {overdrawn
                          .map(
                            (p) =>
                              `${p.name}: ${countLabel((used.get(p.id) ?? 0) - p.budgetDays!)} over budget`,
                          )
                          .join(' · ')}
                        . Your assignments are saved.
                      </span>
                    </div>
                  )}
                  <div className="planner-grid">
                    <section className="panel calendar-panel">
                      <div className="section-heading">
                        <div>
                          <span className="eyebrow">COVERAGE CALENDAR</span>
                          <h2>
                            {month
                              ? fmt(`${month}-01`, {
                                  month: 'long',
                                  year: 'numeric',
                                })
                              : ''}
                          </h2>
                        </div>
                        <div className="month-nav">
                          <button
                            className="icon"
                            aria-label="Previous month"
                            disabled={
                              !month || month <= monthKey(data.days[0].date)
                            }
                            onClick={() => {
                              const d = parse(`${month}-01`);
                              d.setUTCMonth(d.getUTCMonth() - 1);
                              setMonth(monthKey(dateKey(d)));
                            }}
                          >
                            <ArrowLeft size={18} />
                          </button>
                          <button
                            className="icon"
                            aria-label="Next month"
                            disabled={
                              !month ||
                              month >= monthKey(data.days.at(-1)!.date)
                            }
                            onClick={() => {
                              const d = parse(`${month}-01`);
                              d.setUTCMonth(d.getUTCMonth() + 1);
                              setMonth(monthKey(dateKey(d)));
                            }}
                          >
                            <ArrowRight size={18} />
                          </button>
                        </div>
                      </div>
                      {month && (
                        <Calendar
                          data={data}
                          month={month}
                          onDay={(date) =>
                            open({ kind: 'assignment', start: date, end: date })
                          }
                        />
                      )}
                      <div className="legend">
                        <span>
                          <i className="legend-gap" /> Needs coverage
                        </span>
                        <span>
                          <i className="legend-holiday" /> Public holiday
                        </span>
                        {data.people.map((p) => (
                          <span key={p.id}>
                            <i style={{ background: p.color }} />
                            {p.name}
                          </span>
                        ))}
                      </div>
                      <p className="footnote">
                        Monday–Friday · Public holidays don’t use leave or
                        require coverage. The three weeks before birth are
                        optional. Select a weekday to add coverage.
                      </p>
                    </section>
                    <aside className="panel people-panel">
                      <div className="section-heading">
                        <div>
                          <span className="eyebrow">YOUR VILLAGE</span>
                          <h2>People & leave</h2>
                        </div>
                        <button
                          className="icon"
                          aria-label="Add person"
                          onClick={() => open({ kind: 'person' })}
                        >
                          <Plus size={19} />
                        </button>
                      </div>
                      {!data.people.length ? (
                        <div className="empty small">
                          <Baby size={32} />
                          <h3>Who’s lending a hand?</h3>
                          <p>
                            Add parents, family, or anyone helping with care.
                          </p>
                          <button onClick={() => open({ kind: 'person' })}>
                            <Plus size={16} /> Add first person
                          </button>
                        </div>
                      ) : (
                        data.people.map((p) => {
                          const n = used.get(p.id) ?? 0;
                          const remaining =
                            p.budgetDays === undefined
                              ? undefined
                              : p.budgetDays - n;
                          return (
                            <div className="person" key={p.id}>
                              <div className="person-top">
                                <span
                                  className="avatar"
                                  style={{ background: p.color }}
                                >
                                  {p.name.slice(0, 1).toUpperCase()}
                                </span>
                                <div>
                                  <strong>{p.name}</strong>
                                  <small>
                                    {p.budgetDays === undefined
                                      ? 'Unlimited availability'
                                      : `${countLabel(p.budgetDays)} of leave`}
                                  </small>
                                </div>
                                <button
                                  className="icon"
                                  aria-label={`Edit ${p.name}`}
                                  onClick={() =>
                                    open({ kind: 'person', person: p })
                                  }
                                >
                                  <Pencil size={15} />
                                </button>
                              </div>
                              {p.budgetDays !== undefined && (
                                <div
                                  className={`progress ${remaining! < 0 ? 'over' : ''}`}
                                >
                                  <i
                                    style={{
                                      width: `${p.budgetDays ? Math.min((n / p.budgetDays) * 100, 100) : n ? 100 : 0}%`,
                                      background:
                                        remaining! < 0 ? undefined : p.color,
                                    }}
                                  />
                                </div>
                              )}
                              <div className="budget-line">
                                <span>{countLabel(n)} assigned</span>
                                <strong
                                  className={
                                    remaining !== undefined && remaining < 0
                                      ? 'danger'
                                      : ''
                                  }
                                >
                                  {remaining === undefined
                                    ? 'No limit'
                                    : remaining < 0
                                      ? `${countLabel(-remaining)} over`
                                      : `${countLabel(remaining)} left`}
                                </strong>
                              </div>
                            </div>
                          );
                        })
                      )}
                      <p className="footnote">
                        Leave counts unique weekdays, excluding public holidays.
                        Overlapping assignments for one person count once.
                      </p>
                    </aside>
                  </div>
                  <section className="panel gap-panel">
                    <div className="section-heading">
                      <div>
                        <span className="eyebrow">THE OPEN SPACES</span>
                        <h2>
                          {data.gaps.length
                            ? 'Days still needing care'
                            : 'All covered'}{' '}
                          {data.gaps.length > 0 && (
                            <span className="badge">
                              {data.gaps.length}{' '}
                              {data.gaps.length === 1 ? 'window' : 'windows'}
                            </span>
                          )}
                        </h2>
                      </div>
                      {!data.gaps.length && <Check size={24} />}
                    </div>
                    {data.gaps.length ? (
                      <div className="gap-list">
                        {data.gaps.map((g) => (
                          <button
                            key={g.startDate}
                            className="gap-card"
                            disabled={!data.people.length}
                            onClick={() =>
                              open({
                                kind: 'assignment',
                                start: g.startDate,
                                end: g.endDate,
                              })
                            }
                          >
                            <span>
                              {fmt(g.startDate)}
                              {g.endDate !== g.startDate
                                ? ` – ${fmt(g.endDate)}`
                                : ''}
                              <small>
                                {countLabel(g.days)} needing coverage
                              </small>
                            </span>
                            <Plus size={17} />
                          </button>
                        ))}
                      </div>
                    ) : (
                      <p>Every required weekday has someone assigned.</p>
                    )}
                  </section>
                  <section className="panel">
                    <div className="section-heading">
                      <div>
                        <span className="eyebrow">WHO’S THERE, WHEN</span>
                        <h2>
                          Assignments{' '}
                          <span className="badge">
                            {data.assignments.length}
                          </span>
                        </h2>
                      </div>
                      <button
                        disabled={!data.people.length}
                        onClick={() => open({ kind: 'assignment' })}
                      >
                        <Plus size={16} /> Add coverage
                      </button>
                    </div>
                    {!data.assignments.length ? (
                      <p className="empty">
                        No assignments yet. Add a person, then choose their
                        dates.
                      </p>
                    ) : (
                      <div className="assignments">
                        {data.assignments.map((a) => {
                          const p = people.get(a.personId);
                          const outside =
                            a.startDate < data.days[0].date ||
                            a.endDate > data.firstBirthday;
                          return (
                            <div className="assignment" key={a.id}>
                              <span className="person-tag">
                                <i style={{ background: p?.color }} />
                                {p?.name}
                              </span>
                              <div className="assignment-details">
                                <strong>
                                  {fmt(a.startDate)} – {fmt(a.endDate)}
                                </strong>
                                <small>
                                  Monday–Friday
                                  {outside && (
                                    <span className="danger">
                                      {' '}
                                      · Extends outside the planning window
                                    </span>
                                  )}
                                </small>
                                {a.notes && <p>{a.notes}</p>}
                              </div>
                              <button
                                className="icon"
                                aria-label={`Edit assignment for ${p?.name} starting ${a.startDate}`}
                                onClick={() =>
                                  open({ kind: 'assignment', assignment: a })
                                }
                              >
                                <Pencil size={16} />
                              </button>
                              <button
                                className="icon"
                                disabled={busy}
                                aria-label={`Delete assignment for ${p?.name} starting ${a.startDate}`}
                                onClick={() => {
                                  if (
                                    confirm(
                                      `Delete ${p?.name}’s coverage from ${fmt(a.startDate)} to ${fmt(a.endDate)}?`,
                                    )
                                  )
                                    submit(() =>
                                      api.deleteAssignment({ id: a.id }),
                                    );
                                }}
                              >
                                <Trash2 size={16} />
                              </button>
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </section>
                  <section className="panel">
                    <div className="section-heading">
                      <h2>
                        Vacations{' '}
                        <span className="badge">{data.vacations.length}</span>
                      </h2>
                      <button onClick={() => open({ kind: 'vacation' })}>
                        <Plus size={16} /> Add vacation
                      </button>
                    </div>
                    <p className="muted">
                      Mark your travel dates here. Assign people separately to
                      account for coverage and leave days.
                    </p>
                    <div className="assignments">
                      {data.vacations.map((v) => (
                        <div className="assignment" key={v.id}>
                          <Sun size={18} />
                          <div className="assignment-details">
                            <strong>{v.name}</strong>
                            <p>
                              {fmt(v.startDate)} – {fmt(v.endDate)}
                            </p>
                            {(v.startDate < data.days[0].date ||
                              v.endDate > data.firstBirthday) && (
                              <small>Extends outside the planning window</small>
                            )}
                          </div>
                          <button
                            className="icon"
                            aria-label={`Edit vacation ${v.name}`}
                            onClick={() =>
                              open({ kind: 'vacation', vacation: v })
                            }
                          >
                            <Pencil size={16} />
                          </button>
                          <button
                            className="icon"
                            disabled={busy}
                            aria-label={`Delete vacation ${v.name}`}
                            onClick={() => {
                              if (
                                confirm(
                                  `Delete vacation “${v.name}”? People’s assignments will remain.`,
                                )
                              )
                                submit(() => api.deleteVacation({ id: v.id }));
                            }}
                          >
                            <Trash2 size={16} />
                          </button>
                        </div>
                      ))}
                      {!data.vacations.length && (
                        <p className="empty">No vacations marked yet.</p>
                      )}
                    </div>
                  </section>
                  <HolidayList data={data} />
                </>
              )}
              {editor && (
                <EditorDialog
                  key={
                    editor.kind +
                    ('person' in editor
                      ? (editor.person?.id ?? '')
                      : 'assignment' in editor
                        ? (editor.assignment?.id ?? '')
                        : '')
                  }
                  editor={editor}
                  data={data}
                  busy={busy}
                  error={error}
                  close={() => setEditor(null)}
                  submit={submit}
                />
              )}
            </>
          )
        )}
      </main>
      <footer>Made for one little person. Planned together.</footer>
    </>
  );
}

function Calendar({
  data,
  month,
  onDay,
}: {
  data: GetPlanResponse;
  month: string;
  onDay: (date: string) => void;
}) {
  const days = new Map(data.days.map((d) => [d.date, d]));
  const holidays = new Map(data.holidays.map((h) => [h.date, h.name]));
  const people = new Map(data.people.map((p) => [p.id, p]));
  const first = parse(`${month}-01`);
  const offset = (first.getUTCDay() + 6) % 7;
  const last = new Date(first);
  last.setUTCMonth(last.getUTCMonth() + 1, 0);
  return (
    <div className="calendar">
      <div className="weekdays">
        {['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map((d) => (
          <span key={d}>{d}</span>
        ))}
      </div>
      <div className="calendar-days">
        {Array.from({ length: offset }, (_, i) => (
          <div className="day blank" key={`empty-${i}`} />
        ))}
        {Array.from({ length: last.getUTCDate() }, (_, i) => {
          const key = `${month}-${String(i + 1).padStart(2, '0')}`;
          const day = days.get(key);
          const holiday = holidays.get(key);
          const vacations = data.vacations.filter(
            (v) => v.startDate <= key && v.endDate >= key,
          );
          const weekend = [0, 6].includes(parse(key).getUTCDay());
          const prenatal = !!day && key < data.plan!.birthDate;
          const milestoneLabel =
            key === data.firstBirthday
              ? 'Day care'
              : key === data.plan!.birthDate
                ? data.plan!.actual
                  ? 'Birth day'
                  : 'Expected birth'
                : '';
          const gap = day?.required && !day.personIds.length;
          return (
            <button
              type="button"
              className={`day ${!day ? 'outside' : weekend ? 'weekend' : holiday ? 'holiday' : gap ? 'uncovered' : 'covered'}${milestoneLabel ? ' birth-day' : ''}`}
              key={key}
              disabled={!day || weekend || !data.people.length}
              onClick={() => onDay(key)}
              aria-label={`${fmt(key)}: ${vacations.map((v) => `Vacation: ${v.name}. `).join('')}${milestoneLabel ? milestoneLabel + '. ' : ''}${prenatal ? 'Before birth, optional coverage. ' : ''}${holiday ? holiday + '. ' : ''}${weekend ? 'Weekend' : !day ? 'Outside planning window' : day.personIds.length ? day.personIds.map((id) => people.get(id)?.name).join(', ') : day.required ? 'Needs coverage' : 'No coverage required'}`}
            >
              <span className="day-number">{i + 1}</span>
              {milestoneLabel && (
                <span className="birth-label">{milestoneLabel}</span>
              )}
              {day &&
                vacations.map((v) => (
                  <span
                    className="vacation-label"
                    key={v.id}
                    title={`Vacation: ${v.name}`}
                  >
                    Vacation · {v.name}
                  </span>
                ))}
              {day && !weekend && (
                <>
                  <div className="day-people">
                    {day.personIds.map((id) => (
                      <span
                        key={id}
                        style={{ borderColor: people.get(id)?.color }}
                      >
                        <i style={{ background: people.get(id)?.color }} />
                        {people.get(id)?.name}
                      </span>
                    ))}
                  </div>
                  {holiday ? (
                    <span className="day-label holiday-label" title={holiday}>
                      {holiday}
                    </span>
                  ) : gap ? (
                    <span className="day-label">Open</span>
                  ) : prenatal ? (
                    <span className="day-label">Before birth · optional</span>
                  ) : null}
                </>
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function EditorDialog({
  editor,
  data,
  busy,
  error,
  close,
  submit,
}: {
  editor: Editor;
  data: GetPlanResponse;
  busy: boolean;
  error: string;
  close: () => void;
  submit: (fn: () => Promise<unknown>) => void;
}) {
  const p = editor.kind === 'person' ? editor.person : undefined;
  const a = editor.kind === 'assignment' ? editor.assignment : undefined;
  const v = editor.kind === 'vacation' ? editor.vacation : undefined;
  const [color, setColor] = useState(
    p?.color ?? colors[data.people.length % colors.length],
  );
  const [personID, setPersonID] = useState(
    a?.personId ?? data.people[0]?.id ?? '',
  );
  const selected = data.people.find((p) => p.id === personID);
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const value = (key: string) => String(form.get(key) ?? '');
    if (editor.kind === 'plan')
      submit(() =>
        api.updatePlan({
          plan: {
            birthDate: value('birth'),
            actual: form.has('actual'),
          },
        }),
      );
    if (editor.kind === 'person')
      submit(() =>
        api.savePerson({
          person: {
            id: p?.id ?? '',
            name: value('name'),
            color,
            budgetDays:
              value('budget') === '' ? undefined : Number(value('budget')),
          },
        }),
      );
    if (editor.kind === 'vacation')
      submit(() =>
        api.saveVacation({
          vacation: {
            id: v?.id ?? '',
            name: value('name'),
            startDate: value('start'),
            endDate: value('end'),
          },
        }),
      );
    if (editor.kind === 'assignment')
      submit(() =>
        api.saveAssignment({
          assignment: {
            id: a?.id ?? '',
            personId: personID,
            startDate: value('start'),
            endDate: value('end'),
            notes: value('notes'),
          },
        }),
      );
  };
  return (
    <Modal
      title={
        editor.kind === 'plan'
          ? data.plan
            ? 'Birth date'
            : 'Let’s start with a date'
          : editor.kind === 'person'
            ? p
              ? 'Edit person'
              : 'Add a person'
            : editor.kind === 'vacation'
              ? v
                ? 'Edit vacation'
                : 'Add vacation'
              : a
                ? 'Edit coverage'
                : 'Add coverage'
      }
      onClose={close}
      busy={busy}
    >
      <form onSubmit={onSubmit}>
        <fieldset disabled={busy}>
          {editor.kind === 'vacation' && (
            <>
              <label>
                Vacation name
                <input
                  name="name"
                  required
                  maxLength={200}
                  defaultValue={v?.name ?? 'Vacation'}
                />
              </label>
              <div className="form-row">
                <label>
                  From
                  <input
                    name="start"
                    type="date"
                    required
                    defaultValue={v?.startDate ?? data.plan?.birthDate}
                  />
                </label>
                <label>
                  Through
                  <input
                    name="end"
                    type="date"
                    required
                    defaultValue={v?.endDate ?? data.plan?.birthDate}
                  />
                </label>
              </div>
              <p className="field-hint">
                Includes both dates, including weekends. This label doesn’t
                change coverage or leave budgets. Add people’s assignments
                separately.
              </p>
            </>
          )}
          {editor.kind === 'plan' && (
            <>
              <p className="muted">
                An expected date is all you need. You can update it when your
                little one arrives.
              </p>
              <label>
                Birth date
                <input
                  autoFocus
                  type="date"
                  name="birth"
                  required
                  defaultValue={data.plan?.birthDate}
                  max="9998-12-31"
                />
              </label>
              <label className="checkbox">
                <input
                  type="checkbox"
                  name="actual"
                  defaultChecked={data.plan?.actual}
                />{' '}
                Baby has arrived — this is the actual date
              </label>
              {data.plan && (
                <p className="notice">
                  Changing this date updates the coverage year. All existing
                  assignments keep their dates and continue to count towards
                  leave budgets.
                </p>
              )}
            </>
          )}
          {editor.kind === 'person' && (
            <>
              <label>
                Name
                <input
                  autoFocus
                  name="name"
                  required
                  maxLength={100}
                  defaultValue={p?.name}
                  placeholder="e.g. Mum, Dad, Grandma"
                />
              </label>
              <label>
                Leave budget <span className="optional">working days</span>
                <input
                  name="budget"
                  type="number"
                  min="0"
                  max="2147483647"
                  step="1"
                  defaultValue={p?.budgetDays}
                  placeholder="Unlimited"
                />
              </label>
              <p className="field-hint">
                Leave blank for unlimited availability. Weekends and public
                holidays don’t use leave.
              </p>
              <label>Color</label>
              <div className="swatches">
                {colors.map((c) => (
                  <button
                    type="button"
                    key={c}
                    style={{ background: c }}
                    aria-label={`Use color ${c}`}
                    aria-pressed={color === c}
                    onClick={() => setColor(c)}
                  >
                    {color === c && <Check size={17} />}
                  </button>
                ))}
                <input
                  type="color"
                  value={color}
                  onChange={(e) => setColor(e.target.value)}
                  aria-label="Custom person color"
                />
              </div>
            </>
          )}
          {editor.kind === 'assignment' && (
            <>
              <label>
                Person
                <select
                  aria-label="Person"
                  autoFocus
                  required
                  value={personID}
                  onChange={(e) => setPersonID(e.target.value)}
                >
                  {data.people.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </label>
              <div className="date-fields">
                <label>
                  From
                  <input
                    name="start"
                    type="date"
                    required
                    defaultValue={
                      a?.startDate ?? editor.start ?? data.plan?.birthDate
                    }
                  />
                </label>
                <label>
                  Through
                  <input
                    name="end"
                    type="date"
                    required
                    defaultValue={
                      a?.endDate ??
                      editor.end ??
                      editor.start ??
                      data.plan?.birthDate
                    }
                  />
                </label>
              </div>
              <p className="field-hint">
                Includes both dates. Coverage runs Monday–Friday. Public
                holidays don’t require coverage or use leave.
              </p>
              <label>
                Notes <span className="optional">optional</span>
                <textarea
                  name="notes"
                  maxLength={10000}
                  rows={3}
                  defaultValue={a?.notes}
                  placeholder="Anything useful to remember"
                />
              </label>
              {selected?.budgetDays !== undefined && (
                <p className="notice">
                  {selected.name} has a {countLabel(selected.budgetDays)} leave
                  budget. You can save a plan that exceeds it; we’ll flag the
                  extra days.
                </p>
              )}
            </>
          )}
          {error && (
            <p className="warning" role="alert">
              {error}
            </p>
          )}
          <div className="form-actions">
            {p && (
              <button
                type="button"
                className="danger-button"
                onClick={() => {
                  const n = data.assignments.filter(
                    (a) => a.personId === p.id,
                  ).length;
                  if (
                    confirm(
                      `Delete ${p.name}${n ? ` and their ${n} assignment(s)` : ''}?`,
                    )
                  )
                    submit(() => api.deletePerson({ id: p.id }));
                }}
              >
                <Trash2 size={16} /> Delete
              </button>
            )}
            <button type="button" onClick={close}>
              Cancel
            </button>
            <button className="primary" type="submit">
              {busy ? 'Saving…' : 'Save'}
            </button>
          </div>
        </fieldset>
      </form>
    </Modal>
  );
}

function HolidayList({ data }: { data: GetPlanResponse }) {
  return (
    <details className="holidays">
      <summary>Washington State holidays · 2027 & 2028</summary>
      <p>
        Observed Washington State holidays don’t require coverage or consume
        leave. Weekends are already excluded. This calendar covers 2027 and
        2028, including December 31, 2027 for New Year’s Day 2028.
      </p>
      <p>
        <a
          href="https://app.leg.wa.gov/rcw/default.aspx?cite=1.16.050"
          target="_blank"
          rel="noreferrer"
        >
          Washington State holiday and observance rules
        </a>
      </p>
      {data.plan &&
        (data.days[0].date < '2027-01-01' ||
          data.days.at(-1)!.date > '2028-12-31') && (
          <p className="notice">
            Your plan extends beyond the 2027–2028 holiday calendar. Outside
            those years, all weekdays count towards coverage and leave budgets.
          </p>
        )}
      <div className="holiday-list">
        {data.holidays.map((h) => (
          <div key={h.date}>
            <span>{fmt(h.date)}</span>
            <strong>{h.name}</strong>
          </div>
        ))}
      </div>
    </details>
  );
}

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </React.StrictMode>,
);
