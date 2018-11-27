package spec_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

type recorder func(*testing.T, string) func()

func record(t *testing.T) (s recorder, c func() []string) {
	var calls []string
	return func(ts *testing.T, id string) func() {
			return func() {
				if ts == nil {
					t.Fatal("Spec running during parse phase for:", t.Name())
				}
				name := strings.TrimPrefix(ts.Name(), t.Name()+"/") + "->" + id
				calls = append(calls, name)
			}
		}, func() []string {
			return calls
		}
}

func specTestCases(t *testing.T, when spec.G, it spec.S, s recorder) {
	it.Before(s(t, "Before"))
	it.After(s(t, "After"))

	it("S", s(t, "S"))
	it.Pend("S.Pend", s(t, "S.Pend"))
	it.Focus("S.Focus", s(t, "S.Focus"))

	when("G", func() {
		it.Before(s(t, "G.Before"))
		it.After(s(t, "G.After"))
		it("G.S", s(t, "G.S"))
	})
	when.Pend("G.Pend", func() {
		it.Before(s(t, "G.Pend.Before"))
		it.After(s(t, "G.Pend.After"))
		it("G.Pend.S", s(t, "G.Pend.S"))
	})
	when.Focus("G.Focus", func() {
		it.Before(s(t, "G.Focus.Before"))
		it.After(s(t, "G.Focus.After"))
		it("G.Focus.S", s(t, "G.Focus.S"))
	})
}

