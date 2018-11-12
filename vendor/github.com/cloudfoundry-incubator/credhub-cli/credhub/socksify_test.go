package credhub_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync"

	"errors"

	"code.cloudfoundry.org/credhub-cli/credhub"
	"github.com/cloudfoundry/socks5-proxy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Socksify", func() {
	var (
		proxyDialer *FakeProxyDialer
		origDial    credhub.DialFunc
		dialFunc    credhub.DialFunc
	)

	BeforeEach(func() {
		os.Unsetenv("CREDHUB_PROXY")
		os.Unsetenv("https_proxy")
		proxyDialer = &FakeProxyDialer{}
		origDial = credhub.DialFunc(func(x, y string) (net.Conn, error) {
			return nil, errors.New("original dialer")
		})
		dialFunc = credhub.SOCKS5DialFuncFromEnvironment(origDial, proxyDialer)
	})
	Context("When CREDHUB_PROXY is not set", func() {
		It("Returns the dialer that was passed in", func() {
			_, err := dialFunc("", "")
			Expect(err).To(MatchError("original dialer"))
			Expect(proxyDialer.DialerCall.CallCount).To(Equal(0))
		})
	})

	Context("When CREDHUB_PROXY is set", func() {
		Context("When CREDHUB_PROXY is prefixed with ssh+", func() {
			BeforeEach(func() {
				proxyDialer.DialerCall.Returns.DialFunc = proxy.DialFunc(func(x, y string) (net.Conn, error) {
					return nil, errors.New("proxy dialer")
				})
				tempDir, err := ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())
				privateKeyPath := filepath.Join(tempDir, "test.key")
				err = ioutil.WriteFile(privateKeyPath, []byte("some-key"), 0600)
				Expect(err).NotTo(HaveOccurred())
				os.Setenv("CREDHUB_PROXY", fmt.Sprintf("ssh+socks5://user@localhost:12345?private-key=%s", privateKeyPath))
				dialFunc = credhub.SOCKS5DialFuncFromEnvironment(origDial, proxyDialer)
			})

			It("Returns a function that creates a socks5 proxy dialer", func() {
				_, err := dialFunc("", "")
				Expect(err).To(MatchError("proxy dialer"))
				Expect(proxyDialer.DialerCall.CallCount).To(Equal(1))
				Expect(proxyDialer.DialerCall.Receives.Key).To(Equal("some-key"))
				Expect(proxyDialer.DialerCall.Receives.URL).To(Equal("localhost:12345"))
				Expect(proxyDialer.DialerCall.Receives.Username).To(Equal("user"))
				os.Unsetenv("CREDHUB_PROXY")
			})

			It("Can be called multiple times and only create the dialer once", func() {
				_, err := dialFunc("", "")
				Expect(err).To(MatchError("proxy dialer"))
				_, err = dialFunc("", "")
				Expect(err).To(MatchError("proxy dialer"))
				Expect(proxyDialer.DialerCall.CallCount).To(Equal(1))
				Expect(proxyDialer.DialerCall.Receives.Key).To(Equal("some-key"))
				Expect(proxyDialer.DialerCall.Receives.URL).To(Equal("localhost:12345"))
				os.Unsetenv("CREDHUB_PROXY")
			})

			It("Can be concurrently (run ginkgo with -race flag)", func() {
				errs := make(chan error)
				for i := 0; i < 20; i++ {
					go func() {
						_, err := dialFunc("", "")
						errs <- err
					}()
				}
				for i := 0; i < 20; i++ {
					err := <-errs
					Expect(err).To(MatchError("proxy dialer"))
				}
				Expect(proxyDialer.DialerCall.CallCount).To(Equal(1))
				Expect(proxyDialer.DialerCall.Receives.Key).To(Equal("some-key"))
				Expect(proxyDialer.DialerCall.Receives.URL).To(Equal("localhost:12345"))
				os.Unsetenv("CREDHUB_PROXY")
			})

			Context("when the URL after the ssh+ prefix cannot be parsed", func() {
				BeforeEach(func() {
					os.Setenv("CREDHUB_PROXY", fmt.Sprintf("ssh+:cannot-start-with-colon"))
					dialFunc = credhub.SOCKS5DialFuncFromEnvironment(origDial, proxyDialer)
				})
				It("returns the dialer that was passed in", func() {
					_, err := dialFunc("", "")
					Expect(err).To(MatchError("original dialer"))
					os.Unsetenv("CREDHUB_PROXY")
				})
			})

			Context("when the query params in the URL cannot be parsed", func() {
				BeforeEach(func() {
					os.Setenv("CREDHUB_PROXY", fmt.Sprintf("ssh+socks5://localhost:12345?foo=%%"))
					dialFunc = credhub.SOCKS5DialFuncFromEnvironment(origDial, proxyDialer)
				})
				It("returns the dialer that was passed in", func() {
					_, err := dialFunc("", "")
					Expect(err).To(MatchError("original dialer"))
					os.Unsetenv("CREDHUB_PROXY")
				})
			})

			Context("when the query params do not contain the private key path", func() {
				BeforeEach(func() {
					os.Setenv("CREDHUB_PROXY", fmt.Sprintf("ssh+socks5://localhost:12345?foo=bar"))
					dialFunc = credhub.SOCKS5DialFuncFromEnvironment(origDial, proxyDialer)
				})
				It("returns the dialer that was passed in", func() {
					_, err := dialFunc("", "")
					Expect(err).To(MatchError("original dialer"))
					os.Unsetenv("CREDHUB_PROXY")
				})
			})

			Context("when no key exists at the private key path", func() {
				BeforeEach(func() {
					os.Setenv("CREDHUB_PROXY", fmt.Sprintf("ssh+socks5://localhost:12345?private-key=/no/file/here"))
					dialFunc = credhub.SOCKS5DialFuncFromEnvironment(origDial, proxyDialer)
				})
				It("returns the dialer that was passed in", func() {
					_, err := dialFunc("", "")
					Expect(err).To(MatchError("original dialer"))
					os.Unsetenv("CREDHUB_PROXY")
				})
			})
		})

		Context("When CREDHUB_PROXY is *not* prefixed with ssh+", func() {
			// Happy paths not tested
			Context("when the URL cannot be parsed", func() {
				BeforeEach(func() {
					os.Setenv("CREDHUB_PROXY", fmt.Sprintf(":cannot-start-with-colon"))
					dialFunc = credhub.SOCKS5DialFuncFromEnvironment(origDial, proxyDialer)
				})
				It("returns the dialer that was passed in", func() {
					_, err := dialFunc("", "")
					Expect(err).To(MatchError("original dialer"))
					os.Unsetenv("CREDHUB_PROXY")
				})
			})

			Context("when the URL is not a valid proxy scheme", func() {
				BeforeEach(func() {
					os.Setenv("CREDHUB_PROXY", fmt.Sprintf("foo://cannot-start-with-colon"))
					dialFunc = credhub.SOCKS5DialFuncFromEnvironment(origDial, proxyDialer)
				})
				It("returns the dialer that was passed in", func() {
					_, err := dialFunc("", "")
					Expect(err).To(MatchError("original dialer"))
					os.Unsetenv("CREDHUB_PROXY")
				})
			})
		})
	})
})

type FakeProxyDialer struct {
	DialerCall struct {
		CallCount int
		Receives  struct {
			Username string
			Key      string
			URL      string
		}
		Returns struct {
			DialFunc proxy.DialFunc
			Error    error
		}
	}
	mut sync.Mutex
}

func (p *FakeProxyDialer) Dialer(username, key, url string) (proxy.DialFunc, error) {
	p.mut.Lock()
	defer p.mut.Unlock()
	p.DialerCall.CallCount++
	p.DialerCall.Receives.Username = username
	p.DialerCall.Receives.Key = key
	p.DialerCall.Receives.URL = url

	return p.DialerCall.Returns.DialFunc, p.DialerCall.Returns.Error
}
