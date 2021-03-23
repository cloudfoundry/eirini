// Code generated by counterfeiter. DO NOT EDIT.
package eventfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/k8s/informers/route/event"
	v1 "k8s.io/api/apps/v1"
)

type FakeStatefulSetGetter struct {
	GetStub        func(context.Context, string, string) (*v1.StatefulSet, error)
	getMutex       sync.RWMutex
	getArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
	}
	getReturns struct {
		result1 *v1.StatefulSet
		result2 error
	}
	getReturnsOnCall map[int]struct {
		result1 *v1.StatefulSet
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeStatefulSetGetter) Get(arg1 context.Context, arg2 string, arg3 string) (*v1.StatefulSet, error) {
	fake.getMutex.Lock()
	ret, specificReturn := fake.getReturnsOnCall[len(fake.getArgsForCall)]
	fake.getArgsForCall = append(fake.getArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.GetStub
	fakeReturns := fake.getReturns
	fake.recordInvocation("Get", []interface{}{arg1, arg2, arg3})
	fake.getMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeStatefulSetGetter) GetCallCount() int {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	return len(fake.getArgsForCall)
}

func (fake *FakeStatefulSetGetter) GetCalls(stub func(context.Context, string, string) (*v1.StatefulSet, error)) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = stub
}

func (fake *FakeStatefulSetGetter) GetArgsForCall(i int) (context.Context, string, string) {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	argsForCall := fake.getArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeStatefulSetGetter) GetReturns(result1 *v1.StatefulSet, result2 error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	fake.getReturns = struct {
		result1 *v1.StatefulSet
		result2 error
	}{result1, result2}
}

func (fake *FakeStatefulSetGetter) GetReturnsOnCall(i int, result1 *v1.StatefulSet, result2 error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	if fake.getReturnsOnCall == nil {
		fake.getReturnsOnCall = make(map[int]struct {
			result1 *v1.StatefulSet
			result2 error
		})
	}
	fake.getReturnsOnCall[i] = struct {
		result1 *v1.StatefulSet
		result2 error
	}{result1, result2}
}

func (fake *FakeStatefulSetGetter) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeStatefulSetGetter) recordInvocation(key string, args []interface{}) {
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

var _ event.StatefulSetGetter = new(FakeStatefulSetGetter)
