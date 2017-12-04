package metricbatcher_test

import "github.com/cloudfoundry/dropsonde/metric_sender"

type mockMetricSender struct {
	CounterCalled chan bool
	CounterInput  struct {
		Name chan string
	}
	CounterOutput struct {
		Ret0 chan metric_sender.CounterChainer
	}
}

func newMockMetricSender() *mockMetricSender {
	m := &mockMetricSender{}
	m.CounterCalled = make(chan bool, 100)
	m.CounterInput.Name = make(chan string, 100)
	m.CounterOutput.Ret0 = make(chan metric_sender.CounterChainer, 100)
	return m
}
func (m *mockMetricSender) Counter(name string) metric_sender.CounterChainer {
	m.CounterCalled <- true
	m.CounterInput.Name <- name
	return <-m.CounterOutput.Ret0
}
