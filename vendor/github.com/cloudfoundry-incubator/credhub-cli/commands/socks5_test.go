package commands_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/armon/go-socks5"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("SOCKS5 support", func() {
	It("uses a SOCKS5 proxy when configured with the 'https_proxy' environment variable", func() {
		proxyAddress := "127.0.0.1:9000"
		proxyListener, err := net.Listen("tcp", proxyAddress)
		Expect(err).NotTo(HaveOccurred())

		fakeCredhubServer := &http.Server{
			Addr: OutboundServerAddress(),
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"auth-server": {"url": "https://%s"}}`, OutboundServerAddress())
			}),
		}

		go func() {
			proxyServer, err := socks5.New(&socks5.Config{})
			Expect(err).NotTo(HaveOccurred())
			proxyServer.Serve(proxyListener)
		}()

		certPath, keyPath := generateCertificate()

		go func() {
			fakeCredhubServer.ListenAndServeTLS(certPath, keyPath)
		}()

		defer func() {
			fakeCredhubServer.Close()
			os.RemoveAll(certPath)
			os.RemoveAll(keyPath)
			os.Unsetenv("https_proxy")
		}()

		os.Setenv("https_proxy", "socks5://"+proxyAddress)
		session := runCommand("api", "https://"+OutboundServerAddress(), "--ca-cert", certPath)
		Eventually(session).Should(Exit(0))

		Expect(proxyListener.Close()).To(Succeed())

		session = runCommand("api", "https://"+OutboundServerAddress(), "--ca-cert", certPath)
		Eventually(session).Should(Exit(1))
	})
})

func OutboundServerAddress() string {
	return OutboundServerIP().String() + ":9001"
}

func OutboundServerIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func generateCertificate() (string, string) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1337),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, 10),
		IsCA:         true,
		IPAddresses:  []net.IP{OutboundServerIP()},
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	Expect(err).NotTo(HaveOccurred())
	caCertificate, err := x509.CreateCertificate(rand.Reader, ca, ca, &privateKey.PublicKey, privateKey)
	Expect(err).NotTo(HaveOccurred())

	certFile, err := ioutil.TempFile("", "socks5-test")
	Expect(err).NotTo(HaveOccurred())

	pem.Encode(certFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertificate,
	})

	keyFile, err := ioutil.TempFile("", "socks5-test")
	Expect(err).NotTo(HaveOccurred())

	pem.Encode(keyFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return certFile.Name(), keyFile.Name()
}
