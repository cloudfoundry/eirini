package logs_test

import "github.com/cloudfoundry/dropsonde/log_sender"

type mockLogChainer struct {
	SetTimestampCalled chan bool
	SetTimestampInput  struct {
		T chan int64
	}
	SetTimestampOutput struct {
		Ret0 chan log_sender.LogChainer
	}
	SetTagCalled chan bool
	SetTagInput  struct {
		Key, Value chan string
	}
	SetTagOutput struct {
		Ret0 chan log_sender.LogChainer
	}
	SetAppIdCalled chan bool
	SetAppIdInput  struct {
		Id chan string
	}
	SetAppIdOutput struct {
		Ret0 chan log_sender.LogChainer
	}
	SetSourceTypeCalled chan bool
	SetSourceTypeInput  struct {
		S chan string
	}
	SetSourceTypeOutput struct {
		Ret0 chan log_sender.LogChainer
	}
	SetSourceInstanceCalled chan bool
	SetSourceInstanceInput  struct {
		S chan string
	}
	SetSourceInstanceOutput struct {
		Ret0 chan log_sender.LogChainer
	}
	SendCalled chan bool
	SendOutput struct {
		Ret0 chan error
	}
}

func newMockLogChainer() *mockLogChainer {
	m := &mockLogChainer{}
	m.SetTimestampCalled = make(chan bool, 100)
	m.SetTimestampInput.T = make(chan int64, 100)
	m.SetTimestampOutput.Ret0 = make(chan log_sender.LogChainer, 100)
	m.SetTagCalled = make(chan bool, 100)
	m.SetTagInput.Key = make(chan string, 100)
	m.SetTagInput.Value = make(chan string, 100)
	m.SetTagOutput.Ret0 = make(chan log_sender.LogChainer, 100)
	m.SetAppIdCalled = make(chan bool, 100)
	m.SetAppIdInput.Id = make(chan string, 100)
	m.SetAppIdOutput.Ret0 = make(chan log_sender.LogChainer, 100)
	m.SetSourceTypeCalled = make(chan bool, 100)
	m.SetSourceTypeInput.S = make(chan string, 100)
	m.SetSourceTypeOutput.Ret0 = make(chan log_sender.LogChainer, 100)
	m.SetSourceInstanceCalled = make(chan bool, 100)
	m.SetSourceInstanceInput.S = make(chan string, 100)
	m.SetSourceInstanceOutput.Ret0 = make(chan log_sender.LogChainer, 100)
	m.SendCalled = make(chan bool, 100)
	m.SendOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockLogChainer) SetTimestamp(t int64) log_sender.LogChainer {
	m.SetTimestampCalled <- true
	m.SetTimestampInput.T <- t
	return <-m.SetTimestampOutput.Ret0
}
func (m *mockLogChainer) SetTag(key, value string) log_sender.LogChainer {
	m.SetTagCalled <- true
	m.SetTagInput.Key <- key
	m.SetTagInput.Value <- value
	return <-m.SetTagOutput.Ret0
}
func (m *mockLogChainer) SetAppId(id string) log_sender.LogChainer {
	m.SetAppIdCalled <- true
	m.SetAppIdInput.Id <- id
	return <-m.SetAppIdOutput.Ret0
}
func (m *mockLogChainer) SetSourceType(s string) log_sender.LogChainer {
	m.SetSourceTypeCalled <- true
	m.SetSourceTypeInput.S <- s
	return <-m.SetSourceTypeOutput.Ret0
}
func (m *mockLogChainer) SetSourceInstance(s string) log_sender.LogChainer {
	m.SetSourceInstanceCalled <- true
	m.SetSourceInstanceInput.S <- s
	return <-m.SetSourceInstanceOutput.Ret0
}
func (m *mockLogChainer) Send() error {
	m.SendCalled <- true
	return <-m.SendOutput.Ret0
}
