package bdd

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		Name:                "account_links_endpoints",
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format: "pretty",
			Paths:  []string{"features"},
		},
	}

	if status := suite.Run(); status != 0 {
		t.Fatalf("godog suite failed with status: %d", status)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

