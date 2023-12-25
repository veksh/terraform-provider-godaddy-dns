package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/client"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &RecordResource{}
	_ resource.ResourceWithConfigure   = &RecordResource{}
	_ resource.ResourceWithImportState = &RecordResource{}
)

type tfDNSRecord struct {
	Domain types.String `tfsdk:"domain"`
	Type   types.String `tfsdk:"type"`
	Name   types.String `tfsdk:"name"`
	Data   types.String `tfsdk:"data"`
	TTL    types.Int64  `tfsdk:"ttl"`
	ID     types.String `tfsdk:"id"`
}

func NewRecordResource() resource.Resource {
	return &RecordResource{}
}

// RecordResource defines the resource implementation.
type RecordResource struct {
	client *client.Client
}

func (r *RecordResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (r *RecordResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "GoDaddy DNS record",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				MarkdownDescription: "managed domain (top-level)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "type: A, CNAME etc",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"A", "AAAA", "CNAME", "MX", "NS", "SOA", "SRV", "TXT"}...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "name (part of fqdn), may include `.` for sub-domains",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"data": schema.StringAttribute{
				MarkdownDescription: "contents: target for CNAME, ip address for A etc",
				Required:            true,
			},
			"ttl": schema.Int64Attribute{
				MarkdownDescription: "TTL, > 600 < 86400, def 3600",
				Required:            false,
				Computed:            true, // must be computed to use a default
				Default:             int64default.StaticInt64(3600),
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Artificial ID attribute for RR (domain:name:type:data), " +
					"used internally by Terraform and should not be used in configuration",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			/*
				"defaulted": schema.StringAttribute{
					MarkdownDescription: "Example configurable attribute with default value",
					Optional:            true,
					Computed:            true,
					Default:             stringdefault.StaticString("example value when not configured"),
				},
				"id": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "Example identifier",
					PlanModifiers: []planmodifier.String{
						stringplanmodifier.UseStateForUnknown(),
					},
				},
			*/
		},
	}
}

// add record fields to context; export TF_LOG=debug to view
func setLogCtx(ctx context.Context, tfRec tfDNSRecord) context.Context {
	ctx = tflog.SetField(ctx, "domain", tfRec.Domain.ValueString())
	ctx = tflog.SetField(ctx, "type", tfRec.Type.ValueString())
	ctx = tflog.SetField(ctx, "name", tfRec.Name.ValueString())
	// so could search for logtype=custom in logs
	ctx = tflog.SetField(ctx, "logtype", "custom")
	return ctx
}

func makeID(tfRec tfDNSRecord) string {
	return fmt.Sprintf("%s:%s:%s:%s",
		tfRec.Domain.ValueString(),
		tfRec.Type.ValueString(),
		tfRec.Name.ValueString(),
		tfRec.Data.ValueString())
}

