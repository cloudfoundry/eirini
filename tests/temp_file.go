package tests

import (
	"os"

	. "github.com/onsi/gomega"
)

func WriteTempFile(content []byte, fileName string) string {
	configFile, err := os.CreateTemp("", fileName)
	Expect(err).ToNot(HaveOccurred())

	defer configFile.Close()

	err = os.WriteFile(configFile.Name(), content, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	return configFile.Name()
}
