package music

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
)

type Point struct {
	MIDI   int `json:"midi"`
	String int `json:"string,omitempty"`
	Fret   int `json:"fret,omitempty"`
}
type Question struct {
	ID           string   `json:"id"`
	Instrument   string   `json:"instrument"`
	Topic        string   `json:"topic"`
	Kind         string   `json:"kind"`
	Prompt       string   `json:"prompt"`
	Detail       string   `json:"detail"`
	Key          string   `json:"key"`
	Root         int      `json:"root"`
	Level        int      `json:"level"`
	Skill        string   `json:"skill"`
	Single       bool     `json:"single"`
	MinMIDI      int      `json:"minMidi"`
	MaxMIDI      int      `json:"maxMidi"`
	MinFret      int      `json:"minFret"`
	MaxFret      int      `json:"maxFret"`
	TargetString int      `json:"targetString,omitempty"`
	Labels       []string `json:"labels"`
	Display      []Point  `json:"display,omitempty"`
	Choices      []string `json:"choices,omitempty"`
	Degrees      bool     `json:"degrees"`
	Expected     []Point  `json:"-"`
	PitchClasses []int    `json:"-"`
	Bass         int      `json:"-"`
	Choice       string   `json:"-"`
	Explanation  string   `json:"-"`
}

// StoredQuestion preserves private validation data without exposing it in prompts.
type StoredQuestion struct {
	Public       Question `json:"public"`
	Expected     []Point  `json:"expected"`
	PitchClasses []int    `json:"pitchClasses"`
	Bass         int      `json:"bass"`
	Choice       string   `json:"choice"`
	Explanation  string   `json:"explanation"`
}

func (q Question) Stored() StoredQuestion {
	return StoredQuestion{q, q.Expected, q.PitchClasses, q.Bass, q.Choice, q.Explanation}
}
func (s StoredQuestion) Question() Question {
	q := s.Public
	q.Expected = s.Expected
	q.PitchClasses = s.PitchClasses
	q.Bass = s.Bass
	q.Choice = s.Choice
	q.Explanation = s.Explanation
	return q
}

type Answer struct {
	Points      []Point `json:"points"`
	Choice      string  `json:"choice"`
	Assisted    bool    `json:"assisted"`
	Interrupted bool    `json:"interrupted"`
	ElapsedMS   int64   `json:"elapsedMs"`
}
type Feedback struct {
	Correct     bool    `json:"correct"`
	Expected    []Point `json:"expected"`
	Choice      string  `json:"choice,omitempty"`
	Explanation string  `json:"explanation"`
}

var Tuning = []int{40, 45, 50, 55, 59, 64} // low E to high E
var Keys = []string{"C", "G", "D", "A", "E", "B", "F#", "F", "Bb", "Eb", "Ab", "Db"}
var Roots = []int{0, 7, 2, 9, 4, 11, 6, 5, 10, 3, 8, 1}
var Topics = []string{"notes", "keys", "intervals", "chords", "inversions"}
var sharp = []string{"C", "C♯", "D", "D♯", "E", "F", "F♯", "G", "G♯", "A", "A♯", "B"}
var flat = []string{"C", "D♭", "D", "E♭", "E", "F", "G♭", "G", "A♭", "A", "B♭", "B"}
var steps = []int{0, 2, 4, 5, 7, 9, 11}

