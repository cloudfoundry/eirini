// Code generated by counterfeiter. DO NOT EDIT.
package handlerfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/models/cf"
)

type FakeStagingBifrost struct {
	CompleteStagingStub        func(context.Context, cf.StagingCompletedRequest) error
	completeStagingMutex       sync.RWMutex
	completeStagingArgsForCall []struct {
		arg1 context.Context
		arg2 cf.StagingCompletedRequest
	}
	completeStagingReturns struct {
		result1 error
	}
	completeStagingReturnsOnCall map[int]struct {
		result1 error
	}
	TransferStagingStub        func(context.Context, string, cf.StagingRequest) error
	transferStagingMutex       sync.RWMutex
	transferStagingArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 cf.StagingRequest
	}
	transferStagingReturns struct {
		result1 error
	}
	transferStagingReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeStagingBifrost) CompleteStaging(arg1 context.Context, arg2 cf.StagingCompletedRequest) error {
	fake.completeStagingMutex.Lock()
	ret, specificReturn := fake.completeStagingReturnsOnCall[len(fake.completeStagingArgsForCall)]
	fake.completeStagingArgsForCall = append(fake.completeStagingArgsForCall, struct {
		arg1 context.Context
		arg2 cf.StagingCompletedRequest
	}{arg1, arg2})
	stub := fake.CompleteStagingStub
	fakeReturns := fake.completeStagingReturns
	fake.recordInvocation("CompleteStaging", []interface{}{arg1, arg2})
	fake.completeStagingMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeStagingBifrost) CompleteStagingCallCount() int {
	fake.completeStagingMutex.RLock()
	defer fake.completeStagingMutex.RUnlock()
	return len(fake.completeStagingArgsForCall)
}

func (fake *FakeStagingBifrost) CompleteStagingCalls(stub func(context.Context, cf.StagingCompletedRequest) error) {
	fake.completeStagingMutex.Lock()
	defer fake.completeStagingMutex.Unlock()
	fake.CompleteStagingStub = stub
}

func (fake *FakeStagingBifrost) CompleteStagingArgsForCall(i int) (context.Context, cf.StagingCompletedRequest) {
	fake.completeStagingMutex.RLock()
	defer fake.completeStagingMutex.RUnlock()
	argsForCall := fake.completeStagingArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeStagingBifrost) CompleteStagingReturns(result1 error) {
	fake.completeStagingMutex.Lock()
	defer fake.completeStagingMutex.Unlock()
	fake.CompleteStagingStub = nil
	fake.completeStagingReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeStagingBifrost) CompleteStagingReturnsOnCall(i int, result1 error) {
	fake.completeStagingMutex.Lock()
	defer fake.completeStagingMutex.Unlock()
	fake.CompleteStagingStub = nil
	if fake.completeStagingReturnsOnCall == nil {
		fake.completeStagingReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.completeStagingReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeStagingBifrost) TransferStaging(arg1 context.Context, arg2 string, arg3 cf.StagingRequest) error {
	fake.transferStagingMutex.Lock()
	ret, specificReturn := fake.transferStagingReturnsOnCall[len(fake.transferStagingArgsForCall)]
	fake.transferStagingArgsForCall = append(fake.transferStagingArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 cf.StagingRequest
	}{arg1, arg2, arg3})
	stub := fake.TransferStagingStub
	fakeReturns := fake.transferStagingReturns
	fake.recordInvocation("TransferStaging", []interface{}{arg1, arg2, arg3})
	fake.transferStagingMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeStagingBifrost) TransferStagingCallCount() int {
	fake.transferStagingMutex.RLock()
	defer fake.transferStagingMutex.RUnlock()
	return len(fake.transferStagingArgsForCall)
}

func (fake *FakeStagingBifrost) TransferStagingCalls(stub func(context.Context, string, cf.StagingRequest) error) {
	fake.transferStagingMutex.Lock()
	defer fake.transferStagingMutex.Unlock()
	fake.TransferStagingStub = stub
}

func (fake *FakeStagingBifrost) TransferStagingArgsForCall(i int) (context.Context, string, cf.StagingRequest) {
	fake.transferStagingMutex.RLock()
	defer fake.transferStagingMutex.RUnlock()
	argsForCall := fake.transferStagingArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeStagingBifrost) TransferStagingReturns(result1 error) {
	fake.transferStagingMutex.Lock()
	defer fake.transferStagingMutex.Unlock()
	fake.TransferStagingStub = nil
	fake.transferStagingReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeStagingBifrost) TransferStagingReturnsOnCall(i int, result1 error) {
	fake.transferStagingMutex.Lock()
	defer fake.transferStagingMutex.Unlock()
	fake.TransferStagingStub = nil
	if fake.transferStagingReturnsOnCall == nil {
		fake.transferStagingReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.transferStagingReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeStagingBifrost) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.completeStagingMutex.RLock()
	defer fake.completeStagingMutex.RUnlock()
	fake.transferStagingMutex.RLock()
	defer fake.transferStagingMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeStagingBifrost) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ handler.StagingBifrost = new(FakeStagingBifrost)
