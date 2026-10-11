# Melodical design

## Purpose

Help a pianist and guitarist learn songs through understanding keys and chords instead of memorizing every note independently. Develop quick recognition of piano chords and inversions, and fluency locating notes on the guitar neck.

The app must support short sessions away from instruments, including replacing habitual Reddit checking, as well as practice with an instrument in reach.

## Agreed first-version behavior

- Start a short practice session with one tap.
- Default to mixed practice, with focus options for key knowledge, piano chord recognition, and guitar note finding.
- Begin with major-key notes, basic triads and inversions, and guitar note locations. Introduce minor keys afterward.
- Offer piano, guitar, or both, remembering the last choice.
- Answer by locating notes on an interactive keyboard or fretboard; increase difficulty and reduce assistance over time.
- After mistakes, show the answer and a brief explanation, then revisit the material later in the session. Repeated mistakes lead to an easier exercise.
- Remember weak areas and select review exercises automatically. Track accuracy and response speed without a visible countdown.
- Provide optional note and chord playback; sessions remain usable muted. Dedicated ear training is deferred.
- Use standard guitar tuning in the first version.
- For piano chords, practice both constructing requested chords and identifying displayed chords.
- For guitar note finding, begin with a specified string and small fret range, then expand to regions and all occurrences.
- Let the learner choose a practice mode rather than imposing one session length.
- Provide quick rounds with a chosen question count, timed practice with a chosen session duration, and free practice until the learner stops. Timed practice does not impose per-answer deadlines.
- Keep all topics accessible. Recommend difficulty adaptively, allow manual overrides, and track proficiency separately by topic and instrument.
- Begin with note and chord names, then explicitly introduce scale degrees and Roman numerals.
- Initial curriculum covers guitar note finding, major-key notes, major/minor triad construction and recognition, inversions, and supporting interval exercises. Minor-key exercises follow the foundation; seventh chords, modes, and advanced harmony are deferred.
- Mix due review, weak areas, and a small amount of new material. Keep difficulty independent by topic and instrument; assisted answers alone do not establish proficiency.
- Generate questions from musical rules and supply curated explanations rather than requiring a manually authored question bank.
- Use a calm, tactile visual style with large instrument targets, minimal question text, optional explanations, and light/dark themes.
- Accept the correct pitch location for keyboard/fretboard answers and display the spelling appropriate to the musical context. Test note spelling separately when explicitly requested.
- Save completed answers immediately. Resume sessions on the same device, offering Resume or Start fresh after a long gap. Cross-device history is shared; live session handoff is deferred.
- Measure active answering time only, excluding hints, explanations, and background time. Keep assisted answers separate and exclude interrupted attempts from recognition-speed assessment.
- Share practice history across devices through the homelab. Offline practice is deferred.
- Use general musical exercises rather than song-specific content in the first version.
- Offer optional hints and track assisted answers separately. Measure answer time until submission, without a time penalty.
- Show accuracy, recognition speed, and weak areas; leave streaks out initially.
- Use a compact piano keyboard of roughly two octaves and a right-handed horizontal guitar fretboard showing the active region, with larger views on iPad/computer.
- Multi-note answers use an explicit Check action, allowing revision before feedback. Single-note answers submit immediately.
- Accept musically valid chord arrangements, including different spacings, unless the prompt explicitly restricts the voicing. For an inversion, require the correct lowest chord tone.
- Prioritize phones, while supporting iPad and computer use.
- Defer MIDI and audio detection of real-instrument playing.

## Hosting constraint

Eventually run on the user's k3s homelab, accessed over Tailscale. Deployment is managed in a separate repository; this repository provides `make dev` for local development and a buildable container image. Application authentication is out of scope for this setup.

Use Go and deploy as a single container. Store progress in SQLite initially, behind a replaceable storage interface, with persistent data outside the container's disposable filesystem.

Build a CGO-free, statically linked Linux executable using the pure-Go SQLite driver. Use sqlc-generated SQLite queries behind the storage interface.

Bundle a React frontend and embed its production assets in the Go executable, served from the same container. Go manages question generation, answer validation, and persisted progress; the browser manages instrument interactions and optional sound.

## Design status

The user confirmed shared understanding and authorized implementation. Exact visual styling, review intervals, timing thresholds, session presets, and API/database details are implementation choices to tune against this design.
