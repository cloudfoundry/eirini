// Code generated by counterfeiter. DO NOT EDIT.
package taskfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/k8s/informers/task"
	v1 "k8s.io/api/core/v1"
)

type FakePodsClient struct {
	SetAndTestAnnotationStub        func(context.Context, *v1.Pod, string, string, *string) (*v1.Pod, error)
	setAndTestAnnotationMutex       sync.RWMutex
	setAndTestAnnotationArgsForCall []struct {
		arg1 context.Context
		arg2 *v1.Pod
		arg3 string
		arg4 string
		arg5 *string
	}
	setAndTestAnnotationReturns struct {
		result1 *v1.Pod
		result2 error
	}
	setAndTestAnnotationReturnsOnCall map[int]struct {
		result1 *v1.Pod
		result2 error
	}
	SetAnnotationStub        func(context.Context, *v1.Pod, string, string) (*v1.Pod, error)
	setAnnotationMutex       sync.RWMutex
	setAnnotationArgsForCall []struct {
		arg1 context.Context
		arg2 *v1.Pod
		arg3 string
		arg4 string
	}
	setAnnotationReturns struct {
		result1 *v1.Pod
		result2 error
	}
	setAnnotationReturnsOnCall map[int]struct {
		result1 *v1.Pod
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakePodsClient) SetAndTestAnnotation(arg1 context.Context, arg2 *v1.Pod, arg3 string, arg4 string, arg5 *string) (*v1.Pod, error) {
	fake.setAndTestAnnotationMutex.Lock()
	ret, specificReturn := fake.setAndTestAnnotationReturnsOnCall[len(fake.setAndTestAnnotationArgsForCall)]
	fake.setAndTestAnnotationArgsForCall = append(fake.setAndTestAnnotationArgsForCall, struct {
		arg1 context.Context
		arg2 *v1.Pod
		arg3 string
		arg4 string
		arg5 *string
	}{arg1, arg2, arg3, arg4, arg5})
	stub := fake.SetAndTestAnnotationStub
	fakeReturns := fake.setAndTestAnnotationReturns
	fake.recordInvocation("SetAndTestAnnotation", []interface{}{arg1, arg2, arg3, arg4, arg5})
	fake.setAndTestAnnotationMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4, arg5)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePodsClient) SetAndTestAnnotationCallCount() int {
	fake.setAndTestAnnotationMutex.RLock()
	defer fake.setAndTestAnnotationMutex.RUnlock()
	return len(fake.setAndTestAnnotationArgsForCall)
}

func (fake *FakePodsClient) SetAndTestAnnotationCalls(stub func(context.Context, *v1.Pod, string, string, *string) (*v1.Pod, error)) {
	fake.setAndTestAnnotationMutex.Lock()
	defer fake.setAndTestAnnotationMutex.Unlock()
	fake.SetAndTestAnnotationStub = stub
}

func (fake *FakePodsClient) SetAndTestAnnotationArgsForCall(i int) (context.Context, *v1.Pod, string, string, *string) {
	fake.setAndTestAnnotationMutex.RLock()
	defer fake.setAndTestAnnotationMutex.RUnlock()
	argsForCall := fake.setAndTestAnnotationArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4, argsForCall.arg5
}

func (fake *FakePodsClient) SetAndTestAnnotationReturns(result1 *v1.Pod, result2 error) {
	fake.setAndTestAnnotationMutex.Lock()
	defer fake.setAndTestAnnotationMutex.Unlock()
	fake.SetAndTestAnnotationStub = nil
	fake.setAndTestAnnotationReturns = struct {
		result1 *v1.Pod
		result2 error
	}{result1, result2}
}

func (fake *FakePodsClient) SetAndTestAnnotationReturnsOnCall(i int, result1 *v1.Pod, result2 error) {
	fake.setAndTestAnnotationMutex.Lock()
	defer fake.setAndTestAnnotationMutex.Unlock()
	fake.SetAndTestAnnotationStub = nil
	if fake.setAndTestAnnotationReturnsOnCall == nil {
		fake.setAndTestAnnotationReturnsOnCall = make(map[int]struct {
			result1 *v1.Pod
			result2 error
		})
	}
	fake.setAndTestAnnotationReturnsOnCall[i] = struct {
		result1 *v1.Pod
		result2 error
	}{result1, result2}
}

func (fake *FakePodsClient) SetAnnotation(arg1 context.Context, arg2 *v1.Pod, arg3 string, arg4 string) (*v1.Pod, error) {
	fake.setAnnotationMutex.Lock()
	ret, specificReturn := fake.setAnnotationReturnsOnCall[len(fake.setAnnotationArgsForCall)]
	fake.setAnnotationArgsForCall = append(fake.setAnnotationArgsForCall, struct {
		arg1 context.Context
		arg2 *v1.Pod
		arg3 string
		arg4 string
	}{arg1, arg2, arg3, arg4})
	stub := fake.SetAnnotationStub
	fakeReturns := fake.setAnnotationReturns
	fake.recordInvocation("SetAnnotation", []interface{}{arg1, arg2, arg3, arg4})
	fake.setAnnotationMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePodsClient) SetAnnotationCallCount() int {
	fake.setAnnotationMutex.RLock()
	defer fake.setAnnotationMutex.RUnlock()
	return len(fake.setAnnotationArgsForCall)
}

func (fake *FakePodsClient) SetAnnotationCalls(stub func(context.Context, *v1.Pod, string, string) (*v1.Pod, error)) {
	fake.setAnnotationMutex.Lock()
	defer fake.setAnnotationMutex.Unlock()
	fake.SetAnnotationStub = stub
}

func (fake *FakePodsClient) SetAnnotationArgsForCall(i int) (context.Context, *v1.Pod, string, string) {
	fake.setAnnotationMutex.RLock()
	defer fake.setAnnotationMutex.RUnlock()
	argsForCall := fake.setAnnotationArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakePodsClient) SetAnnotationReturns(result1 *v1.Pod, result2 error) {
	fake.setAnnotationMutex.Lock()
	defer fake.setAnnotationMutex.Unlock()
	fake.SetAnnotationStub = nil
	fake.setAnnotationReturns = struct {
		result1 *v1.Pod
		result2 error
	}{result1, result2}
}

func (fake *FakePodsClient) SetAnnotationReturnsOnCall(i int, result1 *v1.Pod, result2 error) {
	fake.setAnnotationMutex.Lock()
	defer fake.setAnnotationMutex.Unlock()
	fake.SetAnnotationStub = nil
	if fake.setAnnotationReturnsOnCall == nil {
		fake.setAnnotationReturnsOnCall = make(map[int]struct {
			result1 *v1.Pod
			result2 error
		})
	}
	fake.setAnnotationReturnsOnCall[i] = struct {
		result1 *v1.Pod
		result2 error
	}{result1, result2}
}

func (fake *FakePodsClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.setAndTestAnnotationMutex.RLock()
	defer fake.setAndTestAnnotationMutex.RUnlock()
	fake.setAnnotationMutex.RLock()
	defer fake.setAnnotationMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakePodsClient) recordInvocation(key string, args []interface{}) {
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

var _ task.PodsClient = new(FakePodsClient)
