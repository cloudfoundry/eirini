package spec_test

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"testing"

	"github.com/sclevine/spec"
)

func optionTestSpec(t *testing.T, it spec.S, s recorder, name string) {
	if name != "" {
		name += "."
	}
	it.Before(s(t, name+"Before.1"))
	it.Before(s(t, name+"Before.2"))
	it.Before(s(t, name+"Before.3"))
	it.After(s(t, name+"After.1"))
	it.After(s(t, name+"After.2"))
	it.After(s(t, name+"After.3"))
	it(name+"S", s(t, name+"S"))
}

func optionTestSpecs(t *testing.T, it spec.S, s recorder, name string) {
	if name != "" {
		name += "."
	}
	it(name+"S.1", s(t, name+"S.1"))
	it(name+"S.2", s(t, name+"S.2"))
	it(name+"S.3", s(t, name+"S.3"))
}

func optionTestCases(t *testing.T, when spec.G, it spec.S, s recorder) {
	optionTestSpecs(t, it, s, "")
	when("G", func() {
		optionTestSpec(t, it, s, "G")
	})
	when("G.Sequential", func() {
		optionTestSpecs(t, it, s, "G.Sequential")
	}, spec.Sequential())
	when("G.Reverse", func() {
		optionTestSpecs(t, it, s, "G.Reverse")
	}, spec.Reverse())
	when("G.Random.Local", func() {
		optionTestSpecs(t, it, s, "G.Random.Local")
	}, spec.Random(), spec.Local())
	when("G.Random.Global", func() {
		optionTestSpecs(t, it, s, "G.Random.Global")
	}, spec.Random(), spec.Global())
}

func optionDefaultOrder(t *testing.T, name string, seed int64) []string {
	s, calls := record(t)

	spec.Run(t, name, func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	}, spec.Seed(seed))

	results := calls()
	for i := range results {
		results[i] = regexp.MustCompile(`#[0-9]+`).ReplaceAllLiteralString(results[i], "")
	}

	return results
}

type testReporter struct {
	StartT, SpecsT *testing.T
	StartPlan      spec.Plan
	SpecOrder      []spec.Spec
}

func (tr *testReporter) Start(t *testing.T, plan spec.Plan) {
	tr.StartT = t
	tr.StartPlan = plan
}

func (tr *testReporter) Specs(t *testing.T, specs <-chan spec.Spec) {
	tr.SpecsT = t
	for s := range specs {
		tr.SpecOrder = append(tr.SpecOrder, s)
	}
}

func TestReport(t *testing.T) {
	s, calls := record(t)
	reporter := &testReporter{}

	suite := spec.New("Suite", spec.Report(reporter), spec.Seed(2))
	suite("Top", func(t *testing.T, when spec.G, it spec.S) {
		when("Top.G.Out", func() {
			it("Top.G.Out.1", func() {
				fmt.Fprint(it.Out(), "Top.G.Out.1")
			})
			it("Top.G.Out.2", func() {
				fmt.Fprint(it.Out(), "Top.G.Out.2")
			})
		}, spec.Reverse())
		optionTestCases(t, when, it, s)
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), optionDefaultOrder(t, "Suite/Top", 2)) {
		t.Fatal("Incorrect order:", calls())
	}
	if reporter.StartT != t {
		t.Fatal("Incorrect value for t on start.")
	}
	if reporter.SpecsT != t {
		t.Fatal("Incorrect value for t on spec run.")
	}
	if reporter.StartPlan != (spec.Plan{
		Text:      "Suite",
		Total:     18,
		Pending:   0,
		Focused:   0,
		Seed:      2,
		HasRandom: true,
		HasFocus:  false,
	}) {
		t.Fatal("Incorrect plan:", reporter.StartPlan)
	}

	out2, err := ioutil.ReadAll(reporter.SpecOrder[0].Out)
	if string(out2) != "Top.G.Out.2" || err != nil {
		t.Fatal("Incorrect output for Top.G.Out.2 buffer.")
	}
	out1, err := ioutil.ReadAll(reporter.SpecOrder[1].Out)
	if string(out1) != "Top.G.Out.1" || err != nil {
		t.Fatal("Incorrect output for Top.G.Out.1 buffer.")
	}
	empty, err := ioutil.ReadAll(reporter.SpecOrder[2].Out)
	if string(empty) != "" || err != nil {
		t.Fatal("Incorrect output for empty buffer.")
	}
	for i := range reporter.SpecOrder {
		reporter.SpecOrder[i].Out = nil
	}

	if !reflect.DeepEqual(reporter.SpecOrder, []spec.Spec{
		{Text: []string{"Top", "Top.G.Out", "Top.G.Out.2"}}, {Text: []string{"Top", "Top.G.Out", "Top.G.Out.1"}},
		{Text: []string{"Top", "S.1"}}, {Text: []string{"Top", "S.2"}}, {Text: []string{"Top", "S.3"}},
		{Text: []string{"Top", "G", "G.S"}},
		{Text: []string{"Top", "G.Sequential", "G.Sequential.S.1"}},
		{Text: []string{"Top", "G.Sequential", "G.Sequential.S.2"}},
		{Text: []string{"Top", "G.Sequential", "G.Sequential.S.3"}},
		{Text: []string{"Top", "G.Reverse", "G.Reverse.S.3"}},
		{Text: []string{"Top", "G.Reverse", "G.Reverse.S.2"}},
		{Text: []string{"Top", "G.Reverse", "G.Reverse.S.1"}},
		{Text: []string{"Top", "G.Random.Local", "G.Random.Local.S.3"}},
		{Text: []string{"Top", "G.Random.Local", "G.Random.Local.S.1"}},
		{Text: []string{"Top", "G.Random.Local", "G.Random.Local.S.2"}},
		{Text: []string{"Top", "G.Random.Global", "G.Random.Global.S.3"}},
		{Text: []string{"Top", "G.Random.Global", "G.Random.Global.S.1"}},
		{Text: []string{"Top", "G.Random.Global", "G.Random.Global.S.2"}},
	}) {
		t.Fatal("Incorrect reported order:", reporter.SpecOrder)
	}
}

