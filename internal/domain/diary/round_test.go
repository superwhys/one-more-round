package diary

import "testing"

func TestRoundRules(t *testing.T) {
	base := func() Round {
		return Round{
			GameID:  "g",
			Date:    "2026-09-18",
			Mode:    "individual",
			Outcome: "win",
			Players: []string{"a", "b"},
			Winners: []string{"a"},
		}
	}
	cases := []struct {
		name   string
		change func(*Round)
		ok     bool
	}{
		{"joint winners", func(r *Round) { r.Winners = []string{"a", "b"} }, true},
		{"draw", func(r *Round) { r.Outcome = "draw"; r.Winners = nil }, true},
		{"unknown", func(r *Round) { r.Outcome = "unknown"; r.Winners = nil }, true},
		{"duplicate", func(r *Round) { r.Players = []string{"a", "a"} }, false},
		{"three photos", func(r *Round) { r.Photos = []string{"p1", "p2", "p3"} }, true},
		{"four photos", func(r *Round) { r.Photos = []string{"p1", "p2", "p3", "p4"} }, false},
		{"duplicate photos", func(r *Round) { r.Photos = []string{"p1", "p1"} }, false},
		{"single competitive", func(r *Round) { r.Players = []string{"a"} }, false},
		{
			"solo coop",
			func(r *Round) { r.Mode = "coop"; r.Players = []string{"a"}; r.Winners = nil },
			true,
		},
		{
			"coop failure",
			func(r *Round) { r.Mode = "coop"; r.Outcome = "loss"; r.Winners = nil },
			true,
		},
		{"invalid date", func(r *Round) { r.Date = "2026-02-30" }, false},
		{"unselected winner", func(r *Round) { r.Winners = []string{"z"} }, false},
		{
			"negative decimal",
			func(r *Round) { s := "-10.0250"; r.Scores = map[string]*string{"a": &s, "b": nil} },
			true,
		},
		{"zero", func(r *Round) { s := "0"; r.Scores = map[string]*string{"a": &s} }, true},
		{
			"precision",
			func(r *Round) { s := "0.00001"; r.Scores = map[string]*string{"a": &s} },
			false,
		},
		{"teams", func(r *Round) {
			r.Mode = "team"
			r.Winners = nil
			r.Teams = []Team{
				{ID: "a", Name: "A", Players: []string{"a"}, Winner: true},
				{ID: "b", Name: "B", Players: []string{"b"}, Winner: true},
			}
		}, true},
		{"empty team", func(r *Round) {
			r.Mode = "team"
			r.Winners = nil
			r.Teams = []Team{
				{ID: "a", Name: "A", Players: []string{"a", "b"}, Winner: true},
				{ID: "b", Name: "B"},
			}
		}, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := base()
			tt.change(&r)
			if (r.Validate() == nil) != tt.ok {
				t.Fatalf("unexpected: %v", r.Validate())
			}
		})
	}
}

func TestUnicodeMemory(t *testing.T) {
	r := Round{
		GameID:  "g",
		Date:    "2026-09-18",
		Mode:    "coop",
		Outcome: "unknown",
		Players: []string{"a"},
	}
	for i := 0; i < 500; i++ {
		r.Memory += "局"
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.Memory += "🧡"
	if r.Validate() == nil {
		t.Fatal("accepted 501 runes")
	}
}

func TestTeamWin(t *testing.T) {
	r := Round{
		Players: []string{"a", "b"},
		Mode:    "team",
		Outcome: "win",
		Teams:   []Team{{Players: []string{"a"}, Winner: true}, {Players: []string{"b"}}},
	}
	if !r.Won("a") || r.Won("b") {
		t.Fatal("team results not inherited")
	}
}

func TestListAdvancedFilters(t *testing.T) {
	hasPhotos := true
	rounds := []*Round{
		{
			ID:      "one",
			GameID:  "g",
			Date:    "2026-09-01",
			Mode:    "coop",
			Outcome: "win",
			Players: []string{"a"},
			Memory:  "第一次打通",
			Photos:  []string{"p"},
		},
		{
			ID:      "two",
			GameID:  "g",
			Date:    "2026-09-02",
			Mode:    "individual",
			Outcome: "draw",
			Players: []string{"a", "b"},
			Memory:  "势均力敌",
		},
	}
	page, err := List(
		rounds,
		Filter{Query: "打通", Mode: "coop", Outcome: "win", HasPhotos: &hasPhotos, Limit: 30},
	)
	if err != nil || page.Total != 1 || page.Items[0].ID != "one" {
		t.Fatalf("advanced filter = %#v, %v", page, err)
	}
	if _, err = List(rounds, Filter{Mode: "mystery", Limit: 30}); err == nil {
		t.Fatal("invalid mode accepted")
	}
}

func TestBuildRecap(t *testing.T) {
	minutes := 45
	r := BuildRecap([]*Round{
		{
			GameID:  "g1",
			Date:    "2026-09-01",
			Players: []string{"a", "b"},
			Minutes: &minutes,
			Photos:  []string{"p1"},
		},
		{GameID: "g1", Date: "2026-09-20", Players: []string{"a"}, Photos: []string{"p2"}},
		{GameID: "g2", Date: "2026-10-01", Players: []string{"b"}},
	}, "2026-09-01", "2026-09-30")
	if r.Rounds != 2 || r.Games != 1 || r.Players != 2 || r.Minutes != 45 || r.TopGame != "g1" ||
		r.TopPlayer != "a" ||
		len(r.Photos) != 2 {
		t.Fatalf("unexpected recap: %#v", r)
	}
}
