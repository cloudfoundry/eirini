// Code generated by counterfeiter. DO NOT EDIT.
package opifakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/opi"
)

type FakeDesirer struct {
	DesireStub        func(ctx context.Context, lrps []opi.LRP) error
	desireMutex       sync.RWMutex
	desireArgsForCall []struct {
		ctx  context.Context
		lrps []opi.LRP
	}
	desireReturns struct {
		result1 error
	}
	desireReturnsOnCall map[int]struct {
		result1 error
	}
	ListStub        func(ctx context.Context) ([]opi.LRP, error)
	listMutex       sync.RWMutex
	listArgsForCall []struct {
		ctx context.Context
	}
	listReturns struct {
		result1 []opi.LRP
		result2 error
	}
	listReturnsOnCall map[int]struct {
		result1 []opi.LRP
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeDesirer) Desire(ctx context.Context, lrps []opi.LRP) error {
	var lrpsCopy []opi.LRP
	if lrps != nil {
		lrpsCopy = make([]opi.LRP, len(lrps))
		copy(lrpsCopy, lrps)
	}
	fake.desireMutex.Lock()
	ret, specificReturn := fake.desireReturnsOnCall[len(fake.desireArgsForCall)]
	fake.desireArgsForCall = append(fake.desireArgsForCall, struct {
		ctx  context.Context
		lrps []opi.LRP
	}{ctx, lrpsCopy})
	fake.recordInvocation("Desire", []interface{}{ctx, lrpsCopy})
	fake.desireMutex.Unlock()
	if fake.DesireStub != nil {
		return fake.DesireStub(ctx, lrps)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.desireReturns.result1
}

func (fake *FakeDesirer) DesireCallCount() int {
	fake.desireMutex.RLock()
	defer fake.desireMutex.RUnlock()
	return len(fake.desireArgsForCall)
}

func (fake *FakeDesirer) DesireArgsForCall(i int) (context.Context, []opi.LRP) {
	fake.desireMutex.RLock()
	defer fake.desireMutex.RUnlock()
	return fake.desireArgsForCall[i].ctx, fake.desireArgsForCall[i].lrps
}

func (fake *FakeDesirer) DesireReturns(result1 error) {
	fake.DesireStub = nil
	fake.desireReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeDesirer) DesireReturnsOnCall(i int, result1 error) {
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

func (fake *FakeDesirer) List(ctx context.Context) ([]opi.LRP, error) {
	fake.listMutex.Lock()
	ret, specificReturn := fake.listReturnsOnCall[len(fake.listArgsForCall)]
	fake.listArgsForCall = append(fake.listArgsForCall, struct {
		ctx context.Context
	}{ctx})
	fake.recordInvocation("List", []interface{}{ctx})
	fake.listMutex.Unlock()
	if fake.ListStub != nil {
		return fake.ListStub(ctx)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.listReturns.result1, fake.listReturns.result2
}

func (fake *FakeDesirer) ListCallCount() int {
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	return len(fake.listArgsForCall)
}

func (fake *FakeDesirer) ListArgsForCall(i int) context.Context {
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	return fake.listArgsForCall[i].ctx
}

func (fake *FakeDesirer) ListReturns(result1 []opi.LRP, result2 error) {
	fake.ListStub = nil
	fake.listReturns = struct {
		result1 []opi.LRP
		result2 error
	}{result1, result2}
}

func (fake *FakeDesirer) ListReturnsOnCall(i int, result1 []opi.LRP, result2 error) {
	fake.ListStub = nil
	if fake.listReturnsOnCall == nil {
		fake.listReturnsOnCall = make(map[int]struct {
			result1 []opi.LRP
			result2 error
		})
	}
	fake.listReturnsOnCall[i] = struct {
		result1 []opi.LRP
		result2 error
	}{result1, result2}
}

func (fake *FakeDesirer) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.desireMutex.RLock()
	defer fake.desireMutex.RUnlock()
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeDesirer) recordInvocation(key string, args []interface{}) {
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

var _ opi.Desirer = new(FakeDesirer)
