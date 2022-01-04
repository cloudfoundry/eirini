// Code generated by counterfeiter. DO NOT EDIT.
package migrationsfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/migrations"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/api/policy/v1beta1"
)

type FakePDBClient struct {
	GetStub        func(context.Context, string, string) (*v1beta1.PodDisruptionBudget, error)
	getMutex       sync.RWMutex
	getArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
	}
	getReturns struct {
		result1 *v1beta1.PodDisruptionBudget
		result2 error
	}
	getReturnsOnCall map[int]struct {
		result1 *v1beta1.PodDisruptionBudget
		result2 error
	}
	SetOwnerStub        func(context.Context, *v1beta1.PodDisruptionBudget, *v1.StatefulSet) (*v1beta1.PodDisruptionBudget, error)
	setOwnerMutex       sync.RWMutex
	setOwnerArgsForCall []struct {
		arg1 context.Context
		arg2 *v1beta1.PodDisruptionBudget
		arg3 *v1.StatefulSet
	}
	setOwnerReturns struct {
		result1 *v1beta1.PodDisruptionBudget
		result2 error
	}
	setOwnerReturnsOnCall map[int]struct {
		result1 *v1beta1.PodDisruptionBudget
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakePDBClient) Get(arg1 context.Context, arg2 string, arg3 string) (*v1beta1.PodDisruptionBudget, error) {
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

func (fake *FakePDBClient) GetCallCount() int {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	return len(fake.getArgsForCall)
}

func (fake *FakePDBClient) GetCalls(stub func(context.Context, string, string) (*v1beta1.PodDisruptionBudget, error)) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = stub
}

func (fake *FakePDBClient) GetArgsForCall(i int) (context.Context, string, string) {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	argsForCall := fake.getArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakePDBClient) GetReturns(result1 *v1beta1.PodDisruptionBudget, result2 error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	fake.getReturns = struct {
		result1 *v1beta1.PodDisruptionBudget
		result2 error
	}{result1, result2}
}

func (fake *FakePDBClient) GetReturnsOnCall(i int, result1 *v1beta1.PodDisruptionBudget, result2 error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	if fake.getReturnsOnCall == nil {
		fake.getReturnsOnCall = make(map[int]struct {
			result1 *v1beta1.PodDisruptionBudget
			result2 error
		})
	}
	fake.getReturnsOnCall[i] = struct {
		result1 *v1beta1.PodDisruptionBudget
		result2 error
	}{result1, result2}
}

func (fake *FakePDBClient) SetOwner(arg1 context.Context, arg2 *v1beta1.PodDisruptionBudget, arg3 *v1.StatefulSet) (*v1beta1.PodDisruptionBudget, error) {
	fake.setOwnerMutex.Lock()
	ret, specificReturn := fake.setOwnerReturnsOnCall[len(fake.setOwnerArgsForCall)]
	fake.setOwnerArgsForCall = append(fake.setOwnerArgsForCall, struct {
		arg1 context.Context
		arg2 *v1beta1.PodDisruptionBudget
		arg3 *v1.StatefulSet
	}{arg1, arg2, arg3})
	stub := fake.SetOwnerStub
	fakeReturns := fake.setOwnerReturns
	fake.recordInvocation("SetOwner", []interface{}{arg1, arg2, arg3})
	fake.setOwnerMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePDBClient) SetOwnerCallCount() int {
	fake.setOwnerMutex.RLock()
	defer fake.setOwnerMutex.RUnlock()
	return len(fake.setOwnerArgsForCall)
}

func (fake *FakePDBClient) SetOwnerCalls(stub func(context.Context, *v1beta1.PodDisruptionBudget, *v1.StatefulSet) (*v1beta1.PodDisruptionBudget, error)) {
	fake.setOwnerMutex.Lock()
	defer fake.setOwnerMutex.Unlock()
	fake.SetOwnerStub = stub
}

func (fake *FakePDBClient) SetOwnerArgsForCall(i int) (context.Context, *v1beta1.PodDisruptionBudget, *v1.StatefulSet) {
	fake.setOwnerMutex.RLock()
	defer fake.setOwnerMutex.RUnlock()
	argsForCall := fake.setOwnerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakePDBClient) SetOwnerReturns(result1 *v1beta1.PodDisruptionBudget, result2 error) {
	fake.setOwnerMutex.Lock()
	defer fake.setOwnerMutex.Unlock()
	fake.SetOwnerStub = nil
	fake.setOwnerReturns = struct {
		result1 *v1beta1.PodDisruptionBudget
		result2 error
	}{result1, result2}
}

func (fake *FakePDBClient) SetOwnerReturnsOnCall(i int, result1 *v1beta1.PodDisruptionBudget, result2 error) {
	fake.setOwnerMutex.Lock()
	defer fake.setOwnerMutex.Unlock()
	fake.SetOwnerStub = nil
	if fake.setOwnerReturnsOnCall == nil {
		fake.setOwnerReturnsOnCall = make(map[int]struct {
			result1 *v1beta1.PodDisruptionBudget
			result2 error
		})
	}
	fake.setOwnerReturnsOnCall[i] = struct {
		result1 *v1beta1.PodDisruptionBudget
		result2 error
	}{result1, result2}
}

func (fake *FakePDBClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	fake.setOwnerMutex.RLock()
	defer fake.setOwnerMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakePDBClient) recordInvocation(key string, args []interface{}) {
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

var _ migrations.PDBClient = new(FakePDBClient)
