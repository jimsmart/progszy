package progszy_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestProgszy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Progszy Suite")
}
