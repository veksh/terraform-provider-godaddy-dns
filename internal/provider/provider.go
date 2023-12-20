// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// https://pkg.go.dev/github.com/hashicorp/terraform-plugin-framework/provider
var _ provider.Provider = &GoDaddyDNSProvider{}

type GoDaddyDNSProvider struct {
	// "dev" for local testing, "test" for acceptance tests, "v1.2.3" for prod
	version string
	// api client is stored in ResourceData in Configure
	// apiClient apiClient
}

func (p *GoDaddyDNSProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	// common prefix for resources
	resp.TypeName = "godaddy-dns"
	// set in configure
	resp.Version = p.version
}

// have to match schema
type GoDaddyDNSProviderModel struct {
	APIKey    types.String `tfsdk:"api_key"`
	APISecret types.String `tfsdk:"api_secret"`
}

func (p *GoDaddyDNSProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "GoDaddy API key",
				Required:            false,
			},
			"api_secret": schema.StringAttribute{
				MarkdownDescription: "GoDaddy API secret",
				Required:            false,
			},
		},
		// also: Blocks
	}
}

func (p *GoDaddyDNSProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {

	var confData GoDaddyDNSProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &confData)...)

	apiKey := os.Getenv("GODADDY_API_KEY")
	if !confData.APIKey.IsNull() {
		apiKey = confData.APIKey.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key Configuration",
			"While configuring the provider, the API key was not found in "+
				"the GODADDY_API_KEY environment variable or provider "+
				"configuration block api_key attribute.",
		)
	}
	apiSecret := os.Getenv("GODADDY_API_SECRET")
	if !confData.APISecret.IsNull() {
		apiSecret = confData.APISecret.ValueString()
	}
	if apiSecret == "" {
		resp.Diagnostics.AddError(
			"Missing API Secret Configuration",
			"While configuring the provider, the API secret was not found in "+
				"the GODADDY_API_SECRET environment variable or provider "+
				"configuration block api_secret attribute.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// err := makeClient()
	// if err != nil {
	// 	resp.Diagnostics.AddError("failed to create API client", err.Error())
	// 	return
	// }
	client := http.DefaultClient
	resp.ResourceData = client
}

func (p *GoDaddyDNSProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewExampleResource,
	}
}

func (p *GoDaddyDNSProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &GoDaddyDNSProvider{
			version: version,
		}
	}
}