func PC(n int) int { return (n%12 + 12) % 12 }
func ID() string {
	b := make([]byte, 16)
	if _, err := cryptorand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Name(pc int, key string) string {
	if strings.Contains(key, "b") || key == "F" {
		return flat[PC(pc)]
	}
	return sharp[PC(pc)]
}
func Root(key string) int {
	for i, k := range Keys {
		if k == key {
			return Roots[i]
		}
	}
	return 0
}
func Scale(key string) ([]int, []string) { return spellScale(key, steps) }
func spellScale(key string, pattern []int) ([]int, []string) {
	root := Root(key)
	pcs := make([]int, 7)
	names := make([]string, 7)
	letters := []string{"C", "D", "E", "F", "G", "A", "B"}
	naturals := []int{0, 2, 4, 5, 7, 9, 11}
	start := 0
	for i, l := range letters {
		if strings.HasPrefix(key, l) {
			start = i
		}
	}
	for i, s := range pattern {
		pcs[i] = PC(root + s)
		li := (start + i) % 7
		diff := PC(pcs[i] - naturals[li])
		accidental := ""
		if diff == 1 {
			accidental = "♯"
		}
		if diff == 11 {
			accidental = "♭"
		}
		if diff == 10 {
			accidental = "♭♭"
		}
		if diff == 2 {
			accidental = "♯♯"
		}
		names[i] = letters[li] + accidental
	}
	return pcs, names
}
func Labels(key string) []string {
	out := make([]string, 12)
	for i := range out {
		out[i] = Name(i, key)
	}
	pcs, names := Scale(key)
	for i, p := range pcs {
		out[p] = names[i]
	}
	return out
}
func Generate(instrument, topic, key string, level int) Question {
	root := Root(key)
	labels := Labels(key)
	q := Question{ID: ID(), Instrument: instrument, Topic: topic, Key: key, Root: root, Level: level, MinMIDI: 48, MaxMIDI: 71, MinFret: 0, MaxFret: 4, Labels: labels, Bass: -1}
	if level >= 2 {
		q.MaxFret = 8
	}
	if level >= 3 {
		q.MaxFret = 12
		q.Degrees = true
	}
	q.Skill = fmt.Sprintf("%s:%s:%s", instrument, topic, key)
	switch topic {
	case "notes":
		q.Kind = "note"
		q.Single = true
		pc := rand.IntN(12)
		if level == 1 {
			pc = []int{0, 2, 4, 5, 7, 9, 11}[rand.IntN(7)]
		}
		q.Prompt = "Find " + labels[pc]
		q.Detail = "Tap one matching note."
		q.PitchClasses = []int{pc}
		if instrument == "guitar" {
			if level < 3 {
				q.TargetString = rand.IntN(6) + 1
				available := []int{}
				for f := q.MinFret; f <= q.MaxFret; f++ {
					available = append(available, PC(Tuning[q.TargetString-1]+f))
				}
				pc = available[rand.IntN(len(available))]
				q.PitchClasses = []int{pc}
				q.Prompt = "Find " + labels[pc]
				q.Detail = fmt.Sprintf("On string %d (%s), frets %d–%d. String 1 is high E; string 6 is low E.", 7-q.TargetString, Name(Tuning[q.TargetString-1], "C"), q.MinFret, q.MaxFret)
			} else {
				q.Kind = "all-notes"
				q.Single = false
				q.Prompt = "Find every " + labels[pc]
				q.Detail = "Select every occurrence in this fretboard region, then check."
				for s, open := range Tuning {
					for f := q.MinFret; f <= q.MaxFret; f++ {
						if PC(open+f) == pc {
							q.Expected = append(q.Expected, Point{MIDI: open + f, String: s + 1, Fret: f})
						}
					}
				}
			}
		}
		q.Explanation = "The highlighted positions are " + labels[pc] + ". Each guitar fret raises a note by one semitone; notes repeat every twelve frets."
		if instrument == "piano" {
			q.Explanation = labels[pc] + " repeats in each octave. Use the groups of two and three black keys as reference points."
		}
	case "keys":
		pcs, names := Scale(key)
		q.Kind = "scale"
		q.PitchClasses = pcs
		q.Prompt = "Find the notes of the " + displayKey(key) + " major scale"
		q.Detail = "Choose one occurrence of each scale note, in any octave."
		q.Explanation = displayKey(key) + " major contains " + strings.Join(names, " · ") + ". Major scales follow whole, whole, half, whole, whole, whole, half steps from the tonic. A song in this key may also use notes outside the scale."
		if level >= 3 && rand.IntN(2) == 0 {
			degree := rand.IntN(7)
			q.Kind = "note"
			q.Single = true
			q.PitchClasses = []int{pcs[degree]}
			q.Prompt = fmt.Sprintf("Find scale degree %d of the %s major scale", degree+1, displayKey(key))
			q.Detail = "Tap any occurrence of the note with this role in the key."
			q.Explanation = fmt.Sprintf("Degree %d of %s major is %s. Count from the tonic through the scale: %s.", degree+1, displayKey(key), names[degree], strings.Join(names, " · "))
		}
		if q.Degrees {
			q.Detail += " These are scale degrees 1–7, relative to the tonic."
		}
	case "intervals":
		offsets := []int{2, 4, 5, 7}
		titles := []string{"major second", "major third", "perfect fourth", "perfect fifth"}
		if level >= 2 {
			offsets = append(offsets, 1, 3, 8, 9, 11)
			titles = append(titles, "minor second", "minor third", "minor sixth", "major sixth", "major seventh")
		}
		idx := rand.IntN(len(offsets))
		offset := offsets[idx]
		target := PC(root + offset)
		q.Kind = "note"
		q.Single = true
		q.PitchClasses = []int{target}
		q.Prompt = "Find a " + titles[idx] + " above " + labels[root]
		q.Detail = "Tap any occurrence of the resulting note."
		degree := map[int]int{1: 2, 2: 2, 3: 3, 4: 3, 5: 4, 7: 5, 8: 6, 9: 6, 11: 7}[offset]
		pattern := append([]int{}, steps...)
		pattern[degree-1] = offset
		_, intervalNames := spellScale(key, pattern)
		labels = append([]string{}, labels...)
		labels[target] = intervalNames[degree-1]
		q.Labels = labels
		q.Explanation = fmt.Sprintf("A %s is %d semitones. Starting from %s gives %s.", titles[idx], offset, labels[root], labels[target])
	case "chords", "inversions":
		minor := rand.IntN(2) == 1
		roman := ""
		if level >= 3 {
			degree := rand.IntN(6)
			scale, _ := Scale(key)
			root = scale[degree]
			q.Root = root
			roman = []string{"I", "ii", "iii", "IV", "V", "vi"}[degree]
			minor = degree == 1 || degree == 2 || degree == 5
		}
		third := 4
		quality := "major"
		if minor {
			third = 3
			quality = "minor"
			minorPCs, minorNames := spellScale(key, []int{0, 2, 3, 5, 7, 8, 10})
			labels = append([]string{}, labels...)
			if roman == "" {
				for i, p := range minorPCs {
					labels[p] = minorNames[i]
				}
			}
			q.Labels = labels
		}
		pcs := []int{root, PC(root + third), PC(root + 7)}
		q.PitchClasses = pcs
		inv := 0
		if topic == "inversions" {
			inv = rand.IntN(2) + 1
		}
		q.Bass = pcs[inv]
		invName := []string{"root position", "first inversion", "second inversion"}[inv]
		chordName := labels[root] + " " + quality
		q.Kind = "chord"
		q.Prompt = "Build the " + chordName + " chord"
		q.Detail = "Select the three chord tones. Any spacing or doubling is valid."
		if instrument == "guitar" {
			q.Detail += " Use at most one fret per string."
		}
		if inv > 0 {
			q.Prompt += " in " + invName
			q.Detail += " The " + []string{"root", "third", "fifth"}[inv] + " must be the lowest note."
		} else {
			q.Bass = -1
		}
		q.Explanation = fmt.Sprintf("%s contains %s, %s, and %s: root, %s third, and perfect fifth. %s", chordName, labels[pcs[0]], labels[pcs[1]], labels[pcs[2]], quality, []string{"These chord tones can be arranged in different voicings.", "First inversion places the third in the bass.", "Second inversion places the fifth in the bass."}[inv])
		if roman != "" {
			q.Prompt = "Build the " + roman + " chord in the key of " + displayKey(key) + " major"
			if inv > 0 {
				q.Prompt += " in " + invName
			}
			q.Explanation += fmt.Sprintf(" In %s major, %s is %s: a chord built on scale degree %d.", displayKey(key), chordName, roman, slicesIndex([]string{"I", "ii", "iii", "IV", "V", "vi"}, roman)+1)
		}
		if q.Degrees {
			if minor {
				q.Explanation += " Its chord formula is 1–♭3–5."
			} else {
				q.Explanation += " Its chord formula is 1–3–5."
			}
		}
		if instrument == "piano" && rand.IntN(2) == 0 {
			q.Kind = "identify"
			q.Prompt = "Name this chord"
			q.Detail = "The highlighted notes form a triad. Choose its name"
			if topic == "inversions" {
				q.Detail += " and inversion"
			}
			q.Detail += "."
			q.Choice = chordName
			if topic == "inversions" {
				q.Choice += " · " + invName
			}
			base := 48 + root
			tones := []int{base, base + third, base + 7}
			for i := 0; i < inv; i++ {
				tones[i] += 12
			}
			sort.Ints(tones)
			if tones[len(tones)-1] > q.MaxMIDI {
				for i := range tones {
					tones[i] -= 12
				}
			}
			for _, m := range tones {
				q.Display = append(q.Display, Point{MIDI: m})
			}
			q.MaxMIDI = 71
			choices := []string{q.Choice}
			if topic == "inversions" {
				for _, v := range []string{"root position", "first inversion", "second inversion"} {
					candidate := chordName + " · " + v
					if candidate != q.Choice {
						choices = append(choices, candidate)
					}
				}
				other := "minor"
				if minor {
					other = "major"
				}
				choices = append(choices, labels[root]+" "+other+" · "+invName)
			} else {
				other := "minor"
				if minor {
					other = "major"
				}
				choices = append(choices, labels[root]+" "+other, labels[PC(root+5)]+" major", labels[PC(root+7)]+" minor")
			}
			rand.Shuffle(len(choices), func(i, j int) { choices[i], choices[j] = choices[j], choices[i] })
			if roman != "" {
				q.Prompt = "Name this chord’s role in the key of " + displayKey(key) + " major"
				q.Choice = roman
				if topic == "inversions" {
					q.Choice += " · " + invName
				}
				choices = []string{q.Choice}
				if topic == "inversions" {
					for _, position := range []string{"root position", "first inversion", "second inversion"} {
						candidate := roman + " · " + position
						if candidate != q.Choice {
							choices = append(choices, candidate)
						}
					}
					other := "I"
					if roman == "I" {
						other = "ii"
					}
					choices = append(choices, other+" · "+invName)
				} else {
					for _, role := range []string{"I", "ii", "iii", "IV", "V", "vi"} {
						if role != roman && len(choices) < 4 {
							choices = append(choices, role)
						}
					}
				}
				rand.Shuffle(len(choices), func(i, j int) { choices[i], choices[j] = choices[j], choices[i] })
				q.Detail = "Identify the Roman numeral"
				if topic == "inversions" {
					q.Detail += " and inversion"
				}
				q.Detail += " for the highlighted chord."
			}
			q.Choices = choices
		}
	}
	if len(q.Expected) == 0 && q.Kind != "identify" {
		q.Expected = q.example()
		for instrument == "guitar" && q.Kind == "chord" && len(q.Expected) < len(q.PitchClasses) && q.MaxFret < 12 {
			q.MaxFret++
			q.Expected = q.example()
		}
	}
	return q
}
func displayKey(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "#", "♯"), "b", "♭")
}
func (q Question) example() []Point {
	result := []Point{}
	pcs := append([]int{}, q.PitchClasses...)
	if q.Bass >= 0 {
		sort.SliceStable(pcs, func(i, j int) bool { return PC(pcs[i]-q.Bass) < PC(pcs[j]-q.Bass) })
	}
	if q.Instrument == "guitar" && q.Kind == "chord" {
		var search func(int, int, map[int]bool) bool
		search = func(index, lowest int, used map[int]bool) bool {
			if index == len(pcs) {
				return true
			}
			for s, open := range Tuning {
				if used[s] {
					continue
				}
				for f := q.MinFret; f <= q.MaxFret; f++ {
					m := open + f
					if PC(m) != pcs[index] || m <= lowest {
						continue
					}
					used[s] = true
					result = append(result, Point{MIDI: m, String: s + 1, Fret: f})
					if search(index+1, m, used) {
						return true
					}
					result = result[:len(result)-1]
					delete(used, s)
				}
			}
			return false
		}
		original := append([]int{}, pcs...)
		orders := [][]int{{original[0], original[1], original[2]}, {original[0], original[2], original[1]}}
		if q.Bass < 0 {
			orders = append(orders, []int{original[1], original[0], original[2]}, []int{original[1], original[2], original[0]}, []int{original[2], original[0], original[1]}, []int{original[2], original[1], original[0]})
		}
		for _, order := range orders {
			pcs = order
			if search(0, 0, map[int]bool{}) {
				break
			}
		}
		return result
	}
	last := q.MinMIDI - 1
	for _, pc := range pcs {
		if q.Instrument == "piano" {
			for m := last + 1; m <= q.MaxMIDI; m++ {
				if PC(m) == pc {
					result = append(result, Point{MIDI: m})
					last = m
					break
				}
			}
		} else {
			found := false
			for s, open := range Tuning {
				if q.TargetString > 0 && s+1 != q.TargetString {
					continue
				}
				for f := q.MinFret; f <= q.MaxFret; f++ {
					if PC(open+f) == pc && (q.Bass < 0 || open+f > last) {
						result = append(result, Point{MIDI: open + f, String: s + 1, Fret: f})
						last = open + f
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
	}
	return result
}
func (q Question) Validate(a Answer) Feedback {
	feedback := Feedback{Expected: q.Expected, Choice: q.Choice, Explanation: q.Explanation}
	if q.Kind == "identify" {
		feedback.Correct = a.Choice == q.Choice
		feedback.Expected = q.Display
		return feedback
	}
	if len(a.Points) == 0 || len(a.Points) > 100 {
		return feedback
	}
	seen := map[string]bool{}
	got := map[int]bool{}
	bass := 999
	stringsUsed := map[int]bool{}
	for _, p := range a.Points {
		if q.Instrument == "piano" {
			if p.MIDI < q.MinMIDI || p.MIDI > q.MaxMIDI || p.String != 0 {
				return feedback
			}
		} else {
			if p.String < 1 || p.String > 6 || p.Fret < q.MinFret || p.Fret > q.MaxFret || Tuning[p.String-1]+p.Fret != p.MIDI || (q.TargetString > 0 && q.TargetString != p.String) {
				return feedback
			}
		}
		if q.Instrument == "guitar" && q.Kind == "chord" {
			if stringsUsed[p.String] {
				return feedback
			}
			stringsUsed[p.String] = true
		}
		cell := fmt.Sprintf("%d:%d:%d", p.MIDI, p.String, p.Fret)
		if seen[cell] {
			return feedback
		}
		seen[cell] = true
		got[PC(p.MIDI)] = true
		if p.MIDI < bass {
			bass = p.MIDI
		}
	}
	if q.Single && len(a.Points) != 1 {
		return feedback
	}
	if q.Kind == "all-notes" {
		if len(a.Points) != len(q.Expected) {
			return feedback
		}
		for _, p := range q.Expected {
			if !seen[fmt.Sprintf("%d:%d:%d", p.MIDI, p.String, p.Fret)] {
				return feedback
			}
		}
		feedback.Correct = true
		feedback.Expected = append([]Point{}, a.Points...)
		return feedback
	}
	if len(got) != len(q.PitchClasses) {
		return feedback
	}
	for _, p := range q.PitchClasses {
		if !got[p] {
			return feedback
		}
	}
	if q.Bass >= 0 && PC(bass) != q.Bass {
		return feedback
	}
	feedback.Correct = true
	feedback.Expected = append([]Point{}, a.Points...)
	return feedback
}

func slicesIndex(values []string, value string) int {
	for i, v := range values {
		if v == value {
			return i
		}
	}
	return -1
}
