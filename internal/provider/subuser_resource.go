package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &subuserResource{}
	_ resource.ResourceWithConfigure   = &subuserResource{}
	_ resource.ResourceWithImportState = &subuserResource{}
)

// NewSubuserResource is a helper function to simplify the provider implementation.
func NewSubuserResource() resource.Resource {
	return &subuserResource{}
}

// subuserResource is the resource implementation.
type subuserResource struct {
	client *admin.API
}

// Configure implements resource.ResourceWithConfigure.
func (r *subuserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*admin.API)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *admin.API, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Metadata returns the resource type name.
func (r *subuserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subuser"
}

// Schema defines the schema for the resource.
func (r *subuserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				Required: true,
			},
			"subuser": schema.StringAttribute{
				Required: true,
			},
			"access": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(admin.SubuserAccessNone),
						string(admin.SubuserAccessRead),
						string(admin.SubuserAccessReadWrite),
						string(admin.SubuserAccessWrite),
						string(admin.SubuserAccessFull),
					),
				},
			},
		},
	}
}

type subuserResourceModel struct {
	UserID  types.String `tfsdk:"user_id"`
	Subuser types.String `tfsdk:"subuser"`
	Access  types.String `tfsdk:"access"`
}

// Read implements resource.Resource.
func (r *subuserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state subuserResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.GetUser(ctx, admin.User{ID: state.UserID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error fetching subuser",
			"Could not fetch user to get subuser: "+err.Error(),
		)
		return
	}

	var found bool
	var matchingSubuser admin.SubuserSpec
	for _, subuser := range user.Subusers {
		if strings.TrimPrefix(subuser.Name, user.ID+":") == state.Subuser.ValueString() {
			found = true
			matchingSubuser = subuser
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Missing subuser",
			"Could not find subuser",
		)
		return
	}

	matchingSubuser = mapSubuser(user.ID, matchingSubuser)

	state.UserID = types.StringValue(user.ID)
	state.Subuser = types.StringValue(matchingSubuser.Name)
	state.Access = types.StringValue(string(matchingSubuser.Access))

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// mapSubuser maps some fields of subuser so that they work with Terraform.
func mapSubuser(parentUser string, subuser admin.SubuserSpec) admin.SubuserSpec {
	// Map Access back to "write" form, as the returned values do not match and then cause terraform to detect changes.
	//
	// See https://pkg.go.dev/github.com/ceph/go-ceph/rgw/admin#SubuserAccess for details
	switch subuser.Access {
	case admin.SubuserAccessReplyNone:
		subuser.Access = admin.SubuserAccessNone
	case admin.SubuserAccessReplyRead:
		subuser.Access = admin.SubuserAccessRead
	case admin.SubuserAccessReplyWrite:
		subuser.Access = admin.SubuserAccessWrite
	case admin.SubuserAccessReplyReadWrite:
		subuser.Access = admin.SubuserAccessReadWrite
	case admin.SubuserAccessReplyFull:
		subuser.Access = admin.SubuserAccessFull
	}

	// remove "<user>:" prefix in subuser name
	subuser.Name = strings.TrimPrefix(subuser.Name, parentUser+":")

	return subuser
}

// ImportState implements resource.ResourceWithImportState.
func (r *subuserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid subuser reference",
			"Subuser must be of format <user>:<subuser>",
		)
		return
	}

	user, err := r.client.GetUser(ctx, admin.User{ID: parts[0]})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error fetching user",
			"Could not fetch user: "+err.Error(),
		)
	}

	var found bool
	var matchingSubuser admin.SubuserSpec
	for _, subuser := range user.Subusers {
		if subuser.Name == req.ID {
			found = true
			matchingSubuser = subuser
			matchingSubuser.Name = strings.TrimPrefix(subuser.Name, user.ID+":")
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Missing subuser",
			"Could not find subuser",
		)
		return
	}

	matchingSubuser = mapSubuser(user.ID, matchingSubuser)

	var state subuserResourceModel
	state.UserID = types.StringValue(user.ID)
	state.Subuser = types.StringValue(matchingSubuser.Name)
	state.Access = types.StringValue(string(matchingSubuser.Access))

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Create implements resource.Resource.
func (r *subuserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan subuserResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	generateKey := false // do not auto-generate keys, they are managed using the radosgw_key resource
	newSubuser := admin.SubuserSpec{
		Name:        plan.Subuser.ValueString(),
		Access:      admin.SubuserAccess(plan.Access.ValueString()),
		GenerateKey: &generateKey,
	}

	err := r.client.CreateSubuser(ctx, admin.User{ID: plan.UserID.ValueString()}, newSubuser)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating subuser",
			"Could not create subuser, unexpected error: "+err.Error(),
		)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update implements resource.Resource.
func (r *subuserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan subuserResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	modifiedSubuser := admin.SubuserSpec{
		Name:   plan.Subuser.ValueString(),
		Access: admin.SubuserAccess(plan.Access.ValueString()),
	}

	err := r.client.ModifySubuser(ctx, admin.User{ID: plan.UserID.ValueString()}, modifiedSubuser)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating subuser",
			"Could not create subuser, unexpected error: "+err.Error(),
		)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete implements resource.Resource.
func (r *subuserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state subuserResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.RemoveSubuser(ctx, admin.User{ID: state.UserID.ValueString()}, admin.SubuserSpec{Name: state.Subuser.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error removing subuser",
			fmt.Sprintf("Could not remove subuser %q: %s", state.Subuser.ValueString(), err),
		)
		return
	}
}
