// Code generated by counterfeiter. DO NOT EDIT.
package stsetfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/k8s/stset"
	v1 "k8s.io/api/core/v1"
)

type FakeSecretsCreator struct {
	CreateStub        func(context.Context, string, *v1.Secret) (*v1.Secret, error)
	createMutex       sync.RWMutex
	createArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 *v1.Secret
	}
	createReturns struct {
		result1 *v1.Secret
		result2 error
	}
	createReturnsOnCall map[int]struct {
		result1 *v1.Secret
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeSecretsCreator) Create(arg1 context.Context, arg2 string, arg3 *v1.Secret) (*v1.Secret, error) {
	fake.createMutex.Lock()
	ret, specificReturn := fake.createReturnsOnCall[len(fake.createArgsForCall)]
	fake.createArgsForCall = append(fake.createArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 *v1.Secret
	}{arg1, arg2, arg3})
	stub := fake.CreateStub
	fakeReturns := fake.createReturns
	fake.recordInvocation("Create", []interface{}{arg1, arg2, arg3})
	fake.createMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeSecretsCreator) CreateCallCount() int {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	return len(fake.createArgsForCall)
}

func (fake *FakeSecretsCreator) CreateCalls(stub func(context.Context, string, *v1.Secret) (*v1.Secret, error)) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = stub
}

func (fake *FakeSecretsCreator) CreateArgsForCall(i int) (context.Context, string, *v1.Secret) {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	argsForCall := fake.createArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeSecretsCreator) CreateReturns(result1 *v1.Secret, result2 error) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = nil
	fake.createReturns = struct {
		result1 *v1.Secret
		result2 error
	}{result1, result2}
}

func (fake *FakeSecretsCreator) CreateReturnsOnCall(i int, result1 *v1.Secret, result2 error) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = nil
	if fake.createReturnsOnCall == nil {
		fake.createReturnsOnCall = make(map[int]struct {
			result1 *v1.Secret
			result2 error
		})
	}
	fake.createReturnsOnCall[i] = struct {
		result1 *v1.Secret
		result2 error
	}{result1, result2}
}

func (fake *FakeSecretsCreator) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeSecretsCreator) recordInvocation(key string, args []interface{}) {
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

var _ stset.SecretsCreator = new(FakeSecretsCreator)
