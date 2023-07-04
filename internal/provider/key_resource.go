package provider

import (
	"context"
	"fmt"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &keyResource{}
	_ resource.ResourceWithConfigure   = &keyResource{}
	_ resource.ResourceWithImportState = &keyResource{}
)

// NewKeyResource is a helper function to simplify the provider implementation.
func NewKeyResource() resource.Resource {
	return &keyResource{}
}

// keyResource is the resource implementation.
type keyResource struct {
	client *admin.API
}

// Configure implements resource.ResourceWithConfigure
func (r *keyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *keyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_key"
}

// Schema defines the schema for the resource.
func (r *keyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"user": schema.StringAttribute{
				Required: true,
			},
			// "subuser": schema.StringAttribute{
			// 	Optional: true,
			// },
			"access_key": schema.StringAttribute{
				Optional:  true,
				Computed:  true,
				Sensitive: true,
			},
			"secret_key": schema.StringAttribute{
				Optional:  true,
				Computed:  true,
				Sensitive: true,
			},
		},
	}
}

type keyResourceModel struct {
	User      types.String `tfsdk:"user"`
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
}

// Create creates the resource and sets the initial Terraform state.
func (r *keyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan keyResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.GetUser(ctx, admin.User{ID: plan.User.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error fetching user",
			"Could not fetch user: "+err.Error(),
		)
		return
	}
	seen := make(map[string]bool, len(user.Keys))
	for _, key := range user.Keys {
		seen[key.AccessKey] = true
	}

	newKey := admin.UserKeySpec{
		User:      plan.User.ValueString(),
		AccessKey: plan.AccessKey.ValueString(),
		SecretKey: plan.SecretKey.ValueString(),

		UID:     plan.User.ValueString(),
		KeyType: "s3",
	}
	if newKey.AccessKey == "" || newKey.SecretKey == "" {
		generateKey := true
		newKey.GenerateKey = &generateKey
	}

	keys, err := r.client.CreateKey(ctx, newKey)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating key",
			"Could not create key: "+err.Error(),
		)
		return
	}

	for _, key := range *keys {
		if seen[key.AccessKey] {
			continue
		}

		plan.User = types.StringValue(key.User)
		plan.AccessKey = types.StringValue(key.AccessKey)
		plan.SecretKey = types.StringValue(key.SecretKey)

		break
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *keyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state keyResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.GetUser(ctx, admin.User{ID: state.User.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error fetching user for key retrieval",
			"Could not fetch user for key retrieval: "+err.Error(),
		)
		return
	}

	var found bool
	var matchingKey admin.UserKeySpec
	for _, key := range user.Keys {
		if key.AccessKey == state.AccessKey.ValueString() && key.SecretKey == state.SecretKey.ValueString() {
			found = true
			matchingKey = key
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Key is missing from user",
			"Could not find matching key",
		)
		return
	}

	state.User = types.StringValue(matchingKey.User)
	state.AccessKey = types.StringValue(matchingKey.AccessKey)
	state.SecretKey = types.StringValue(matchingKey.SecretKey)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *keyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	users, err := r.client.GetUsers(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error fetching users for key import",
			"Could not fetch users for key import: "+err.Error(),
		)
		return
	}

	var found bool
	var matchingKey admin.UserKeySpec
	for _, userName := range *users {
		user, err := r.client.GetUser(ctx, admin.User{ID: userName})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error fetching user",
				"Could not fetch user: "+err.Error(),
			)
			return
		}

		for _, key := range user.Keys {
			if key.AccessKey == req.ID {
				found = true
				matchingKey = key
				break
			}
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Key is missing from user",
			"Could not find matching key",
		)
		return
	}

	var state keyResourceModel
	state.User = types.StringValue(matchingKey.User)
	state.AccessKey = types.StringValue(matchingKey.AccessKey)
	state.SecretKey = types.StringValue(matchingKey.SecretKey)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

}

// Update updates the resource and sets the updated Terraform state on success.
func (r *keyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan keyResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: implement Update

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *keyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// TODO: implement Delete
}
