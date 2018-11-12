package rfc5424

import (
	"bytes"

	. "gopkg.in/check.v1"
)

var _ = Suite(&StreamTest{})

type StreamTest struct {
}

func (s *StreamTest) TestCanReadAndWrite(c *C) {
	stream := bytes.Buffer{}
	for i := 0; i < 4; i++ {
		m := Message{Priority: Priority(i), Timestamp: T("2003-08-24T05:14:15.000003-07:00")}
		nbytes, err := m.WriteTo(&stream)
		c.Assert(err, IsNil)
		c.Assert(nbytes, Equals, int64(50))
	}

	c.Assert(string(stream.Bytes()), Equals,
		`47 <0>1 2003-08-24T05:14:15.000003-07:00 - - - - -`+
			`47 <1>1 2003-08-24T05:14:15.000003-07:00 - - - - -`+
			`47 <2>1 2003-08-24T05:14:15.000003-07:00 - - - - -`+
			`47 <3>1 2003-08-24T05:14:15.000003-07:00 - - - - -`)

	for i := 0; i < 4; i++ {
		m := Message{Priority: Priority(i << 3)}
		nbytes, err := m.ReadFrom(&stream)
		c.Assert(err, IsNil)
		c.Assert(nbytes, Equals, int64(50))
		c.Assert(m, DeepEquals, Message{Priority: Priority(i),
			Timestamp:      T("2003-08-24T05:14:15.000003-07:00"),
			StructuredData: []StructuredData{}})
	}
}

func (s *StreamTest) TestRejectsInvalidStream(c *C) {
	stream := bytes.NewBufferString(`99 <0>1 2003-08-24T05:14:15.000003-07:00 - - - - -`)
	for i := 0; i < 4; i++ {
		m := Message{Priority: Priority(i << 3)}
		_, err := m.ReadFrom(stream)
		c.Assert(err, Not(IsNil))
	}
}

func (s *StreamTest) TestRejectsInvalidStream2(c *C) {
	stream := bytes.NewBufferString(`0 <0>1 2003-08-24T05:14:15.000003-07:00 - - - - -`)
	for i := 0; i < 4; i++ {
		m := Message{Priority: Priority(i << 3)}
		_, err := m.ReadFrom(stream)
		c.Assert(err, Not(IsNil))
	}
}
