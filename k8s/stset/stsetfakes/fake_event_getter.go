// Code generated by counterfeiter. DO NOT EDIT.
package stsetfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/k8s/stset"
	v1 "k8s.io/api/core/v1"
)

type FakeEventGetter struct {
	GetByPodStub        func(context.Context, v1.Pod) ([]v1.Event, error)
	getByPodMutex       sync.RWMutex
	getByPodArgsForCall []struct {
		arg1 context.Context
		arg2 v1.Pod
	}
	getByPodReturns struct {
		result1 []v1.Event
		result2 error
	}
	getByPodReturnsOnCall map[int]struct {
		result1 []v1.Event
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeEventGetter) GetByPod(arg1 context.Context, arg2 v1.Pod) ([]v1.Event, error) {
	fake.getByPodMutex.Lock()
	ret, specificReturn := fake.getByPodReturnsOnCall[len(fake.getByPodArgsForCall)]
	fake.getByPodArgsForCall = append(fake.getByPodArgsForCall, struct {
		arg1 context.Context
		arg2 v1.Pod
	}{arg1, arg2})
	stub := fake.GetByPodStub
	fakeReturns := fake.getByPodReturns
	fake.recordInvocation("GetByPod", []interface{}{arg1, arg2})
	fake.getByPodMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeEventGetter) GetByPodCallCount() int {
	fake.getByPodMutex.RLock()
	defer fake.getByPodMutex.RUnlock()
	return len(fake.getByPodArgsForCall)
}

func (fake *FakeEventGetter) GetByPodCalls(stub func(context.Context, v1.Pod) ([]v1.Event, error)) {
	fake.getByPodMutex.Lock()
	defer fake.getByPodMutex.Unlock()
	fake.GetByPodStub = stub
}

func (fake *FakeEventGetter) GetByPodArgsForCall(i int) (context.Context, v1.Pod) {
	fake.getByPodMutex.RLock()
	defer fake.getByPodMutex.RUnlock()
	argsForCall := fake.getByPodArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeEventGetter) GetByPodReturns(result1 []v1.Event, result2 error) {
	fake.getByPodMutex.Lock()
	defer fake.getByPodMutex.Unlock()
	fake.GetByPodStub = nil
	fake.getByPodReturns = struct {
		result1 []v1.Event
		result2 error
	}{result1, result2}
}

func (fake *FakeEventGetter) GetByPodReturnsOnCall(i int, result1 []v1.Event, result2 error) {
	fake.getByPodMutex.Lock()
	defer fake.getByPodMutex.Unlock()
	fake.GetByPodStub = nil
	if fake.getByPodReturnsOnCall == nil {
		fake.getByPodReturnsOnCall = make(map[int]struct {
			result1 []v1.Event
			result2 error
		})
	}
	fake.getByPodReturnsOnCall[i] = struct {
		result1 []v1.Event
		result2 error
	}{result1, result2}
}

func (fake *FakeEventGetter) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getByPodMutex.RLock()
	defer fake.getByPodMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeEventGetter) recordInvocation(key string, args []interface{}) {
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

var _ stset.EventGetter = new(FakeEventGetter)
