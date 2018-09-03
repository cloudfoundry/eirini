package spec_test

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/sclevine/spec"
	"fmt"
	"io/ioutil"
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
	for i, call := range results {
		results[i] = regexp.MustCompile(name+`#[0-9]+\/`).ReplaceAllLiteralString(call, name+"/")
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

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		when("G.Out", func() {
			it("Out.1", func() {
				fmt.Fprint(it.Out(), "Out.1")
			})
			it("Out.2", func() {
				fmt.Fprint(it.Out(), "Out.2")
			})
		}, spec.Reverse())
		optionTestCases(t, when, it, s)
	}, spec.Report(reporter), spec.Seed(2))

	if !reflect.DeepEqual(calls(), optionDefaultOrder(t, "Run", 2)) {
		t.Fatal("Incorrect order:", calls())
	}
	if reporter.StartT != t {
		t.Fatal("Incorrect value for t on start.")
	}
	if reporter.SpecsT != t {
		t.Fatal("Incorrect value for t on spec run.")
	}
	if reporter.StartPlan != (spec.Plan{
		Text:      "Run",
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
	if string(out2) != "Out.2" || err != nil {
		t.Fatal("Incorrect output for Out.2 buffer.")
	}
	out1, err := ioutil.ReadAll(reporter.SpecOrder[1].Out)
	if string(out1) != "Out.1" || err != nil {
		t.Fatal("Incorrect output for Out.1 buffer.")
	}
	empty, err := ioutil.ReadAll(reporter.SpecOrder[2].Out)
	if string(empty) != "" || err != nil {
		t.Fatal("Incorrect output for empty buffer.")
	}
	for i := range reporter.SpecOrder {
		reporter.SpecOrder[i].Out = nil
	}

	if !reflect.DeepEqual(reporter.SpecOrder, []spec.Spec{
		{Text: []string{"G.Out", "Out.2"}}, {Text: []string{"G.Out", "Out.1"}},
		{Text: []string{"S.1"}}, {Text: []string{"S.2"}}, {Text: []string{"S.3"}},
		{Text: []string{"G", "G.S"}},
		{Text: []string{"G.Sequential", "G.Sequential.S.1"}},
		{Text: []string{"G.Sequential", "G.Sequential.S.2"}},
		{Text: []string{"G.Sequential", "G.Sequential.S.3"}},
		{Text: []string{"G.Reverse", "G.Reverse.S.3"}},
		{Text: []string{"G.Reverse", "G.Reverse.S.2"}},
		{Text: []string{"G.Reverse", "G.Reverse.S.1"}},
		{Text: []string{"G.Random.Local", "G.Random.Local.S.3"}},
		{Text: []string{"G.Random.Local", "G.Random.Local.S.1"}},
		{Text: []string{"G.Random.Local", "G.Random.Local.S.2"}},
		{Text: []string{"G.Random.Global", "G.Random.Global.S.3"}},
		{Text: []string{"G.Random.Global", "G.Random.Global.S.1"}},
		{Text: []string{"G.Random.Global", "G.Random.Global.S.2"}},
	}) {
		t.Fatal("Incorrect reported order:", reporter.SpecOrder)
	}
}

func TestDefault(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	}, spec.Seed(2))

	if !reflect.DeepEqual(calls(), []string{
		"Run/S.1->S.1", "Run/S.2->S.2", "Run/S.3->S.3",

		"Run/G/G.S->G.Before.1", "Run/G/G.S->G.Before.2", "Run/G/G.S->G.Before.3",
		"Run/G/G.S->G.S",
		"Run/G/G.S->G.After.1", "Run/G/G.S->G.After.2", "Run/G/G.S->G.After.3",

		"Run/G.Sequential/G.Sequential.S.1->G.Sequential.S.1",
		"Run/G.Sequential/G.Sequential.S.2->G.Sequential.S.2",
		"Run/G.Sequential/G.Sequential.S.3->G.Sequential.S.3",

		"Run/G.Reverse/G.Reverse.S.3->G.Reverse.S.3",
		"Run/G.Reverse/G.Reverse.S.2->G.Reverse.S.2",
		"Run/G.Reverse/G.Reverse.S.1->G.Reverse.S.1",

		"Run/G.Random.Local/G.Random.Local.S.3->G.Random.Local.S.3",
		"Run/G.Random.Local/G.Random.Local.S.1->G.Random.Local.S.1",
		"Run/G.Random.Local/G.Random.Local.S.2->G.Random.Local.S.2",

		"Run/G.Random.Global/G.Random.Global.S.3->G.Random.Global.S.3",
		"Run/G.Random.Global/G.Random.Global.S.1->G.Random.Global.S.1",
		"Run/G.Random.Global/G.Random.Global.S.2->G.Random.Global.S.2",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestSequential(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	}, spec.Sequential(), spec.Seed(2))

	if !reflect.DeepEqual(calls(), optionDefaultOrder(t, "Run", 2)) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestRandom(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	}, spec.Random(), spec.Seed(2))

	if !reflect.DeepEqual(calls(), []string{
		"Run/G.Random.Local/G.Random.Local.S.3->G.Random.Local.S.3",
		"Run/G.Random.Local/G.Random.Local.S.1->G.Random.Local.S.1",
		"Run/G.Random.Local/G.Random.Local.S.2->G.Random.Local.S.2",

		"Run/G/G.S->G.Before.1", "Run/G/G.S->G.Before.2", "Run/G/G.S->G.Before.3",
		"Run/G/G.S->G.S",
		"Run/G/G.S->G.After.1", "Run/G/G.S->G.After.2", "Run/G/G.S->G.After.3",

		"Run/G.Random.Global/G.Random.Global.S.3->G.Random.Global.S.3",
		"Run/G.Random.Global/G.Random.Global.S.1->G.Random.Global.S.1",
		"Run/G.Random.Global/G.Random.Global.S.2->G.Random.Global.S.2",

		"Run/G.Sequential/G.Sequential.S.1->G.Sequential.S.1",
		"Run/G.Sequential/G.Sequential.S.2->G.Sequential.S.2",
		"Run/G.Sequential/G.Sequential.S.3->G.Sequential.S.3",

		"Run/G.Reverse/G.Reverse.S.3->G.Reverse.S.3",
		"Run/G.Reverse/G.Reverse.S.2->G.Reverse.S.2",
		"Run/G.Reverse/G.Reverse.S.1->G.Reverse.S.1",

		"Run/S.1->S.1", "Run/S.2->S.2", "Run/S.3->S.3",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestReverse(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	}, spec.Reverse(), spec.Seed(2))

	if !reflect.DeepEqual(calls(), []string{
		"Run/G.Random.Global/G.Random.Global.S.3->G.Random.Global.S.3",
		"Run/G.Random.Global/G.Random.Global.S.1->G.Random.Global.S.1",
		"Run/G.Random.Global/G.Random.Global.S.2->G.Random.Global.S.2",

		"Run/G.Random.Local/G.Random.Local.S.3->G.Random.Local.S.3",
		"Run/G.Random.Local/G.Random.Local.S.1->G.Random.Local.S.1",
		"Run/G.Random.Local/G.Random.Local.S.2->G.Random.Local.S.2",

		"Run/G.Reverse/G.Reverse.S.3->G.Reverse.S.3",
		"Run/G.Reverse/G.Reverse.S.2->G.Reverse.S.2",
		"Run/G.Reverse/G.Reverse.S.1->G.Reverse.S.1",

		"Run/G.Sequential/G.Sequential.S.1->G.Sequential.S.1",
		"Run/G.Sequential/G.Sequential.S.2->G.Sequential.S.2",
		"Run/G.Sequential/G.Sequential.S.3->G.Sequential.S.3",

		"Run/G/G.S->G.Before.1", "Run/G/G.S->G.Before.2", "Run/G/G.S->G.Before.3",
		"Run/G/G.S->G.S",
		"Run/G/G.S->G.After.1", "Run/G/G.S->G.After.2", "Run/G/G.S->G.After.3",

		"Run/S.3->S.3", "Run/S.2->S.2", "Run/S.1->S.1",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestLocal(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	}, spec.Local(), spec.Seed(2))

	if !reflect.DeepEqual(calls(), optionDefaultOrder(t, "Run", 2)) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestGlobal(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	}, spec.Random(), spec.Seed(2), spec.Global())

	if !reflect.DeepEqual(calls(), []string{
		"Run/G/G.S->G.Before.1", "Run/G/G.S->G.Before.2", "Run/G/G.S->G.Before.3",
		"Run/G/G.S->G.S",
		"Run/G/G.S->G.After.1", "Run/G/G.S->G.After.2", "Run/G/G.S->G.After.3",

		"Run/G.Reverse/G.Reverse.S.1->G.Reverse.S.1",

		"Run/G.Random.Global/G.Random.Global.S.3->G.Random.Global.S.3",

		"Run/G.Reverse/G.Reverse.S.3->G.Reverse.S.3",

		"Run/G.Sequential/G.Sequential.S.2->G.Sequential.S.2",

		"Run/G.Sequential/G.Sequential.S.3->G.Sequential.S.3",

		"Run/S.2->S.2",

		"Run/G.Random.Local/G.Random.Local.S.3->G.Random.Local.S.3",
		"Run/G.Random.Local/G.Random.Local.S.1->G.Random.Local.S.1",
		"Run/G.Random.Local/G.Random.Local.S.2->G.Random.Local.S.2",

		"Run/G.Reverse/G.Reverse.S.2->G.Reverse.S.2",

		"Run/G.Random.Global/G.Random.Global.S.2->G.Random.Global.S.2",

		"Run/S.3->S.3",

		"Run/S.1->S.1",

		"Run/G.Random.Global/G.Random.Global.S.1->G.Random.Global.S.1",

		"Run/G.Sequential/G.Sequential.S.1->G.Sequential.S.1",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestFlat(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		optionTestCases(t, when, it, s)
	}, spec.Flat(), spec.Seed(2))

	if !reflect.DeepEqual(calls(), optionDefaultOrder(t, "Run", 2)) {
		t.Fatal("Incorrect order:", calls())
	}
}
