// Code generated by counterfeiter. DO NOT EDIT.
package reconcilerfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/k8s/reconciler"
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/opi"
)

type FakeLRPDesirer struct {
	DesireStub        func(context.Context, string, *opi.LRP, ...shared.Option) error
	desireMutex       sync.RWMutex
	desireArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 *opi.LRP
		arg4 []shared.Option
	}
	desireReturns struct {
		result1 error
	}
	desireReturnsOnCall map[int]struct {
		result1 error
	}
	GetStub        func(context.Context, opi.LRPIdentifier) (*opi.LRP, error)
	getMutex       sync.RWMutex
	getArgsForCall []struct {
		arg1 context.Context
		arg2 opi.LRPIdentifier
	}
	getReturns struct {
		result1 *opi.LRP
		result2 error
	}
	getReturnsOnCall map[int]struct {
		result1 *opi.LRP
		result2 error
	}
	UpdateStub        func(context.Context, *opi.LRP) error
	updateMutex       sync.RWMutex
	updateArgsForCall []struct {
		arg1 context.Context
		arg2 *opi.LRP
	}
	updateReturns struct {
		result1 error
	}
	updateReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeLRPDesirer) Desire(arg1 context.Context, arg2 string, arg3 *opi.LRP, arg4 ...shared.Option) error {
	fake.desireMutex.Lock()
	ret, specificReturn := fake.desireReturnsOnCall[len(fake.desireArgsForCall)]
	fake.desireArgsForCall = append(fake.desireArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 *opi.LRP
		arg4 []shared.Option
	}{arg1, arg2, arg3, arg4})
	stub := fake.DesireStub
	fakeReturns := fake.desireReturns
	fake.recordInvocation("Desire", []interface{}{arg1, arg2, arg3, arg4})
	fake.desireMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeLRPDesirer) DesireCallCount() int {
	fake.desireMutex.RLock()
	defer fake.desireMutex.RUnlock()
	return len(fake.desireArgsForCall)
}

func (fake *FakeLRPDesirer) DesireCalls(stub func(context.Context, string, *opi.LRP, ...shared.Option) error) {
	fake.desireMutex.Lock()
	defer fake.desireMutex.Unlock()
	fake.DesireStub = stub
}

func (fake *FakeLRPDesirer) DesireArgsForCall(i int) (context.Context, string, *opi.LRP, []shared.Option) {
	fake.desireMutex.RLock()
	defer fake.desireMutex.RUnlock()
	argsForCall := fake.desireArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeLRPDesirer) DesireReturns(result1 error) {
	fake.desireMutex.Lock()
	defer fake.desireMutex.Unlock()
	fake.DesireStub = nil
	fake.desireReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeLRPDesirer) DesireReturnsOnCall(i int, result1 error) {
	fake.desireMutex.Lock()
	defer fake.desireMutex.Unlock()
	fake.DesireStub = nil
	if fake.desireReturnsOnCall == nil {
		fake.desireReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.desireReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeLRPDesirer) Get(arg1 context.Context, arg2 opi.LRPIdentifier) (*opi.LRP, error) {
	fake.getMutex.Lock()
	ret, specificReturn := fake.getReturnsOnCall[len(fake.getArgsForCall)]
	fake.getArgsForCall = append(fake.getArgsForCall, struct {
		arg1 context.Context
		arg2 opi.LRPIdentifier
	}{arg1, arg2})
	stub := fake.GetStub
	fakeReturns := fake.getReturns
	fake.recordInvocation("Get", []interface{}{arg1, arg2})
	fake.getMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeLRPDesirer) GetCallCount() int {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	return len(fake.getArgsForCall)
}

func (fake *FakeLRPDesirer) GetCalls(stub func(context.Context, opi.LRPIdentifier) (*opi.LRP, error)) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = stub
}

func (fake *FakeLRPDesirer) GetArgsForCall(i int) (context.Context, opi.LRPIdentifier) {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	argsForCall := fake.getArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeLRPDesirer) GetReturns(result1 *opi.LRP, result2 error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	fake.getReturns = struct {
		result1 *opi.LRP
		result2 error
	}{result1, result2}
}

func (fake *FakeLRPDesirer) GetReturnsOnCall(i int, result1 *opi.LRP, result2 error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	if fake.getReturnsOnCall == nil {
		fake.getReturnsOnCall = make(map[int]struct {
			result1 *opi.LRP
			result2 error
		})
	}
	fake.getReturnsOnCall[i] = struct {
		result1 *opi.LRP
		result2 error
	}{result1, result2}
}

func (fake *FakeLRPDesirer) Update(arg1 context.Context, arg2 *opi.LRP) error {
	fake.updateMutex.Lock()
	ret, specificReturn := fake.updateReturnsOnCall[len(fake.updateArgsForCall)]
	fake.updateArgsForCall = append(fake.updateArgsForCall, struct {
		arg1 context.Context
		arg2 *opi.LRP
	}{arg1, arg2})
	stub := fake.UpdateStub
	fakeReturns := fake.updateReturns
	fake.recordInvocation("Update", []interface{}{arg1, arg2})
	fake.updateMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeLRPDesirer) UpdateCallCount() int {
	fake.updateMutex.RLock()
	defer fake.updateMutex.RUnlock()
	return len(fake.updateArgsForCall)
}

func (fake *FakeLRPDesirer) UpdateCalls(stub func(context.Context, *opi.LRP) error) {
	fake.updateMutex.Lock()
	defer fake.updateMutex.Unlock()
	fake.UpdateStub = stub
}

func (fake *FakeLRPDesirer) UpdateArgsForCall(i int) (context.Context, *opi.LRP) {
	fake.updateMutex.RLock()
	defer fake.updateMutex.RUnlock()
	argsForCall := fake.updateArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeLRPDesirer) UpdateReturns(result1 error) {
	fake.updateMutex.Lock()
	defer fake.updateMutex.Unlock()
	fake.UpdateStub = nil
	fake.updateReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeLRPDesirer) UpdateReturnsOnCall(i int, result1 error) {
	fake.updateMutex.Lock()
	defer fake.updateMutex.Unlock()
	fake.UpdateStub = nil
	if fake.updateReturnsOnCall == nil {
		fake.updateReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.updateReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeLRPDesirer) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.desireMutex.RLock()
	defer fake.desireMutex.RUnlock()
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	fake.updateMutex.RLock()
	defer fake.updateMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeLRPDesirer) recordInvocation(key string, args []interface{}) {
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

var _ reconciler.LRPDesirer = new(FakeLRPDesirer)
