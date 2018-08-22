# GO Shims

## why?
Have you ever wanted to fake out go system libary calls? In most cases you create an interface and then provide a mock/fake implementation and a shim that calls the real calls. That's great if you only have to do it once. What happens when it becomes a pattern and these little utilities end up duplicated everywhere...that's a problem. This repo is the solution.

## how was it made?
As of [this](https://github.com/maxbrunsfeld/counterfeiter/commit/c2f4a41282ca1e8652d0b534450f021380b1bf39) commit on maxbrunsfeld/counterfeiter, counterfeiter now has the ability to auto-generate interfaces/shims for system libaries. That's cool! Instead of generating those on the fly all the time, we collected some popular ones here for your convience. 

## how to use it?
In your struct for your class add a varible referencing the interface:
```
package abroker
import (
	"code.cloudfoundry.org/goshims/ioutilshim"
	"code.cloudfoundry.org/goshims/osshim"
)

type broker struct {
	os          osshim.Os
	ioutil      ioutilshim.Ioutil
}

func New(
	os osshim.Os, ioutil ioutilshim.Ioutil,
) *broker {
	theBroker := broker{
		os:          os,
		ioutil:      ioutil,
	}
	return &theBroker
}

func (b *broker) Serialize(state interface{}) error {
	stateFile := "/tmp/abrokerstate.json"

	stateData, err := json.Marshal(state)
	if err != nil {
		return err
	}

	err = b.ioutil.WriteFile(stateFile, stateData, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

```
In the factory method to construct that class, dependency inject the right version of the implemenation.
For example, your test code would use the fakes:
```
package abroker_test
import(
	"github.com/something/abroker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"code.cloudfoundry.org/goshims/osshim/os_fake"
	"code.cloudfoundry.org/goshims/ioutilshim/ioutil_fake"
)

var (
	fakeOs             *os_fake.FakeOs
	fakeIoutil         *ioutil_fake.FakeIoutil
... 
BeforeEach(func() {
	fakeOs = &os_fake.FakeOs{}
	fakeIoutil = &ioutil_fake.FakeIoutil{}
...

It("Should error if write state fails", func(){
	fakeIoutil.WriteFileReturns(errors.New("Error writing file."))
	broker = abroker.New(fakeOs, fakeIoutil)
	err := broker.Serialize(someData)
	Expect(err).To(HaveOccurred())
})
```
In your production code you would use the real implementation:
```
pacakge main
import(
	"github.com/something/abroker"
	"code.cloudfoundry.org/goshims/ioutilshim"
	"code.cloudfoundry.org/goshims/osshim"
)
...

func main() {
	broker := abroker.New(&osshim.OsShim{}, &ioutilshim.IoutilShim{})

...

```

## what's included

Let's just look at the details of one of the packages: osshim

It is an interface for faking out your os, just in case your code interacts with the file system heavily and you want to be able to induce failures.

That batteries are included!
The Os implementation in the base directory calls through to go's os package,
The Os implementation in the fakes directory calls to a counterfeiter fake for use in test.

The other packages behave the same and are aptly named.

## enjoy!
Feel free to PR more packages and we'll be happy to include them. Otherwise, we hope you find this as usefull as we do.
