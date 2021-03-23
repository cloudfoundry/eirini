// Code generated by counterfeiter. DO NOT EDIT.
package migrationsfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/migrations"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type FakeCPURequestSetter struct {
	SetCPURequestStub        func(context.Context, *v1.StatefulSet, *resource.Quantity) (*v1.StatefulSet, error)
	setCPURequestMutex       sync.RWMutex
	setCPURequestArgsForCall []struct {
		arg1 context.Context
		arg2 *v1.StatefulSet
		arg3 *resource.Quantity
	}
	setCPURequestReturns struct {
		result1 *v1.StatefulSet
		result2 error
	}
	setCPURequestReturnsOnCall map[int]struct {
		result1 *v1.StatefulSet
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeCPURequestSetter) SetCPURequest(arg1 context.Context, arg2 *v1.StatefulSet, arg3 *resource.Quantity) (*v1.StatefulSet, error) {
	fake.setCPURequestMutex.Lock()
	ret, specificReturn := fake.setCPURequestReturnsOnCall[len(fake.setCPURequestArgsForCall)]
	fake.setCPURequestArgsForCall = append(fake.setCPURequestArgsForCall, struct {
		arg1 context.Context
		arg2 *v1.StatefulSet
		arg3 *resource.Quantity
	}{arg1, arg2, arg3})
	stub := fake.SetCPURequestStub
	fakeReturns := fake.setCPURequestReturns
	fake.recordInvocation("SetCPURequest", []interface{}{arg1, arg2, arg3})
	fake.setCPURequestMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeCPURequestSetter) SetCPURequestCallCount() int {
	fake.setCPURequestMutex.RLock()
	defer fake.setCPURequestMutex.RUnlock()
	return len(fake.setCPURequestArgsForCall)
}

func (fake *FakeCPURequestSetter) SetCPURequestCalls(stub func(context.Context, *v1.StatefulSet, *resource.Quantity) (*v1.StatefulSet, error)) {
	fake.setCPURequestMutex.Lock()
	defer fake.setCPURequestMutex.Unlock()
	fake.SetCPURequestStub = stub
}

func (fake *FakeCPURequestSetter) SetCPURequestArgsForCall(i int) (context.Context, *v1.StatefulSet, *resource.Quantity) {
	fake.setCPURequestMutex.RLock()
	defer fake.setCPURequestMutex.RUnlock()
	argsForCall := fake.setCPURequestArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeCPURequestSetter) SetCPURequestReturns(result1 *v1.StatefulSet, result2 error) {
	fake.setCPURequestMutex.Lock()
	defer fake.setCPURequestMutex.Unlock()
	fake.SetCPURequestStub = nil
	fake.setCPURequestReturns = struct {
		result1 *v1.StatefulSet
		result2 error
	}{result1, result2}
}

func (fake *FakeCPURequestSetter) SetCPURequestReturnsOnCall(i int, result1 *v1.StatefulSet, result2 error) {
	fake.setCPURequestMutex.Lock()
	defer fake.setCPURequestMutex.Unlock()
	fake.SetCPURequestStub = nil
	if fake.setCPURequestReturnsOnCall == nil {
		fake.setCPURequestReturnsOnCall = make(map[int]struct {
			result1 *v1.StatefulSet
			result2 error
		})
	}
	fake.setCPURequestReturnsOnCall[i] = struct {
		result1 *v1.StatefulSet
		result2 error
	}{result1, result2}
}

func (fake *FakeCPURequestSetter) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.setCPURequestMutex.RLock()
	defer fake.setCPURequestMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeCPURequestSetter) recordInvocation(key string, args []interface{}) {
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

var _ migrations.CPURequestSetter = new(FakeCPURequestSetter)
