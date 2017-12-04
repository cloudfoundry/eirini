package log_sender_test

import (
	"errors"
	"io"
	"reflect"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/sonde-go/events"
)

type fakeReader struct {
	counter int
}

func (f *fakeReader) Read(p []byte) (int, error) {
	f.counter++

	switch f.counter {
	case 1: // message
		return copy(p, "one\n"), nil
	case 2: // read error
		return 0, errors.New("Read Error")
	case 3: // message
		return copy(p, "two\n"), nil
	default: // eof
		return 0, io.EOF
	}
}

type infiniteReader struct {
	stopChan chan struct{}
}

func (i infiniteReader) Read(p []byte) (int, error) {
	select {
	case <-i.stopChan:
		return 0, io.EOF
	default:
	}

	return copy(p, "hello\n"), nil
}

func getLogMessages(messages []fake.Message) []string {
	var logMessages []string
	for _, msg := range messages {
		log, ok := msg.Event.(*events.LogMessage)
		if ok {
			logMessages = append(logMessages, string(log.GetMessage()))
		}
	}
	return logMessages
}

func keepMockChansDrained(stop chan struct{}, called chan bool, inputs interface{}) {
	for {
		select {
		case <-stop:
			return
		case <-called:
			drainInputs(inputs)
		}
	}
}

func drainInputs(inputs interface{}) {
	v := reflect.ValueOf(inputs)
	for i := 0; i < v.NumField(); i++ {
		v.Field(i).Recv()
	}
}
