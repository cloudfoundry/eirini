package rfc5424

import . "gopkg.in/check.v1"

var _ = Suite(&MessageTest{})

type MessageTest struct {
}

func (s *MessageTest) TestAddDatum(c *C) {
	m := Message{}
	m.AddDatum("id", "name", "value")
	c.Assert(m, DeepEquals, Message{
		StructuredData: []StructuredData{
			{
				ID: "id",
				Parameters: []SDParam{
					{"name", "value"},
				},
			},
		},
	})

	m.AddDatum("id2", "name", "value")
	c.Assert(m, DeepEquals, Message{
		StructuredData: []StructuredData{
			{
				ID: "id",
				Parameters: []SDParam{
					{"name", "value"},
				},
			},
			{
				ID: "id2",
				Parameters: []SDParam{
					{"name", "value"},
				},
			},
		},
	})

	m.AddDatum("id", "name2", "value2")
	c.Assert(m, DeepEquals, Message{
		StructuredData: []StructuredData{
			{
				ID: "id",
				Parameters: []SDParam{
					{"name", "value"},
					{"name2", "value2"},
				},
			},
			{
				ID: "id2",
				Parameters: []SDParam{
					{"name", "value"},
				},
			},
		},
	})

	m.AddDatum("id", "name", "value3")
	c.Assert(m, DeepEquals, Message{
		StructuredData: []StructuredData{
			{
				ID: "id",
				Parameters: []SDParam{
					{"name", "value"},
					{"name2", "value2"},
					{"name", "value3"},
				},
			},
			{
				ID: "id2",
				Parameters: []SDParam{
					{"name", "value"},
				},
			},
		},
	})
}
