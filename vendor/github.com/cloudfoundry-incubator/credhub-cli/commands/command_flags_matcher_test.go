package commands

import (
	"reflect"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

func HaveFlag(long, short string) types.GomegaMatcher {
	return &CommandFlagMatcher{
		Long:  long,
		Short: short,
	}
}

type CommandFlagMatcher struct {
	Long  string
	Short string
}

func (matcher *CommandFlagMatcher) Match(actual interface{}) (success bool, err error) {
	t := reflect.TypeOf(actual)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		short := field.Tag.Get("short")
		long := field.Tag.Get("long")
		if long == matcher.Long {
			return short == matcher.Short, nil
		}
	}
	return false, nil
}

func (matcher *CommandFlagMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to have command with flags", matcher)
}

func (matcher *CommandFlagMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to have command with flags", matcher)
}
