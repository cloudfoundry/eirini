package loggregator_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/golang/protobuf/jsonpb"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

var _ = Describe("RlpGatewayClient", func() {
	var (
		spyDoer *spyDoer
		c       *loggregator.RLPGatewayClient
	)

	BeforeEach(func() {
		spyDoer = newSpyDoer()
		c = loggregator.NewRLPGatewayClient(
			"https://some.addr",
			loggregator.WithRLPGatewayHTTPClient(spyDoer),
		)
	})

	It("requests envelopes from the RLP", func() {
		ch := make(chan []byte, 100)
		spyDoer.resps = append(spyDoer.resps, &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(channelReader(ch)),
		})
		spyDoer.errs = []error{nil}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c.Stream(ctx, &loggregator_v2.EgressBatchRequest{
			ShardId:           "some-shard",
			DeterministicName: "some-name",
			Selectors: []*loggregator_v2.Selector{
				{SourceId: "some-id-1", Message: &loggregator_v2.Selector_Log{Log: &loggregator_v2.LogSelector{}}},
			},
		})

		Eventually(spyDoer.Reqs).Should(HaveLen(1))

		req := spyDoer.Reqs()[0]

		Expect(req.URL.Scheme).To(Equal("https"))
		Expect(req.URL.Host).To(Equal("some.addr"))
		Expect(req.URL.Path).To(Equal("/v2/read"))
		Expect(req.Header.Get("Accept")).To(Equal("text/event-stream"))
		Expect(req.Header.Get("Cache-Control")).To(Equal("no-cache"))
		Expect(req.Method).To(Equal(http.MethodGet))
		Expect(req.URL.Query().Get("shard_id")).To(Equal("some-shard"))
		Expect(req.URL.Query().Get("deterministic_name")).To(Equal("some-name"))
		Expect(req.URL.Query().Get("source_id")).To(Equal("some-id-1"))
		Expect(req.URL.Query()).To(HaveKey("log"))
	})

	DescribeTable("encodes selectors correctly",
		func(selectors []*loggregator_v2.Selector, paramKey string, paramValue []string) {
			ch := make(chan []byte, 100)
			spyDoer.resps = append(spyDoer.resps, &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(channelReader(ch)),
			})
			spyDoer.errs = []error{nil}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c.Stream(ctx, &loggregator_v2.EgressBatchRequest{
				ShardId:           "some-shard",
				DeterministicName: "some-name",
				Selectors:         selectors,
			})

			Eventually(spyDoer.Reqs).Should(HaveLen(1))

			req := spyDoer.Reqs()[0]
			Expect(req.URL.Query()).To(HaveKeyWithValue(paramKey, paramValue))
			Expect(req.URL.Query()).To(HaveLen(3))
		},
		Entry("log",
			[]*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			}, "log", []string{""}),
		Entry("counter",
			[]*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Counter{
						Counter: &loggregator_v2.CounterSelector{},
					},
				},
			}, "counter", []string{""}),
		Entry("counter and counter with name",
			[]*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Counter{
						Counter: &loggregator_v2.CounterSelector{},
					},
				},
				{
					Message: &loggregator_v2.Selector_Counter{
						Counter: &loggregator_v2.CounterSelector{
							Name: "cntr",
						},
					},
				},
			}, "counter.name", []string{"cntr"}),
		Entry("gauge", []*loggregator_v2.Selector{
			{
				Message: &loggregator_v2.Selector_Gauge{
					Gauge: &loggregator_v2.GaugeSelector{},
				},
			},
		}, "gauge", []string{""}),
		Entry("gauge with name",
			[]*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Gauge{
						Gauge: &loggregator_v2.GaugeSelector{}},
				},
				{
					Message: &loggregator_v2.Selector_Gauge{
						Gauge: &loggregator_v2.GaugeSelector{
							Names: []string{"gauge"},
						},
					},
				},
			}, "gauge.name", []string{"gauge"}),
		Entry("timer",
			[]*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Timer{
						Timer: &loggregator_v2.TimerSelector{},
					},
				},
			}, "timer", []string{""}),
		Entry("event",
			[]*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Event{
						Event: &loggregator_v2.EventSelector{},
					},
				},
			}, "event", []string{""}),
		Entry("many source ID",
			[]*loggregator_v2.Selector{
				{
					SourceId: "some-id-1",
				},
				{
					SourceId: "some-id-2",
				},
				{
					SourceId: "some-id-2",
				},
			}, "source_id", []string{"some-id-1", "some-id-2"}),
	)

	It("streams envelopes", func() {
		ch := make(chan []byte, 100)
		spyDoer.resps = append(spyDoer.resps, &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(channelReader(ch)),
		})
		spyDoer.errs = []error{nil}

		go func() {
			m := jsonpb.Marshaler{}
			for i := 0; i < 10; i++ {
				s, err := m.MarshalToString(&loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{
						{Timestamp: int64(i)},
					},
				})
				if err != nil {
					panic(err)
				}
				ch <- []byte(fmt.Sprintf("data: %s\n\n", s))
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		es := c.Stream(ctx, &loggregator_v2.EgressBatchRequest{})

		envelopes := make(chan *loggregator_v2.Envelope, 100)
		go func() {
			for ctx.Err() == nil {
				for _, e := range es() {
					envelopes <- e
				}
			}
		}()

		Eventually(envelopes).Should(HaveLen(10))
	})

	It("batches envelopes", func() {
		ch := make(chan []byte, 100)
		spyDoer.resps = append(spyDoer.resps, &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(channelReader(ch)),
		})
		spyDoer.errs = []error{nil}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := jsonpb.Marshaler{}
			for i := 0; i < 10; i++ {
				s, err := m.MarshalToString(&loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{
						{Timestamp: int64(i)},
					},
				})
				if err != nil {
					panic(err)
				}
				ch <- []byte(fmt.Sprintf("data: %s\n\n", s))
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		es := c.Stream(ctx, &loggregator_v2.EgressBatchRequest{})

		wg.Wait()
		// Let the everything settle
		time.Sleep(250 * time.Millisecond)

		Expect(es()).ToNot(HaveLen(1))
	})

	It("reconnects for non-200 requests", func() {
		spyDoer.resps = append(spyDoer.resps, &http.Response{StatusCode: 500})
		spyDoer.resps = append(spyDoer.resps, &http.Response{StatusCode: 500})
		spyDoer.resps = append(spyDoer.resps, &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(channelReader(nil)),
		})
		spyDoer.errs = []error{nil, nil, nil}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		c.Stream(ctx, &loggregator_v2.EgressBatchRequest{})

		Eventually(spyDoer.Reqs).Should(HaveLen(3))
	})

	It("reconnects for any errors", func() {
		spyDoer.resps = append(spyDoer.resps, &http.Response{StatusCode: 200})
		spyDoer.resps = append(spyDoer.resps, &http.Response{StatusCode: 200})
		spyDoer.resps = append(spyDoer.resps, &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(channelReader(nil)),
		})
		spyDoer.errs = []error{errors.New("some-error"), errors.New("some-error"), nil}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		c.Stream(ctx, &loggregator_v2.EgressBatchRequest{})

		Eventually(spyDoer.Reqs).Should(HaveLen(3))
	})
})

type spyDoer struct {
	mu    sync.Mutex
	reqs  []*http.Request
	resps []*http.Response
	errs  []error
}

func newSpyDoer() *spyDoer {
	return &spyDoer{}
}

func (s *spyDoer) Do(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reqs = append(s.reqs, r)

	if len(s.resps) != len(s.errs) {
		panic("out of sync")
	}

	if len(s.resps) == 0 {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader(nil)),
		}, nil
	}

	resp, err := s.resps[0], s.errs[0]
	s.resps, s.errs = s.resps[1:], s.errs[1:]

	if resp.Body == nil {
		resp.Body = ioutil.NopCloser(bytes.NewReader(nil))
	}

	return resp, err
}

func (s *spyDoer) Reqs() []*http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]*http.Request, len(s.reqs))
	copy(results, s.reqs)
	return results
}

type channelReader <-chan []byte

func (r channelReader) Read(buf []byte) (int, error) {
	data := <-r
	n := copy(buf, data)
	return n, nil
}
