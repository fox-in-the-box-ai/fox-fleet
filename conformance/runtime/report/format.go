package report

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Status string

const (
	Pass Status = "PASS"
	Fail Status = "FAIL"
	Skip Status = "SKIP"
)

type Result struct {
	Number   int
	Name     string
	Status   Status
	Duration time.Duration
	Detail   string
}

type Suite struct {
	Image   string
	Results []Result
	Elapsed time.Duration
}

func (s *Suite) Add(r Result) { s.Results = append(s.Results, r) }

func (s *Suite) Passed() int {
	n := 0
	for _, r := range s.Results {
		if r.Status == Pass {
			n++
		}
	}
	return n
}

func (s *Suite) Failed() int {
	n := 0
	for _, r := range s.Results {
		if r.Status == Fail {
			n++
		}
	}
	return n
}

func (s *Suite) Skipped() int {
	n := 0
	for _, r := range s.Results {
		if r.Status == Skip {
			n++
		}
	}
	return n
}

func (s *Suite) OK() bool { return s.Failed() == 0 }

func (s *Suite) Format() string {
	sort.Slice(s.Results, func(i, j int) bool {
		return s.Results[i].Number < s.Results[j].Number
	})

	var b strings.Builder
	fmt.Fprintf(&b, "Runtime Conformance — %s\n", s.Image)
	b.WriteString(strings.Repeat("=", 60))
	b.WriteString("\n\n")

	for _, r := range s.Results {
		marker := "[PASS]"
		switch r.Status {
		case Fail:
			marker = "[FAIL]"
		case Skip:
			marker = "[SKIP]"
		}
		fmt.Fprintf(&b, "  %s %02d %-42s %s\n", marker, r.Number, r.Name, r.Duration.Round(time.Millisecond))
		if r.Detail != "" && r.Status != Pass {
			fmt.Fprintf(&b, "        -> %s\n", r.Detail)
		}
	}

	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", 60))
	fmt.Fprintf(&b, "\n  %d passed, %d failed, %d skipped (%s)\n",
		s.Passed(), s.Failed(), s.Skipped(), s.Elapsed.Round(time.Millisecond))

	return b.String()
}
