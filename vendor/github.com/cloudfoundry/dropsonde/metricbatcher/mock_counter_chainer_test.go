package metricbatcher_test

import "github.com/cloudfoundry/dropsonde/metric_sender"

type mockCounterChainer struct {
	SetTagCalled chan bool
	SetTagInput  struct {
		Key, Value chan string
	}
	SetTagOutput struct {
		Ret0 chan metric_sender.CounterChainer
	}
	IncrementCalled chan bool
	IncrementOutput struct {
		Ret0 chan error
	}
	AddCalled chan bool
	AddInput  struct {
		Delta chan uint64
	}
	AddOutput struct {
		Ret0 chan error
	}
}

func newMockCounterChainer() *mockCounterChainer {
	m := &mockCounterChainer{}
	m.SetTagCalled = make(chan bool, 100)
	m.SetTagInput.Key = make(chan string, 100)
	m.SetTagInput.Value = make(chan string, 100)
	m.SetTagOutput.Ret0 = make(chan metric_sender.CounterChainer, 100)
	m.IncrementCalled = make(chan bool, 100)
	m.IncrementOutput.Ret0 = make(chan error, 100)
	m.AddCalled = make(chan bool, 100)
	m.AddInput.Delta = make(chan uint64, 100)
	m.AddOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockCounterChainer) SetTag(key, value string) metric_sender.CounterChainer {
	m.SetTagCalled <- true
	m.SetTagInput.Key <- key
	m.SetTagInput.Value <- value
	return <-m.SetTagOutput.Ret0
}
func (m *mockCounterChainer) Increment() error {
	m.IncrementCalled <- true
	return <-m.IncrementOutput.Ret0
}
func (m *mockCounterChainer) Add(delta uint64) error {
	m.AddCalled <- true
	m.AddInput.Delta <- delta
	return <-m.AddOutput.Ret0
}