func TestDefault(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite", spec.Seed(2))
	suite("Top", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), []string{
		"Suite/Top/S.1->S.1", "Suite/Top/S.2->S.2", "Suite/Top/S.3->S.3",

		"Suite/Top/G/G.S->G.Before.1", "Suite/Top/G/G.S->G.Before.2", "Suite/Top/G/G.S->G.Before.3",
		"Suite/Top/G/G.S->G.S",
		"Suite/Top/G/G.S->G.After.1", "Suite/Top/G/G.S->G.After.2", "Suite/Top/G/G.S->G.After.3",

		"Suite/Top/G.Sequential/G.Sequential.S.1->G.Sequential.S.1",
		"Suite/Top/G.Sequential/G.Sequential.S.2->G.Sequential.S.2",
		"Suite/Top/G.Sequential/G.Sequential.S.3->G.Sequential.S.3",

		"Suite/Top/G.Reverse/G.Reverse.S.3->G.Reverse.S.3",
		"Suite/Top/G.Reverse/G.Reverse.S.2->G.Reverse.S.2",
		"Suite/Top/G.Reverse/G.Reverse.S.1->G.Reverse.S.1",

		"Suite/Top/G.Random.Local/G.Random.Local.S.3->G.Random.Local.S.3",
		"Suite/Top/G.Random.Local/G.Random.Local.S.1->G.Random.Local.S.1",
		"Suite/Top/G.Random.Local/G.Random.Local.S.2->G.Random.Local.S.2",

		"Suite/Top/G.Random.Global/G.Random.Global.S.3->G.Random.Global.S.3",
		"Suite/Top/G.Random.Global/G.Random.Global.S.1->G.Random.Global.S.1",
		"Suite/Top/G.Random.Global/G.Random.Global.S.2->G.Random.Global.S.2",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestSequential(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite", spec.Sequential(), spec.Seed(2))
	suite("Top", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), optionDefaultOrder(t, "Suite/Top", 2)) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestRandom(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite", spec.Random(), spec.Seed(2))
	suite("Top", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), []string{
		"Suite/Top/G.Random.Local/G.Random.Local.S.3->G.Random.Local.S.3",
		"Suite/Top/G.Random.Local/G.Random.Local.S.1->G.Random.Local.S.1",
		"Suite/Top/G.Random.Local/G.Random.Local.S.2->G.Random.Local.S.2",

		"Suite/Top/G/G.S->G.Before.1", "Suite/Top/G/G.S->G.Before.2", "Suite/Top/G/G.S->G.Before.3",
		"Suite/Top/G/G.S->G.S",
		"Suite/Top/G/G.S->G.After.1", "Suite/Top/G/G.S->G.After.2", "Suite/Top/G/G.S->G.After.3",

		"Suite/Top/G.Random.Global/G.Random.Global.S.3->G.Random.Global.S.3",
		"Suite/Top/G.Random.Global/G.Random.Global.S.1->G.Random.Global.S.1",
		"Suite/Top/G.Random.Global/G.Random.Global.S.2->G.Random.Global.S.2",

		"Suite/Top/G.Sequential/G.Sequential.S.1->G.Sequential.S.1",
		"Suite/Top/G.Sequential/G.Sequential.S.2->G.Sequential.S.2",
		"Suite/Top/G.Sequential/G.Sequential.S.3->G.Sequential.S.3",

		"Suite/Top/G.Reverse/G.Reverse.S.3->G.Reverse.S.3",
		"Suite/Top/G.Reverse/G.Reverse.S.2->G.Reverse.S.2",
		"Suite/Top/G.Reverse/G.Reverse.S.1->G.Reverse.S.1",

		"Suite/Top/S.1->S.1", "Suite/Top/S.2->S.2", "Suite/Top/S.3->S.3",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestReverse(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite", spec.Reverse(), spec.Seed(2))
	suite("Top", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), []string{
		"Suite/Top/G.Random.Global/G.Random.Global.S.3->G.Random.Global.S.3",
		"Suite/Top/G.Random.Global/G.Random.Global.S.1->G.Random.Global.S.1",
		"Suite/Top/G.Random.Global/G.Random.Global.S.2->G.Random.Global.S.2",

		"Suite/Top/G.Random.Local/G.Random.Local.S.3->G.Random.Local.S.3",
		"Suite/Top/G.Random.Local/G.Random.Local.S.1->G.Random.Local.S.1",
		"Suite/Top/G.Random.Local/G.Random.Local.S.2->G.Random.Local.S.2",

		"Suite/Top/G.Reverse/G.Reverse.S.3->G.Reverse.S.3",
		"Suite/Top/G.Reverse/G.Reverse.S.2->G.Reverse.S.2",
		"Suite/Top/G.Reverse/G.Reverse.S.1->G.Reverse.S.1",

		"Suite/Top/G.Sequential/G.Sequential.S.1->G.Sequential.S.1",
		"Suite/Top/G.Sequential/G.Sequential.S.2->G.Sequential.S.2",
		"Suite/Top/G.Sequential/G.Sequential.S.3->G.Sequential.S.3",

		"Suite/Top/G/G.S->G.Before.1", "Suite/Top/G/G.S->G.Before.2", "Suite/Top/G/G.S->G.Before.3",
		"Suite/Top/G/G.S->G.S",
		"Suite/Top/G/G.S->G.After.1", "Suite/Top/G/G.S->G.After.2", "Suite/Top/G/G.S->G.After.3",

		"Suite/Top/S.3->S.3", "Suite/Top/S.2->S.2", "Suite/Top/S.1->S.1",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestLocal(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite", spec.Local(), spec.Seed(2))
	suite("Top", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), optionDefaultOrder(t, "Suite/Top", 2)) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestGlobal(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite", spec.Random(), spec.Seed(2), spec.Global())
	suite("Top", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), []string{
		"Suite/Top/G.Reverse/G.Reverse.S.3->G.Reverse.S.3",

		"Suite/Top/G.Random.Global/G.Random.Global.S.2->G.Random.Global.S.2",

		"Suite/Top/S.1->S.1",

		"Suite/Top/G.Random.Local/G.Random.Local.S.3->G.Random.Local.S.3",
		"Suite/Top/G.Random.Local/G.Random.Local.S.1->G.Random.Local.S.1",
		"Suite/Top/G.Random.Local/G.Random.Local.S.2->G.Random.Local.S.2",

		"Suite/Top/G.Sequential/G.Sequential.S.3->G.Sequential.S.3",

		"Suite/Top/S.2->S.2",

		"Suite/Top/G.Reverse/G.Reverse.S.1->G.Reverse.S.1",

		"Suite/Top/S.3->S.3",

		"Suite/Top/G.Reverse/G.Reverse.S.2->G.Reverse.S.2",

		"Suite/Top/G.Sequential/G.Sequential.S.1->G.Sequential.S.1",

		"Suite/Top/G.Random.Global/G.Random.Global.S.3->G.Random.Global.S.3",

		"Suite/Top/G/G.S->G.Before.1", "Suite/Top/G/G.S->G.Before.2", "Suite/Top/G/G.S->G.Before.3",
		"Suite/Top/G/G.S->G.S",
		"Suite/Top/G/G.S->G.After.1", "Suite/Top/G/G.S->G.After.2", "Suite/Top/G/G.S->G.After.3",

		"Suite/Top/G.Random.Global/G.Random.Global.S.1->G.Random.Global.S.1",

		"Suite/Top/G.Sequential/G.Sequential.S.2->G.Sequential.S.2",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestFlat(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite", spec.Flat(), spec.Seed(2))
	suite("Top", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), optionDefaultOrder(t, "Suite/Top", 2)) {
		t.Fatal("Incorrect order:", calls())
	}
}
