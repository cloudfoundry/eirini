// Code generated by counterfeiter. DO NOT EDIT.
package migrationsfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/migrations"
	v1 "k8s.io/api/batch/v1"
)

type FakeJobsClient struct {
	ListStub        func(context.Context, bool) ([]v1.Job, error)
	listMutex       sync.RWMutex
	listArgsForCall []struct {
		arg1 context.Context
		arg2 bool
	}
	listReturns struct {
		result1 []v1.Job
		result2 error
	}
	listReturnsOnCall map[int]struct {
		result1 []v1.Job
		result2 error
	}
	SetAnnotationStub        func(context.Context, *v1.Job, string, string) (*v1.Job, error)
	setAnnotationMutex       sync.RWMutex
	setAnnotationArgsForCall []struct {
		arg1 context.Context
		arg2 *v1.Job
		arg3 string
		arg4 string
	}
	setAnnotationReturns struct {
		result1 *v1.Job
		result2 error
	}
	setAnnotationReturnsOnCall map[int]struct {
		result1 *v1.Job
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeJobsClient) List(arg1 context.Context, arg2 bool) ([]v1.Job, error) {
	fake.listMutex.Lock()
	ret, specificReturn := fake.listReturnsOnCall[len(fake.listArgsForCall)]
	fake.listArgsForCall = append(fake.listArgsForCall, struct {
		arg1 context.Context
		arg2 bool
	}{arg1, arg2})
	stub := fake.ListStub
	fakeReturns := fake.listReturns
	fake.recordInvocation("List", []interface{}{arg1, arg2})
	fake.listMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeJobsClient) ListCallCount() int {
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	return len(fake.listArgsForCall)
}

func (fake *FakeJobsClient) ListCalls(stub func(context.Context, bool) ([]v1.Job, error)) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = stub
}

func (fake *FakeJobsClient) ListArgsForCall(i int) (context.Context, bool) {
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	argsForCall := fake.listArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeJobsClient) ListReturns(result1 []v1.Job, result2 error) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = nil
	fake.listReturns = struct {
		result1 []v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobsClient) ListReturnsOnCall(i int, result1 []v1.Job, result2 error) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = nil
	if fake.listReturnsOnCall == nil {
		fake.listReturnsOnCall = make(map[int]struct {
			result1 []v1.Job
			result2 error
		})
	}
	fake.listReturnsOnCall[i] = struct {
		result1 []v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobsClient) SetAnnotation(arg1 context.Context, arg2 *v1.Job, arg3 string, arg4 string) (*v1.Job, error) {
	fake.setAnnotationMutex.Lock()
	ret, specificReturn := fake.setAnnotationReturnsOnCall[len(fake.setAnnotationArgsForCall)]
	fake.setAnnotationArgsForCall = append(fake.setAnnotationArgsForCall, struct {
		arg1 context.Context
		arg2 *v1.Job
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

func (fake *FakeJobsClient) SetAnnotationCallCount() int {
	fake.setAnnotationMutex.RLock()
	defer fake.setAnnotationMutex.RUnlock()
	return len(fake.setAnnotationArgsForCall)
}

func (fake *FakeJobsClient) SetAnnotationCalls(stub func(context.Context, *v1.Job, string, string) (*v1.Job, error)) {
	fake.setAnnotationMutex.Lock()
	defer fake.setAnnotationMutex.Unlock()
	fake.SetAnnotationStub = stub
}

func (fake *FakeJobsClient) SetAnnotationArgsForCall(i int) (context.Context, *v1.Job, string, string) {
	fake.setAnnotationMutex.RLock()
	defer fake.setAnnotationMutex.RUnlock()
	argsForCall := fake.setAnnotationArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeJobsClient) SetAnnotationReturns(result1 *v1.Job, result2 error) {
	fake.setAnnotationMutex.Lock()
	defer fake.setAnnotationMutex.Unlock()
	fake.SetAnnotationStub = nil
	fake.setAnnotationReturns = struct {
		result1 *v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobsClient) SetAnnotationReturnsOnCall(i int, result1 *v1.Job, result2 error) {
	fake.setAnnotationMutex.Lock()
	defer fake.setAnnotationMutex.Unlock()
	fake.SetAnnotationStub = nil
	if fake.setAnnotationReturnsOnCall == nil {
		fake.setAnnotationReturnsOnCall = make(map[int]struct {
			result1 *v1.Job
			result2 error
		})
	}
	fake.setAnnotationReturnsOnCall[i] = struct {
		result1 *v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobsClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	fake.setAnnotationMutex.RLock()
	defer fake.setAnnotationMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeJobsClient) recordInvocation(key string, args []interface{}) {
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

var _ migrations.JobsClient = new(FakeJobsClient)
