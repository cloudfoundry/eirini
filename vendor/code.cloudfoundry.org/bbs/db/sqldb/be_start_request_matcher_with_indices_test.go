package sqldb_test

import (
	"fmt"
	"reflect"
	"sort"

	"code.cloudfoundry.org/auctioneer"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func BeActualLRPStartRequest(lrpStartRequest auctioneer.LRPStartRequest) gomega.OmegaMatcher {
	sort.Ints(lrpStartRequest.Indices)
	return &BeActualLRPStartRequestMatcher{
		request: lrpStartRequest,
	}
}

type BeActualLRPStartRequestMatcher struct {
	request auctioneer.LRPStartRequest
}

func (matcher *BeActualLRPStartRequestMatcher) Match(actual interface{}) (success bool, err error) {
	lrp, ok := actual.(*auctioneer.LRPStartRequest)
	if !ok {
		return false, fmt.Errorf("BeActualLRP matcher expects a auctioneer.LRPStartRequest.  Got:\n%s", format.Object(actual, 1))
	}
	sort.Ints(lrp.Indices)
	if !reflect.DeepEqual(*lrp, matcher.request) {
		return false, nil
	}

	return true, nil
}

func (matcher *BeActualLRPStartRequestMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto have:\n%#v", format.Object(actual, 1), matcher.request)
}

func (matcher *BeActualLRPStartRequestMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to have:\n%#v", format.Object(actual, 1), matcher.request)
}
