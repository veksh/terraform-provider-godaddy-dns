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
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &RecordResource{}
	_ resource.ResourceWithConfigure   = &RecordResource{}
	_ resource.ResourceWithImportState = &RecordResource{}
)

type tfDNSRecord struct {
	Domain   types.String `tfsdk:"domain"`
	Type     types.String `tfsdk:"type"`
	Name     types.String `tfsdk:"name"`
	Data     types.String `tfsdk:"data"`
	TTL      types.Int64  `tfsdk:"ttl"`
	Priority types.Int64  `tfsdk:"priority"`
}

// add record fields to context; export TF_LOG=debug to view
func setLogCtx(ctx context.Context, tfRec tfDNSRecord, op string) context.Context {
	ctx = tflog.SetField(ctx, "domain", tfRec.Domain.ValueString())
	ctx = tflog.SetField(ctx, "type", tfRec.Type.ValueString())
	ctx = tflog.SetField(ctx, "name", tfRec.Name.ValueString())
	ctx = tflog.SetField(ctx, "operation", op)
	return ctx
}

// convert from terraform data model into api data model
func tf2model(tfData tfDNSRecord) (model.DNSDomain, model.DNSRecord) {
	return model.DNSDomain(tfData.Domain.ValueString()),
		model.DNSRecord{
			Name:     model.DNSRecordName(tfData.Name.ValueString()),
			Type:     model.DNSRecordType(tfData.Type.ValueString()),
			Data:     model.DNSRecordData(tfData.Data.ValueString()),
			TTL:      model.DNSRecordTTL(tfData.TTL.ValueInt64()),
			Priority: model.DNSRecordPrio(tfData.Priority.ValueInt64()),
		}
}

// RecordResource defines the implementation of GoDaddy DNS RR
type RecordResource struct {
	client model.DNSApiClient
}

func NewRecordResource() resource.Resource {
	return &RecordResource{}
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
					// TODO: implement SRV management :)
					stringvalidator.Any(
						stringvalidator.OneOf([]string{"A", "AAAA", "CNAME", "NS", "TXT"}...),
						stringvalidator.All(
							// mx requires priority
							stringvalidator.OneOf([]string{"MX"}...),
							stringvalidator.AlsoRequires(path.Expressions{
								path.MatchRoot("priority"),
							}...),
						),
					),
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
			"priority": schema.Int64Attribute{
				MarkdownDescription: "Priority for MX and SRV, def 0",
				Optional:            true,
			},
		},
	}
}

func (r *RecordResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// or it will panic on none
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(model.DNSApiClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Internal error: expected *model.DNSApiClient, got: %T", req.ProviderData),
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

	ctx = setLogCtx(ctx, planData, "create")

	apiDomain, apiRecPlan := tf2model(planData)
	// add: does not check (read) if creating w/o prior state
	// and so will fail on uniqueness violation (e.g. if CNAME already
	// exists, even with the same name); ok for us -- let API do checking
	err := r.client.AddRecords(ctx, apiDomain, []model.DNSRecord{apiRecPlan})

	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to create record: %s", err))
		return
	}

	tflog.Info(ctx, "DNS record created")

	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *RecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var priorData tfDNSRecord
	resp.Diagnostics.Append(req.State.Get(ctx, &priorData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = setLogCtx(ctx, priorData, "read")

	apiDomain, apiRecState := tf2model(priorData)

	apiAllRecs, err := r.client.GetRecords(ctx, apiDomain, apiRecState.Type, apiRecState.Name)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Reading DNS records: query failed: %s", err))
		return
	}
	if numRecs := len(apiAllRecs); numRecs == 0 {
		tflog.Debug(ctx, "Reading DNS record: currently absent")
		// no resource found: mb ok or need to re-create
		resp.State.RemoveResource(ctx)
		return
	} else {
		tflog.Info(ctx, fmt.Sprintf(
			"Reading DNS record: got %d answers", numRecs))
		// meaning of "match" is different between types
		//  - for A, AAAA, and CNAME (and SOA), there could be only 1 records
		//    with a given name in domain
		//  - for TXT, MX and NS there could be several, have to match by data
		//    - MXes could have different priorities; in theory, MX 0 and MX 10
		//      could point to the same "data", but lets think that it is a
		//      preversion and replace it with one :)
		//    - TXT and NS for same name could differ only in TTL
		//  - for SRV there are a slew of PK attrs, will do it later
		numFound := 0
		for _, rec := range apiAllRecs {
			tflog.Debug(ctx, fmt.Sprintf(
				"Got DNS record: data %s, prio %d, ttl %d", rec.Data, rec.Priority, rec.TTL))
			if rec.SameKey(apiRecState) {
				tflog.Info(ctx, "matching DNS record found")
				priorData.Data = types.StringValue(string(rec.Data))
				priorData.TTL = types.Int64Value(int64(rec.TTL))
				switch rec.Type {
				case "TXT", "MX", "NS":
					priorData.Priority = types.Int64Value(int64(rec.Priority))
				}
				numFound += 1
			}
		}
		if numFound == 0 {
			tflog.Info(ctx, "no matching record found")
		} else {
			if numFound > 1 {
				tflog.Warn(ctx, "more than one maching record found, using last")
			}
		}
	}

	tflog.Info(ctx, "DNS records read")

	resp.Diagnostics.Append(resp.State.Set(ctx, &priorData)...)
}

func (r *RecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData tfDNSRecord
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = setLogCtx(ctx, planData, "update")

	apiDomain, apiRecPlan := tf2model(planData)

	apiUpdateRec := model.DNSRecord{
		Data: apiRecPlan.Data,
		TTL:  apiRecPlan.TTL,
	}

	// simple case of A/CNAME: only one record, so it is safe to replace
	// TODO: implement read + modify + write for TXT etc
	err := r.client.SetRecords(ctx, apiDomain, apiRecPlan.Type, apiRecPlan.Name, []model.DNSRecord{apiUpdateRec})

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

	ctx = setLogCtx(ctx, priorData, "delete")

	apiDomain, apiRecState := tf2model(priorData)

	err := r.client.DelRecords(ctx, apiDomain, apiRecState.Type, apiRecState.Name)

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
			fmt.Sprintf("Expected import identifier format: domain:TYPE:name:data"+
				"like mydom.com:CNAME:www.subdom:www.other.com. Got: %q", req.ID),
		)
		return
	}

	for i, f := range []string{"domain", "type", "name", "data"} {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(f), idParts[i])...)
	}
}
