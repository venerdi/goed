package edGalaxy

import (
	"encoding/json"
	"fmt"
	"testing"
)

type tStep struct {
	val int64
	expected string
}

var (
tSteps = []tStep{ 
	tStep{2, `[{"time_mark":2,"visit_count":1}]`},
	tStep{4, `[{"time_mark":4,"visit_count":1},{"time_mark":2,"visit_count":1}]`},                                                                
	tStep{6, `[{"time_mark":6,"visit_count":1},{"time_mark":4,"visit_count":1},{"time_mark":2,"visit_count":1}]`},                                
	tStep{1, `[{"time_mark":6,"visit_count":1},{"time_mark":4,"visit_count":1},{"time_mark":2,"visit_count":1},{"time_mark":1,"visit_count":1}]`},
	tStep{6, `[{"time_mark":6,"visit_count":2},{"time_mark":4,"visit_count":1},{"time_mark":2,"visit_count":1},{"time_mark":1,"visit_count":1}]`},
	tStep{4, `[{"time_mark":6,"visit_count":2},{"time_mark":4,"visit_count":2},{"time_mark":2,"visit_count":1},{"time_mark":1,"visit_count":1}]`},
	tStep{2, `[{"time_mark":6,"visit_count":2},{"time_mark":4,"visit_count":2},{"time_mark":2,"visit_count":2},{"time_mark":1,"visit_count":1}]`},
	tStep{5, `[{"time_mark":6,"visit_count":2},{"time_mark":5,"visit_count":1},{"time_mark":4,"visit_count":2},{"time_mark":2,"visit_count":2}]`},
	tStep{8, `[{"time_mark":8,"visit_count":1},{"time_mark":6,"visit_count":2},{"time_mark":5,"visit_count":1},{"time_mark":4,"visit_count":2}]`},
	tStep{7, `[{"time_mark":8,"visit_count":1},{"time_mark":7,"visit_count":1},{"time_mark":6,"visit_count":2},{"time_mark":5,"visit_count":1}]`}}
)

func dump(c *TimeVisitStatCollector) {
	b, err := json.Marshal(c)
	if err != nil {
		fmt.Printf("Failed to dump : %v\n", err)
		return
	}
	fmt.Println("---")
	fmt.Println(string(b))
}

func compare(t *testing.T, c *TimeVisitStatCollector, expected string) bool {
	var expectedVisits []TimeVisitStat
	err := json.Unmarshal([]byte(expected), expectedVisits)
	if err != nil {
		t.Fatalf("Decode expected string failed: %v\n", err)
		return false
	}
	if len(expectedVisits) != len(c.Visits) {
		dump(c)
		t.Fatalf("Length mismatch expected: %s\n", expected)
		return false
	}
	for i, r := range c.Visits {
		if expectedVisits[i].VisitCount != r.VisitCount {
			dump(c)
			t.Fatalf("Visit count mismatch expected: %s\n", expected)
		}
		if expectedVisits[i].Timemark != r.Timemark {
			dump(c)
			t.Fatalf("Time mark mismatch expected: %s\n", expected)
		}
	}
	return true
}

func TestTimeVisitStatCollector(t *testing.T) {
	c := NewTimeVisitStatCollector(4, 1)
	for _, st := range tSteps {
		c.noteVisitByTimemark(st.val)
		
	}
}
