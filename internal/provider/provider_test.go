package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/client"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	// pass test to the constructor
	"godaddy-dns": providerserver.NewProtocol6WithError(New(
		"test",
		func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
			return client.NewClient(apiURL, apiKey, apiSecret)
		})()),
}

func testAccPreCheck(t *testing.T) {
	// common pre-checks for all acceptance tests
	if os.Getenv("GODADDY_API_KEY") == "" || os.Getenv("GODADDY_API_SECRET") == "" {
		t.Fatal("env vars GODADDY_API_KEY and GODADDY_API_SECRET must be set")
	}
}
