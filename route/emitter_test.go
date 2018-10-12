package route_test

import (
	"encoding/json"
	"errors"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/route/routefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/route"
)

var _ = Describe("Emitter", func() {

	var (
		scheduler    *routefakes.FakeTaskScheduler
		publisher    *routefakes.FakePublisher
		workChannel  chan []*eirini.Routes
		emitter      *Emitter
		routes       []*eirini.Routes
		messageCount int
		useIngress   bool
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

	getRegisterMessage := func(routes *eirini.Routes) []byte {
		m := RegistryMessage{
			Host:    routes.ServiceAddress,
			Port:    routes.ServicePort,
			TLSPort: routes.ServiceTLSPort,
			URIs:    routes.Routes,
			App:     routes.Name,
		}

		if useIngress {
			m.Host = kubeEndpoint
			m.Port = httpPort
			m.TLSPort = tlsPort
		}

		data, err := json.Marshal(m)
		Expect(err).ToNot(HaveOccurred())
		return data
	}

	getUnregisterMessage := func(routes *eirini.Routes) []byte {
		m := RegistryMessage{
			Host:    routes.ServiceAddress,
			Port:    routes.ServicePort,
			TLSPort: routes.ServiceTLSPort,
			URIs:    routes.UnregisteredRoutes,
			App:     routes.Name,
		}

		if useIngress {
			m.Host = kubeEndpoint
			m.Port = httpPort
			m.TLSPort = tlsPort
		}

		data, err := json.Marshal(m)
		Expect(err).ToNot(HaveOccurred())
		return data
	}

	getExpectedData := func() (registered [][]byte, unregistered [][]byte) {
		for _, r := range routes {
			if len(r.Routes) != 0 {
				m := getRegisterMessage(r)
				registered = append(registered, m)
			}
			if len(r.UnregisteredRoutes) != 0 {
				m := getUnregisterMessage(r)
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
		workChannel = make(chan []*eirini.Routes, 1)

		route := eirini.Routes{
			Routes:             []string{"route1.my.app.com"},
			UnregisteredRoutes: []string{"removed.route1.my.app.com"},
			Name:               "app1",
			ServiceAddress:     "203.0.113.2",
			ServicePort:        8080,
			ServiceTLSPort:     8443,
		}

		routes = []*eirini.Routes{&route}

		messageCount = countMessages()
		emitter = NewEmitter(publisher, workChannel, scheduler, kubeEndpoint, useIngress)
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
	})

	Context("When the publisher returns an error", func() {
		useIngress = true

		BeforeEach(func() {
			publisher.PublishReturns(errors.New("Failed to publish message"))
		})

		assertInteractionsWithFakes()
	})

	Context("When not using an ingress", func() {
		useIngress = false

		BeforeEach(func() {
			publisher.PublishReturns(errors.New("Failed to publish message"))
		})
	})
})