func TestRun(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		specTestCases(t, when, it, s)
	})

	if !reflect.DeepEqual(calls(), []string{
		"Run/S.Focus->Before",
		"Run/S.Focus->S.Focus",
		"Run/S.Focus->After",

		"Run/G.Focus/G.Focus.S->Before", "Run/G.Focus/G.Focus.S->G.Focus.Before",
		"Run/G.Focus/G.Focus.S->G.Focus.S",
		"Run/G.Focus/G.Focus.S->G.Focus.After", "Run/G.Focus/G.Focus.S->After",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestSuiteRun(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite")
	suite("Top.1", func(t *testing.T, when spec.G, it spec.S) {
		specTestCases(t, when, it, s)
	})
	suite("Top.2", func(t *testing.T, when spec.G, it spec.S) {
		it.Focus("Top.2.S.Focus", s(t, "Top.2.S.Focus"))
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), []string{
		"Suite/Top.1/S.Focus->Before",
		"Suite/Top.1/S.Focus->S.Focus",
		"Suite/Top.1/S.Focus->After",

		"Suite/Top.1/G.Focus/G.Focus.S->Before", "Suite/Top.1/G.Focus/G.Focus.S->G.Focus.Before",
		"Suite/Top.1/G.Focus/G.Focus.S->G.Focus.S",
		"Suite/Top.1/G.Focus/G.Focus.S->G.Focus.After", "Suite/Top.1/G.Focus/G.Focus.S->After",

		"Suite/Top.2/Top.2.S.Focus->Top.2.S.Focus",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestPend(t *testing.T) {
	s, calls := record(t)

	spec.Pend(t, "Pend", func(t *testing.T, when spec.G, it spec.S) {
		specTestCases(t, when, it, s)
	})

	if len(calls()) != 0 {
		t.Fatal("Failed to pend:", calls())
	}
}

func TestSuitePend(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite")
	suite.Pend("Top.Pend", func(t *testing.T, when spec.G, it spec.S) {
		specTestCases(t, when, it, s)
	})
	suite.Run(t)

	if len(calls()) != 0 {
		t.Fatal("Failed to pend:", calls())
	}
}

func TestGPend(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		when.Pend("Run.G.Pend", func() {
			specTestCases(t, when, it, s)
		})
	})

	if len(calls()) != 0 {
		t.Fatal("Failed to pend:", calls())
	}
}

func TestSPend(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		it.Pend("Run.S", s(t, "Run.S"))
	})

	if len(calls()) != 0 {
		t.Fatal("Failed to pend:", calls())
	}
}

func TestFocus(t *testing.T) {
	s, calls := record(t)

	spec.Focus(t, "Focus", func(t *testing.T, when spec.G, it spec.S) {
		specTestCases(t, when, it, s)
	})

	if !reflect.DeepEqual(calls(), []string{
		"Focus/S->Before",
		"Focus/S->S",
		"Focus/S->After",

		"Focus/S.Focus->Before",
		"Focus/S.Focus->S.Focus",
		"Focus/S.Focus->After",

		"Focus/G/G.S->Before", "Focus/G/G.S->G.Before",
		"Focus/G/G.S->G.S",
		"Focus/G/G.S->G.After", "Focus/G/G.S->After",

		"Focus/G.Focus/G.Focus.S->Before", "Focus/G.Focus/G.Focus.S->G.Focus.Before",
		"Focus/G.Focus/G.Focus.S->G.Focus.S",
		"Focus/G.Focus/G.Focus.S->G.Focus.After", "Focus/G.Focus/G.Focus.S->After",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestSuiteFocus(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite")
	suite.Focus("Top.Focus", func(t *testing.T, when spec.G, it spec.S) {
		specTestCases(t, when, it, s)
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), []string{
		"Suite/Top.Focus/S->Before",
		"Suite/Top.Focus/S->S",
		"Suite/Top.Focus/S->After",

		"Suite/Top.Focus/S.Focus->Before",
		"Suite/Top.Focus/S.Focus->S.Focus",
		"Suite/Top.Focus/S.Focus->After",

		"Suite/Top.Focus/G/G.S->Before", "Suite/Top.Focus/G/G.S->G.Before",
		"Suite/Top.Focus/G/G.S->G.S",
		"Suite/Top.Focus/G/G.S->G.After", "Suite/Top.Focus/G/G.S->After",

		"Suite/Top.Focus/G.Focus/G.Focus.S->Before", "Suite/Top.Focus/G.Focus/G.Focus.S->G.Focus.Before",
		"Suite/Top.Focus/G.Focus/G.Focus.S->G.Focus.S",
		"Suite/Top.Focus/G.Focus/G.Focus.S->G.Focus.After", "Suite/Top.Focus/G.Focus/G.Focus.S->After",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestGFocus(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		when.Focus("Run.G.Focus", func() {
			specTestCases(t, when, it, s)
		})
	})

	if !reflect.DeepEqual(calls(), []string{
		"Run/Run.G.Focus/S->Before",
		"Run/Run.G.Focus/S->S",
		"Run/Run.G.Focus/S->After",

		"Run/Run.G.Focus/S.Focus->Before",
		"Run/Run.G.Focus/S.Focus->S.Focus",
		"Run/Run.G.Focus/S.Focus->After",

		"Run/Run.G.Focus/G/G.S->Before", "Run/Run.G.Focus/G/G.S->G.Before",
		"Run/Run.G.Focus/G/G.S->G.S",
		"Run/Run.G.Focus/G/G.S->G.After", "Run/Run.G.Focus/G/G.S->After",

		"Run/Run.G.Focus/G.Focus/G.Focus.S->Before", "Run/Run.G.Focus/G.Focus/G.Focus.S->G.Focus.Before",
		"Run/Run.G.Focus/G.Focus/G.Focus.S->G.Focus.S",
		"Run/Run.G.Focus/G.Focus/G.Focus.S->G.Focus.After", "Run/Run.G.Focus/G.Focus/G.Focus.S->After",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestSFocus(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		it.Focus("Run.S.Focus", s(t, "Run.S.Focus"))
	})

	if !reflect.DeepEqual(calls(), []string{
		"Run/Run.S.Focus->Run.S.Focus",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestSuiteBefore(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite")
	suite.Before(func(t *testing.T) {
		s(t, "Before.1")()
	})
	suite("Top", func(t *testing.T, when spec.G, it spec.S) {
		it.Before(s(t, "Top.Before"))
		when("Top.G", func() {
			it.Before(s(t, "Top.G.Before"))
			it("Top.G.S", s(t, "Top.G.S"))
		})
	})
	suite.Before(func(t *testing.T) {
		s(t, "Before.2")()
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), []string{
		"Suite/Top/Top.G/Top.G.S->Before.1", "Suite/Top/Top.G/Top.G.S->Before.2",
		"Suite/Top/Top.G/Top.G.S->Top.Before",
		"Suite/Top/Top.G/Top.G.S->Top.G.Before",
		"Suite/Top/Top.G/Top.G.S->Top.G.S",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestSBefore(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		it.Before(s(t, "Run.Before.1"))
		it("Run.S", s(t, "Run.S"))
		it.Before(s(t, "Run.Before.2"))
		when("Run.G", func() {
			it.Before(s(t, "Run.G.Before.1"))
			it("Run.G.S", s(t, "Run.G.S"))
			it.Before(s(t, "Run.G.Before.2"))
		})
		it.Before(s(t, "Run.Before.3"))
	})

	if !reflect.DeepEqual(calls(), []string{
		"Run/Run.S->Run.Before.1", "Run/Run.S->Run.Before.2", "Run/Run.S->Run.Before.3",
		"Run/Run.S->Run.S",

		"Run/Run.G/Run.G.S->Run.Before.1", "Run/Run.G/Run.G.S->Run.Before.2", "Run/Run.G/Run.G.S->Run.Before.3",
		"Run/Run.G/Run.G.S->Run.G.Before.1", "Run/Run.G/Run.G.S->Run.G.Before.2",
		"Run/Run.G/Run.G.S->Run.G.S",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestSAfter(t *testing.T) {
	s, calls := record(t)

	spec.Run(t, "Run", func(t *testing.T, when spec.G, it spec.S) {
		it.After(s(t, "Run.After.1"))
		it("Run.S", s(t, "Run.S"))
		it.After(s(t, "Run.After.2"))
		when("Run.G", func() {
			it.After(s(t, "Run.G.After.1"))
			it("Run.G.S", s(t, "Run.G.S"))
			it.After(s(t, "Run.G.After.2"))
		})
		it.After(s(t, "Run.After.3"))
	})

	if !reflect.DeepEqual(calls(), []string{
		"Run/Run.S->Run.S",
		"Run/Run.S->Run.After.1", "Run/Run.S->Run.After.2", "Run/Run.S->Run.After.3",

		"Run/Run.G/Run.G.S->Run.G.S",
		"Run/Run.G/Run.G.S->Run.G.After.1", "Run/Run.G/Run.G.S->Run.G.After.2",
		"Run/Run.G/Run.G.S->Run.After.1", "Run/Run.G/Run.G.S->Run.After.2", "Run/Run.G/Run.G.S->Run.After.3",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestSuiteAfter(t *testing.T) {
	s, calls := record(t)

	suite := spec.New("Suite")
	suite.After(func(t *testing.T) {
		s(t, "After.1")()
	})
	suite("Top", func(t *testing.T, when spec.G, it spec.S) {
		it.After(s(t, "Top.After"))
		when("Top.G", func() {
			it.After(s(t, "Top.G.After"))
			it("Top.G.S", s(t, "Top.G.S"))
		})
	})
	suite.After(func(t *testing.T) {
		s(t, "After.2")()
	})
	suite.Run(t)

	if !reflect.DeepEqual(calls(), []string{
		"Suite/Top/Top.G/Top.G.S->Top.G.S",
		"Suite/Top/Top.G/Top.G.S->Top.G.After",
		"Suite/Top/Top.G/Top.G.S->Top.After",
		"Suite/Top/Top.G/Top.G.S->After.1", "Suite/Top/Top.G/Top.G.S->After.2",
	}) {
		t.Fatal("Incorrect order:", calls())
	}
}

func TestSpec(t *testing.T) {
	spec.Run(t, "spec", func(t *testing.T, when spec.G, it spec.S) {
		when("something happens", func() {
			var someStr string

			it.Before(func() {
				t.Log("before")
				if someStr == "some-value" {
					t.Fatal("test pollution")
				}
				someStr = "some-value"
			})

			it.After(func() {
				t.Log("after")
			})

			it("should do something", func() {
				t.Log("first")
			})

			when("something else also happens", func() {
				it.Before(func() {
					t.Log("nested before")
				})

				it("should do something nested", func() {
					t.Log("second")
				})

				it.After(func() {
					t.Log("nested after")
				})
			})

			when("some things happen in parallel at the end", func() {
				it.After(func() {
					t.Log("lone after")
				})

				it("should do one thing in parallel", func() {
					t.Log("first parallel")
				})

				it("should do another thing in parallel", func() {
					t.Log("second parallel")
				})
			}, spec.Parallel())

			when("some things happen randomly", func() {
				it.Before(func() {
					t.Log("before random")
				})

				it("should do one thing randomly", func() {
					t.Log("first random")
				})

				it("should do another thing randomly", func() {
					t.Log("second random")
				})
			}, spec.Random())

			when("some things happen in reverse and in nested subtests", func() {
				it.Before(func() {
					t.Log("before reverse")
				})

				it("should do another thing second", func() {
					t.Log("second reverse")
				})

				it("should do one thing first", func() {
					t.Log("first reverse")
				})
			}, spec.Reverse(), spec.Nested())

			when("some things happen in globally random order", func() {
				it.Before(func() {
					t.Log("before global")
				})

				when("grouped first", func() {
					it.Before(func() {
						t.Log("before group one global")
					})

					it("should do one thing in group one randomly", func() {
						t.Log("group one, spec one, global random")
					})

					it("should do another thing in group one randomly", func() {
						t.Log("group one, spec two, global random")
					})
				})

				when("grouped second", func() {
					it.Before(func() {
						t.Log("before group two global")
					})

					it("should do one thing in group two randomly", func() {
						t.Log("group two, spec one, global random")
					})

					it("should do another thing in group two randomly", func() {
						t.Log("group two, spec two, global random")
					})
				}, spec.Local())

				it("should do one thing ungrouped", func() {
					t.Log("ungrouped global random")
				})
			}, spec.Random(), spec.Global())

			it("should do something else", func() {
				t.Log("third")
			})

			it.Pend("should not do this", func() {
				t.Log("forth")
			})

			when.Pend("nothing important happens", func() {
				it.Focus("should not really focus on this", func() {
					t.Log("fifth")
				})
			})
		})
	}, spec.Report(report.Terminal{}))
}
