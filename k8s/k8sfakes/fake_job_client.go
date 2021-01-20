// Code generated by counterfeiter. DO NOT EDIT.
package k8sfakes

import (
	"sync"

	"code.cloudfoundry.org/eirini/k8s"
	v1 "k8s.io/api/batch/v1"
)

type FakeJobClient struct {
	CreateStub        func(string, *v1.Job) (*v1.Job, error)
	createMutex       sync.RWMutex
	createArgsForCall []struct {
		arg1 string
		arg2 *v1.Job
	}
	createReturns struct {
		result1 *v1.Job
		result2 error
	}
	createReturnsOnCall map[int]struct {
		result1 *v1.Job
		result2 error
	}
	DeleteStub        func(string, string) error
	deleteMutex       sync.RWMutex
	deleteArgsForCall []struct {
		arg1 string
		arg2 string
	}
	deleteReturns struct {
		result1 error
	}
	deleteReturnsOnCall map[int]struct {
		result1 error
	}
	GetByGUIDStub        func(string, bool) ([]v1.Job, error)
	getByGUIDMutex       sync.RWMutex
	getByGUIDArgsForCall []struct {
		arg1 string
		arg2 bool
	}
	getByGUIDReturns struct {
		result1 []v1.Job
		result2 error
	}
	getByGUIDReturnsOnCall map[int]struct {
		result1 []v1.Job
		result2 error
	}
	ListStub        func(bool) ([]v1.Job, error)
	listMutex       sync.RWMutex
	listArgsForCall []struct {
		arg1 bool
	}
	listReturns struct {
		result1 []v1.Job
		result2 error
	}
	listReturnsOnCall map[int]struct {
		result1 []v1.Job
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeJobClient) Create(arg1 string, arg2 *v1.Job) (*v1.Job, error) {
	fake.createMutex.Lock()
	ret, specificReturn := fake.createReturnsOnCall[len(fake.createArgsForCall)]
	fake.createArgsForCall = append(fake.createArgsForCall, struct {
		arg1 string
		arg2 *v1.Job
	}{arg1, arg2})
	stub := fake.CreateStub
	fakeReturns := fake.createReturns
	fake.recordInvocation("Create", []interface{}{arg1, arg2})
	fake.createMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeJobClient) CreateCallCount() int {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	return len(fake.createArgsForCall)
}

func (fake *FakeJobClient) CreateCalls(stub func(string, *v1.Job) (*v1.Job, error)) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = stub
}

func (fake *FakeJobClient) CreateArgsForCall(i int) (string, *v1.Job) {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	argsForCall := fake.createArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeJobClient) CreateReturns(result1 *v1.Job, result2 error) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = nil
	fake.createReturns = struct {
		result1 *v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobClient) CreateReturnsOnCall(i int, result1 *v1.Job, result2 error) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = nil
	if fake.createReturnsOnCall == nil {
		fake.createReturnsOnCall = make(map[int]struct {
			result1 *v1.Job
			result2 error
		})
	}
	fake.createReturnsOnCall[i] = struct {
		result1 *v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobClient) Delete(arg1 string, arg2 string) error {
	fake.deleteMutex.Lock()
	ret, specificReturn := fake.deleteReturnsOnCall[len(fake.deleteArgsForCall)]
	fake.deleteArgsForCall = append(fake.deleteArgsForCall, struct {
		arg1 string
		arg2 string
	}{arg1, arg2})
	stub := fake.DeleteStub
	fakeReturns := fake.deleteReturns
	fake.recordInvocation("Delete", []interface{}{arg1, arg2})
	fake.deleteMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeJobClient) DeleteCallCount() int {
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	return len(fake.deleteArgsForCall)
}

func (fake *FakeJobClient) DeleteCalls(stub func(string, string) error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = stub
}

func (fake *FakeJobClient) DeleteArgsForCall(i int) (string, string) {
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	argsForCall := fake.deleteArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeJobClient) DeleteReturns(result1 error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = nil
	fake.deleteReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeJobClient) DeleteReturnsOnCall(i int, result1 error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = nil
	if fake.deleteReturnsOnCall == nil {
		fake.deleteReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeJobClient) GetByGUID(arg1 string, arg2 bool) ([]v1.Job, error) {
	fake.getByGUIDMutex.Lock()
	ret, specificReturn := fake.getByGUIDReturnsOnCall[len(fake.getByGUIDArgsForCall)]
	fake.getByGUIDArgsForCall = append(fake.getByGUIDArgsForCall, struct {
		arg1 string
		arg2 bool
	}{arg1, arg2})
	stub := fake.GetByGUIDStub
	fakeReturns := fake.getByGUIDReturns
	fake.recordInvocation("GetByGUID", []interface{}{arg1, arg2})
	fake.getByGUIDMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeJobClient) GetByGUIDCallCount() int {
	fake.getByGUIDMutex.RLock()
	defer fake.getByGUIDMutex.RUnlock()
	return len(fake.getByGUIDArgsForCall)
}

func (fake *FakeJobClient) GetByGUIDCalls(stub func(string, bool) ([]v1.Job, error)) {
	fake.getByGUIDMutex.Lock()
	defer fake.getByGUIDMutex.Unlock()
	fake.GetByGUIDStub = stub
}

func (fake *FakeJobClient) GetByGUIDArgsForCall(i int) (string, bool) {
	fake.getByGUIDMutex.RLock()
	defer fake.getByGUIDMutex.RUnlock()
	argsForCall := fake.getByGUIDArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeJobClient) GetByGUIDReturns(result1 []v1.Job, result2 error) {
	fake.getByGUIDMutex.Lock()
	defer fake.getByGUIDMutex.Unlock()
	fake.GetByGUIDStub = nil
	fake.getByGUIDReturns = struct {
		result1 []v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobClient) GetByGUIDReturnsOnCall(i int, result1 []v1.Job, result2 error) {
	fake.getByGUIDMutex.Lock()
	defer fake.getByGUIDMutex.Unlock()
	fake.GetByGUIDStub = nil
	if fake.getByGUIDReturnsOnCall == nil {
		fake.getByGUIDReturnsOnCall = make(map[int]struct {
			result1 []v1.Job
			result2 error
		})
	}
	fake.getByGUIDReturnsOnCall[i] = struct {
		result1 []v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobClient) List(arg1 bool) ([]v1.Job, error) {
	fake.listMutex.Lock()
	ret, specificReturn := fake.listReturnsOnCall[len(fake.listArgsForCall)]
	fake.listArgsForCall = append(fake.listArgsForCall, struct {
		arg1 bool
	}{arg1})
	stub := fake.ListStub
	fakeReturns := fake.listReturns
	fake.recordInvocation("List", []interface{}{arg1})
	fake.listMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeJobClient) ListCallCount() int {
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	return len(fake.listArgsForCall)
}

func (fake *FakeJobClient) ListCalls(stub func(bool) ([]v1.Job, error)) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = stub
}

func (fake *FakeJobClient) ListArgsForCall(i int) bool {
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	argsForCall := fake.listArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeJobClient) ListReturns(result1 []v1.Job, result2 error) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = nil
	fake.listReturns = struct {
		result1 []v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobClient) ListReturnsOnCall(i int, result1 []v1.Job, result2 error) {
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

func (fake *FakeJobClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	fake.getByGUIDMutex.RLock()
	defer fake.getByGUIDMutex.RUnlock()
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeJobClient) recordInvocation(key string, args []interface{}) {
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

var _ k8s.JobClient = new(FakeJobClient)