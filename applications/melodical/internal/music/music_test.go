package music

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestGeneratedExercisesHaveValidSolutions(t *testing.T) {
	for _, instrument := range []string{"piano", "guitar"} {
		for _, topic := range Topics {
			for _, key := range Keys {
				for level := 1; level <= 3; level++ {
					for n := 0; n < 25; n++ {
						q := Generate(instrument, topic, key, level)
						if !q.Validate(Answer{Points: q.Expected, Choice: q.Choice}).Correct {
							t.Fatalf("invalid generated solution: %s %s %s level %d %s expected %+v pitch classes %v bass %d", instrument, topic, key, level, q.Prompt, q.Expected, q.PitchClasses, q.Bass)
						}
						for _, p := range q.Display {
							if p.MIDI < q.MinMIDI || p.MIDI > q.MaxMIDI {
								t.Fatalf("displayed note out of view: %+v, %+v", p, q)
							}
						}
						public, _ := json.Marshal(q)
						for _, private := range []string{"expected", "pitchClasses", "explanation"} {
							if strings.Contains(string(public), `"`+private+`"`) {
								t.Fatalf("prompt leaks %s", private)
							}
						}
						saved, _ := json.Marshal(q.Stored())
						var stored StoredQuestion
						if err := json.Unmarshal(saved, &stored); err != nil {
							t.Fatal(err)
						}
						if !stored.Question().Validate(Answer{Points: q.Expected, Choice: q.Choice}).Correct {
							t.Fatal("storage roundtrip lost validation data")
						}
					}
				}
			}
		}
	}
}
func TestChordVoicingsAndInversions(t *testing.T) {
	q := Question{Instrument: "piano", Kind: "chord", MinMIDI: 48, MaxMIDI: 71, PitchClasses: []int{0, 4, 7}, Bass: 4}
	for _, points := range [][]Point{{{MIDI: 52}, {MIDI: 55}, {MIDI: 60}}, {{MIDI: 52}, {MIDI: 60}, {MIDI: 67}, {MIDI: 64}}} {
		if !q.Validate(Answer{Points: points}).Correct {
			t.Errorf("valid inversion rejected: %+v", points)
		}
	}
	for _, points := range [][]Point{{{MIDI: 48}, {MIDI: 52}, {MIDI: 55}}, {{MIDI: 52}, {MIDI: 55}}, {{MIDI: 52}, {MIDI: 55}, {MIDI: 61}}, {{MIDI: 52}, {MIDI: 55}, {MIDI: 60}, {MIDI: 60}}} {
		if q.Validate(Answer{Points: points}).Correct {
			t.Errorf("invalid inversion accepted: %+v", points)
		}
	}
}
func TestFretboardCoordinatesAndCompleteRegion(t *testing.T) {
	q := Question{Instrument: "guitar", Kind: "note", Single: true, MinFret: 0, MaxFret: 4, PitchClasses: []int{0}, TargetString: 2, Bass: -1}
	if !q.Validate(Answer{Points: []Point{{MIDI: 48, String: 2, Fret: 3}}}).Correct {
		t.Fatal("A string fret 3 should be C")
	}
	if q.Validate(Answer{Points: []Point{{MIDI: 48, String: 2, Fret: 2}}}).Correct {
		t.Fatal("forged MIDI accepted")
	}
	q = Generate("guitar", "notes", "C", 3)
	if q.Validate(Answer{Points: q.Expected[:len(q.Expected)-1]}).Correct {
		t.Fatal("incomplete all-occurrences answer accepted")
	}
}
func TestSpelling(t *testing.T) {
	_, names := Scale("F#")
	if !slices.Contains(names, "E♯") {
		t.Fatal("F# major must use E#, not F")
	}
	_, names = Scale("Db")
	if !slices.Equal(names, []string{"D♭", "E♭", "F", "G♭", "A♭", "B♭", "C"}) {
		t.Fatal(names)
	}
	_, names = spellScale("Db", []int{0, 2, 3, 5, 7, 8, 10})
	if names[2] != "F♭" {
		t.Fatal("Db minor's third must be Fb", names)
	}
}
