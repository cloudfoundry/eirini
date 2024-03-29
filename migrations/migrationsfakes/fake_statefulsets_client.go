// Code generated by counterfeiter. DO NOT EDIT.
package migrationsfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/migrations"
	v1 "k8s.io/api/apps/v1"
)

type FakeStatefulsetsClient struct {
	GetBySourceTypeStub        func(context.Context, string) ([]v1.StatefulSet, error)
	getBySourceTypeMutex       sync.RWMutex
	getBySourceTypeArgsForCall []struct {
		arg1 context.Context
		arg2 string
	}
	getBySourceTypeReturns struct {
		result1 []v1.StatefulSet
		result2 error
	}
	getBySourceTypeReturnsOnCall map[int]struct {
		result1 []v1.StatefulSet
		result2 error
	}
	SetAnnotationStub        func(context.Context, *v1.StatefulSet, string, string) (*v1.StatefulSet, error)
	setAnnotationMutex       sync.RWMutex
	setAnnotationArgsForCall []struct {
		arg1 context.Context
		arg2 *v1.StatefulSet
		arg3 string
		arg4 string
	}
	setAnnotationReturns struct {
		result1 *v1.StatefulSet
		result2 error
	}
	setAnnotationReturnsOnCall map[int]struct {
		result1 *v1.StatefulSet
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeStatefulsetsClient) GetBySourceType(arg1 context.Context, arg2 string) ([]v1.StatefulSet, error) {
	fake.getBySourceTypeMutex.Lock()
	ret, specificReturn := fake.getBySourceTypeReturnsOnCall[len(fake.getBySourceTypeArgsForCall)]
	fake.getBySourceTypeArgsForCall = append(fake.getBySourceTypeArgsForCall, struct {
		arg1 context.Context
		arg2 string
	}{arg1, arg2})
	stub := fake.GetBySourceTypeStub
	fakeReturns := fake.getBySourceTypeReturns
	fake.recordInvocation("GetBySourceType", []interface{}{arg1, arg2})
	fake.getBySourceTypeMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeStatefulsetsClient) GetBySourceTypeCallCount() int {
	fake.getBySourceTypeMutex.RLock()
	defer fake.getBySourceTypeMutex.RUnlock()
	return len(fake.getBySourceTypeArgsForCall)
}

func (fake *FakeStatefulsetsClient) GetBySourceTypeCalls(stub func(context.Context, string) ([]v1.StatefulSet, error)) {
	fake.getBySourceTypeMutex.Lock()
	defer fake.getBySourceTypeMutex.Unlock()
	fake.GetBySourceTypeStub = stub
}

func (fake *FakeStatefulsetsClient) GetBySourceTypeArgsForCall(i int) (context.Context, string) {
	fake.getBySourceTypeMutex.RLock()
	defer fake.getBySourceTypeMutex.RUnlock()
	argsForCall := fake.getBySourceTypeArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeStatefulsetsClient) GetBySourceTypeReturns(result1 []v1.StatefulSet, result2 error) {
	fake.getBySourceTypeMutex.Lock()
	defer fake.getBySourceTypeMutex.Unlock()
	fake.GetBySourceTypeStub = nil
	fake.getBySourceTypeReturns = struct {
		result1 []v1.StatefulSet
		result2 error
	}{result1, result2}
}

func (fake *FakeStatefulsetsClient) GetBySourceTypeReturnsOnCall(i int, result1 []v1.StatefulSet, result2 error) {
	fake.getBySourceTypeMutex.Lock()
	defer fake.getBySourceTypeMutex.Unlock()
	fake.GetBySourceTypeStub = nil
	if fake.getBySourceTypeReturnsOnCall == nil {
		fake.getBySourceTypeReturnsOnCall = make(map[int]struct {
			result1 []v1.StatefulSet
			result2 error
		})
	}
	fake.getBySourceTypeReturnsOnCall[i] = struct {
		result1 []v1.StatefulSet
		result2 error
	}{result1, result2}
}

func (fake *FakeStatefulsetsClient) SetAnnotation(arg1 context.Context, arg2 *v1.StatefulSet, arg3 string, arg4 string) (*v1.StatefulSet, error) {
	fake.setAnnotationMutex.Lock()
	ret, specificReturn := fake.setAnnotationReturnsOnCall[len(fake.setAnnotationArgsForCall)]
	fake.setAnnotationArgsForCall = append(fake.setAnnotationArgsForCall, struct {
		arg1 context.Context
		arg2 *v1.StatefulSet
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

func (fake *FakeStatefulsetsClient) SetAnnotationCallCount() int {
	fake.setAnnotationMutex.RLock()
	defer fake.setAnnotationMutex.RUnlock()
	return len(fake.setAnnotationArgsForCall)
}

func (fake *FakeStatefulsetsClient) SetAnnotationCalls(stub func(context.Context, *v1.StatefulSet, string, string) (*v1.StatefulSet, error)) {
	fake.setAnnotationMutex.Lock()
	defer fake.setAnnotationMutex.Unlock()
	fake.SetAnnotationStub = stub
}

func (fake *FakeStatefulsetsClient) SetAnnotationArgsForCall(i int) (context.Context, *v1.StatefulSet, string, string) {
	fake.setAnnotationMutex.RLock()
	defer fake.setAnnotationMutex.RUnlock()
	argsForCall := fake.setAnnotationArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeStatefulsetsClient) SetAnnotationReturns(result1 *v1.StatefulSet, result2 error) {
	fake.setAnnotationMutex.Lock()
	defer fake.setAnnotationMutex.Unlock()
	fake.SetAnnotationStub = nil
	fake.setAnnotationReturns = struct {
		result1 *v1.StatefulSet
		result2 error
	}{result1, result2}
}

func (fake *FakeStatefulsetsClient) SetAnnotationReturnsOnCall(i int, result1 *v1.StatefulSet, result2 error) {
	fake.setAnnotationMutex.Lock()
	defer fake.setAnnotationMutex.Unlock()
	fake.SetAnnotationStub = nil
	if fake.setAnnotationReturnsOnCall == nil {
		fake.setAnnotationReturnsOnCall = make(map[int]struct {
			result1 *v1.StatefulSet
			result2 error
		})
	}
	fake.setAnnotationReturnsOnCall[i] = struct {
		result1 *v1.StatefulSet
		result2 error
	}{result1, result2}
}

func (fake *FakeStatefulsetsClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getBySourceTypeMutex.RLock()
	defer fake.getBySourceTypeMutex.RUnlock()
	fake.setAnnotationMutex.RLock()
	defer fake.setAnnotationMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeStatefulsetsClient) recordInvocation(key string, args []interface{}) {
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

var _ migrations.StatefulsetsClient = new(FakeStatefulsetsClient)
