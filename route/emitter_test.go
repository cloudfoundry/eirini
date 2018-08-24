package route_test

import (
	"encoding/json"
	"errors"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/eirinifakes"
	"code.cloudfoundry.org/eirini/route/routefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/route"
)

var _ = FDescribe("Emitter", func() {

	var (
		scheduler           *routefakes.FakeTaskScheduler
		publisher           *routefakes.FakePublisher
		fakeRemoveRouteFunc *eirinifakes.FakeRemoveCallbackFunc
		workChannel         chan []*eirini.Routes
		emitter             *Emitter
		routes              []*eirini.Routes
		messageCount        int
	)

	const (
		host              = "our.host.com"
		natsSubj          = "router.register"
		kubeEndpoint      = "example.com/kube"
		httpPort          = 80
		tlsPort           = 443
		registerSubject   = "router.register"
		unregisterSubject = "router.unregister"
	)

	countMessages := func() int {
		count := 0
		for _, r := range routes {
			count += len(r.Routes)
			count += len(r.UnregisteredRoutes)
		}
		return count
	}

	getRegisterMessage := func(routes []string, name string) []byte {
		m := RegistryMessage{
			Host:    kubeEndpoint,
			Port:    httpPort,
			TLSPort: tlsPort,
			URIs:    routes,
			App:     name,
		}
		data, err := json.Marshal(m)
		Expect(err).ToNot(HaveOccurred())
		return data
	}

	getExpectedData := func() (registered [][]byte, unregistered [][]byte) {
		for _, r := range routes {
			if len(r.Routes) != 0 {
				m := getRegisterMessage(r.Routes, r.Name)
				registered = append(registered, m)
			}
			if len(r.UnregisteredRoutes) != 0 {
				m := getRegisterMessage(r.UnregisteredRoutes, r.Name)
				unregistered = append(unregistered, m)
			}

		}
		return
	}

	getPublishData := func() (registered [][]byte, unregistered [][]byte) {
		for i := 0; i < messageCount; i++ {
			subj, data := publisher.PublishArgsForCall(i)
			Expect([]string{registerSubject, unregisterSubject}).To(ContainElement(subj))
			if subj == registerSubject {
				registered = append(registered, data)
			} else {
				unregistered = append(unregistered, data)
			}
		}
		return
	}

	compareSlices := func(actual [][]byte, expected [][]byte) {
		Expect(len(actual)).To(Equal(len(expected)))
		for _, e := range expected {
			Expect(actual).To(ContainElement(e))
		}
	}

	assertInteractionsWithFakes := func() {
		It("should publish the routes", func() {
			Expect(publisher.PublishCallCount()).To(Equal(messageCount))
			actualRegistered, actualUnregistered := getPublishData()
			expectedRegistered, expectedUnregistered := getExpectedData()

			compareSlices(actualRegistered, expectedRegistered)
			compareSlices(actualUnregistered, expectedUnregistered)
		})
	}

	BeforeEach(func() {
		scheduler = new(routefakes.FakeTaskScheduler)
		publisher = new(routefakes.FakePublisher)
		fakeRemoveRouteFunc = new(eirinifakes.FakeRemoveCallbackFunc)
		workChannel = make(chan []*eirini.Routes, 1)

		route := eirini.NewRoutes(fakeRemoveRouteFunc.Spy)
		route.Routes = []string{"route1.my.app.com"}
		route.UnregisteredRoutes = []string{"removed.route1.my.app.com"}
		route.Name = "app1"

		routes = []*eirini.Routes{route}

		messageCount = countMessages()
		emitter = NewEmitter(publisher, workChannel, scheduler, kubeEndpoint)
		emitter.Start()
	})

	AfterEach(func() {
		close(workChannel)
	})

	JustBeforeEach(func() {
		task := scheduler.ScheduleArgsForCall(0)
		workChannel <- routes

		err := task()
		Expect(err).ToNot(HaveOccurred())
		time.Sleep(time.Millisecond * 100) //TODO: Think of a better way (abstract goroutine? waitgroups?)
	})

	Context("When emitter is started", func() {
		assertInteractionsWithFakes()

		It("should remove the unregistered route", func() {
			Expect(fakeRemoveRouteFunc.CallCount()).To(Equal(1))
		})
	})

	Context("When the publisher returns an error", func() {
		BeforeEach(func() {
			publisher.PublishReturns(errors.New("Failed to publish message"))
		})

		assertInteractionsWithFakes()
		It("should not remove the unregistered route", func() {
			Expect(fakeRemoveRouteFunc.CallCount()).To(Equal(0))
		})
	})
})
