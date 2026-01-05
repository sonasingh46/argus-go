// Package integration contains end-to-end integration tests for ArgusGo.
// These tests verify the complete flow from HTTP request to alert creation.
package integration

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ArgusGo Integration Suite")
}
