package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/tlsconfig/certtest"
	. "github.com/onsi/ginkgo" //nolint:golint,stylecheck
	. "github.com/onsi/gomega" //nolint:golint,stylecheck
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v2"
)

func WriteTempFile(content []byte, fileName string) string {
	configFile, err := ioutil.TempFile("", fileName)
	Expect(err).ToNot(HaveOccurred())
	defer configFile.Close()

	err = ioutil.WriteFile(configFile.Name(), content, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())
	return configFile.Name()
}

func RunBinary(binaryPath string, config interface{}) (*gexec.Session, string) {
	configBytes, err := yaml.Marshal(config)
	Expect(err).NotTo(HaveOccurred())

	configFile := WriteTempFile(configBytes, filepath.Base(binaryPath)+"-config.yaml")
	command := exec.Command(binaryPath, "-c", configFile) // #nosec G204
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	return session, configFile
}

func GenerateKeyPair(name string) (string, string) {
	authority, err := certtest.BuildCA(name)
	Expect(err).NotTo(HaveOccurred())
	cert, err := authority.BuildSignedCertificate(name, certtest.WithDomains(name))
	Expect(err).NotTo(HaveOccurred())

	certData, keyData, err := cert.CertificatePEMAndPrivateKey()
	Expect(err).NotTo(HaveOccurred())
	metricsCertPath := WriteTempFile(certData, fmt.Sprintf("%s-cert", name))
	metricsKeyPath := WriteTempFile(keyData, fmt.Sprintf("%s-key", name))

	return metricsCertPath, metricsKeyPath
}
