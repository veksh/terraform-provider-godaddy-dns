package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/client"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

// common pre-checks for all acceptance tests
func testAccPreCheck(t *testing.T) {
	if os.Getenv("GODADDY_API_KEY") == "" || os.Getenv("GODADDY_API_SECRET") == "" {
		t.Fatal("env vars GODADDY_API_KEY and GODADDY_API_SECRET must be set")
	}
}

// provider instantiation for acceptance tests: use real API
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	// pass test to the constructor
	"godaddy-dns": providerserver.NewProtocol6WithError(New(
		"test",
		func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
			return client.NewClient(apiURL, apiKey, apiSecret)
		})()),
}

// provider instantiation for unit tests: use mock API
func mockClientProviderFactory(c *model.MockDNSApiClient) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"godaddy-dns": providerserver.NewProtocol6WithError(New(
			"unittest",
			func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
				return model.DNSApiClient(c), nil
			})()),
	}
}
