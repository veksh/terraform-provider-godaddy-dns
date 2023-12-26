package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

// testing api at ote is useless
const GODADDY_API_URL = "https://api.godaddy.com"

// https://pkg.go.dev/github.com/hashicorp/terraform-plugin-framework/provider
var _ provider.Provider = &GoDaddyDNSProvider{}

type APIClientFactory func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error)

type GoDaddyDNSProvider struct {
	// "dev" for local testing, "test" for acceptance tests, "v1.2.3" for prod
	version       string
	clientFactory APIClientFactory
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
		// full documentation: conf in templates
		// see https://github.com/hashicorp/terraform-provider-tls/blob/main/templates/index.md.tmpl
		MarkdownDescription: "GoDaddy DNS provider",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "GoDaddy API key",
				Optional:            true,
				Sensitive:           true,
			},
			"api_secret": schema.StringAttribute{
				MarkdownDescription: "GoDaddy API secret",
				Optional:            true,
				Sensitive:           true,
			},
		},
		// also: Blocks
	}
}

func (p *GoDaddyDNSProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {

	var confData GoDaddyDNSProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &confData)...)

	apiKey := os.Getenv("GODADDY_API_KEY")
	if !(confData.APIKey.IsUnknown() || confData.APIKey.IsNull()) {
		apiKey = confData.APIKey.ValueString()
	}
	if apiKey == "" {
		// be more specific than resp.Diagnostics.AddError(...)
		resp.Diagnostics.AddAttributeError(path.Root("api_key"),
			"Missing API Key Configuration",
			"While configuring the provider, the API key was not found in "+
				"the GODADDY_API_KEY environment variable or provider "+
				"configuration block api_key attribute.",
		)
	}
	apiSecret := os.Getenv("GODADDY_API_SECRET")
	if !(confData.APISecret.IsUnknown() || confData.APISecret.IsNull()) {
		apiSecret = confData.APISecret.ValueString()
	}
	if apiSecret == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_secret"),
			"Missing API Secret Configuration",
			"While configuring the provider, the API secret was not found in "+
				"the GODADDY_API_SECRET environment variable or provider "+
				"configuration block api_secret attribute.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client, err := p.clientFactory(GODADDY_API_URL, apiKey, apiSecret)
	if err != nil {
		resp.Diagnostics.AddError("failed to create API client", err.Error())
		return
	}

	resp.ResourceData = client
}

func (p *GoDaddyDNSProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRecordResource,
	}
}

func (p *GoDaddyDNSProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
	// return []func() datasource.DataSource{
	// 	NewEmptyDataSource,
	// }
}

func New(version string, clientFactory APIClientFactory) func() provider.Provider {
	return func() provider.Provider {
		return &GoDaddyDNSProvider{
			version:       version,
			clientFactory: clientFactory,
		}
	}
}
