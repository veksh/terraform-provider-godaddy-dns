package main

// docs generation + dependencies moved to tools

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/client"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/provider"
)

var (
	// these will be set by the goreleaser configuration
	// to appropriate values for the compiled binary.
	// see https://goreleaser.com/cookbooks/using-main.version/
	version string = "dev"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/veksh/godaddy-dns",
		Debug:   debug,
	}

	apiClientFactory := func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
		return client.NewClient(apiURL, apiKey, apiSecret)
	}

	err := providerserver.Serve(context.Background(), provider.New(version, apiClientFactory), opts)

	if err != nil {
		log.Fatal(err.Error())
	}
}
