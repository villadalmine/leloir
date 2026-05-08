package minimal_test

import (
	"testing"

	"github.com/leloir/sdk/conformance"
	"github.com/leloir/sdk/examples/minimal"
)

// TestConformance verifies the minimal adapter satisfies the AgentAdapter contract.
func TestConformance(t *testing.T) {
	a := minimal.New()
	conformance.RunSuite(t, a, conformance.DefaultOptions())
}