func (r *RecordResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// or it will panic on none
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *RecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var planData tfDNSRecord
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = setLogCtx(ctx, planData)

	apiRec := model.DNSRecord{
		Name: model.DNSRecordName(planData.Name.ValueString()),
		Type: model.DNSRecordType(planData.Type.ValueString()),
		Data: model.DNSRecordData(planData.Data.ValueString()),
		TTL:  model.DNSRecordTTL(planData.TTL.ValueInt64()),
	}
	apiDomain := model.DNSDomain(planData.Domain.ValueString())
	// add: does not check (read) if creating w/o prior state
	// and so will fail on uniqueness violation (e.g. if CNAME already
	// exists, even with the same name)
	err := r.client.AddRecords(ctx, apiDomain, []model.DNSRecord{apiRec})

	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to create record: %s", err))
		return
	}

	tflog.Info(ctx, "DNS record created")
	// 	map[string]any{"domain": planData.Domain.ValueString(),}
	planData.ID = types.StringValue(makeID(planData))

	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *RecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var priorData tfDNSRecord
	resp.Diagnostics.Append(req.State.Get(ctx, &priorData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = setLogCtx(ctx, priorData)

	apiDomain := model.DNSDomain(priorData.Domain.ValueString())
	apiRecType := model.DNSRecordType(priorData.Type.ValueString())
	apiRecName := model.DNSRecordName(priorData.Name.ValueString())

	apiRecs, err := r.client.GetRecords(ctx, apiDomain, apiRecType, apiRecName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Reading DNS records: query failed: %s", err))
		return
	}
	if numRecs := len(apiRecs); numRecs == 0 {
		tflog.Debug(ctx, "Reading DNS record: currently absent")
		// no resource found: mb ok or need to re-create
		resp.State.RemoveResource(ctx)
		return
	} else {
		tflog.Debug(ctx, fmt.Sprintf(
			"Reading DNS record: found %d matching records", numRecs))
		// meaning of "match" is different between types
		//  - for A, AAAA, and CNAME (and SOA), there could be only 1 records
		//    with a given name in domain
		//  - for TXT there could be several, have to match by name
		//  - for MX there could be several, have to match by name
		//    - they could have different priorities; in theory, MX 0 and MX 10
		//      could point to the same "name", but lets think that it is a
		//      preversion :)
		//  - for SRV there could several records with the same fields and
		//    different names for e.g. load balancing
		for _, rec := range apiRecs {
			tflog.Debug(ctx, fmt.Sprintf(
				"Reading DNS record: data %s, prio %d, ttl %d",
				rec.Data, rec.Priority, rec.TTL))
			if rec.Type == "A" || rec.Type == "CNAME" || rec.Type == "AAAA" {
				// TODO: ok to always update? or need to check for match?
				priorData.Data = types.StringValue(string(rec.Data))
				priorData.TTL = types.Int64Value(int64(rec.TTL))
				priorData.ID = types.StringValue(makeID(priorData))
			}
			// will deal with more complex types later :)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &priorData)...)
}

func (r *RecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData tfDNSRecord
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = setLogCtx(ctx, planData)

	apiRec := model.DNSRecord{
		Data: model.DNSRecordData(planData.Data.ValueString()),
		TTL:  model.DNSRecordTTL(planData.TTL.ValueInt64()),
	}
	apiDomain := model.DNSDomain(planData.Domain.ValueString())
	apiName := model.DNSRecordName(planData.Name.ValueString())
	apiType := model.DNSRecordType(planData.Type.ValueString())

	// simple case of A/CNAME: only one record, so it is safe to replace
	// TODO: implement read + modify + write for TXT etc
	err := r.client.SetRecords(ctx, apiDomain, apiType, apiName, []model.DNSRecord{apiRec})

	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Updating DNS record: query failed: %s", err))
		return
	}

	tflog.Info(ctx, "DNS record updated")

	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *RecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var priorData tfDNSRecord

	resp.Diagnostics.Append(req.State.Get(ctx, &priorData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = setLogCtx(ctx, priorData)

	apiDomain := model.DNSDomain(priorData.Domain.ValueString())
	apiName := model.DNSRecordName(priorData.Name.ValueString())
	apiType := model.DNSRecordType(priorData.Type.ValueString())

	err := r.client.DelRecords(ctx, apiDomain, apiType, apiName)

	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Deleting DNS record: query failed: %s", err))
		return
	}

	tflog.Info(ctx, "DNS record deleted")
}

// terraform import godaddy-dns_record.new-cname domain:CNAME:_test:testing.com
func (r *RecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// resource.ImportStatePassthroughID(ctx, path.Root("data"), req, resp)

	// for some reason Terraform does not pass schema data to Read on import
	// either as a separate structure in ReadRequest or as defaults: if only
	// they were accessible, it would eliminate the need to pass anything here

	idParts := strings.Split(req.ID, ":")

	// mb check format and emptiness
	if len(idParts) != 4 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: domain:TYPE:name:data like mydom.com:CNAME:redir:www.other.com. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), idParts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), idParts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("data"), idParts[3])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
