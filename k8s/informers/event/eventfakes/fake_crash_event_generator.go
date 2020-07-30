// Code generated by counterfeiter. DO NOT EDIT.
package eventfakes

import (
	"sync"

	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s/informers/event"
	"code.cloudfoundry.org/lager"
	v1 "k8s.io/api/core/v1"
)

type FakeCrashEventGenerator struct {
	GenerateStub        func(*v1.Pod, lager.Logger) (events.CrashEvent, bool)
	generateMutex       sync.RWMutex
	generateArgsForCall []struct {
		arg1 *v1.Pod
		arg2 lager.Logger
	}
	generateReturns struct {
		result1 events.CrashEvent
		result2 bool
	}
	generateReturnsOnCall map[int]struct {
		result1 events.CrashEvent
		result2 bool
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeCrashEventGenerator) Generate(arg1 *v1.Pod, arg2 lager.Logger) (events.CrashEvent, bool) {
	fake.generateMutex.Lock()
	ret, specificReturn := fake.generateReturnsOnCall[len(fake.generateArgsForCall)]
	fake.generateArgsForCall = append(fake.generateArgsForCall, struct {
		arg1 *v1.Pod
		arg2 lager.Logger
	}{arg1, arg2})
	fake.recordInvocation("Generate", []interface{}{arg1, arg2})
	fake.generateMutex.Unlock()
	if fake.GenerateStub != nil {
		return fake.GenerateStub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.generateReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeCrashEventGenerator) GenerateCallCount() int {
	fake.generateMutex.RLock()
	defer fake.generateMutex.RUnlock()
	return len(fake.generateArgsForCall)
}

func (fake *FakeCrashEventGenerator) GenerateCalls(stub func(*v1.Pod, lager.Logger) (events.CrashEvent, bool)) {
	fake.generateMutex.Lock()
	defer fake.generateMutex.Unlock()
	fake.GenerateStub = stub
}

func (fake *FakeCrashEventGenerator) GenerateArgsForCall(i int) (*v1.Pod, lager.Logger) {
	fake.generateMutex.RLock()
	defer fake.generateMutex.RUnlock()
	argsForCall := fake.generateArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeCrashEventGenerator) GenerateReturns(result1 events.CrashEvent, result2 bool) {
	fake.generateMutex.Lock()
	defer fake.generateMutex.Unlock()
	fake.GenerateStub = nil
	fake.generateReturns = struct {
		result1 events.CrashEvent
		result2 bool
	}{result1, result2}
}

func (fake *FakeCrashEventGenerator) GenerateReturnsOnCall(i int, result1 events.CrashEvent, result2 bool) {
	fake.generateMutex.Lock()
	defer fake.generateMutex.Unlock()
	fake.GenerateStub = nil
	if fake.generateReturnsOnCall == nil {
		fake.generateReturnsOnCall = make(map[int]struct {
			result1 events.CrashEvent
			result2 bool
		})
	}
	fake.generateReturnsOnCall[i] = struct {
		result1 events.CrashEvent
		result2 bool
	}{result1, result2}
}

func (fake *FakeCrashEventGenerator) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.generateMutex.RLock()
	defer fake.generateMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeCrashEventGenerator) recordInvocation(key string, args []interface{}) {
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

var _ event.CrashEventGenerator = new(FakeCrashEventGenerator)
