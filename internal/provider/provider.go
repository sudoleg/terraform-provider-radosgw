// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/ceph/go-ceph/rgw/admin"
)

// Ensure RadosgwProvider satisfies various provider interfaces.
var _ provider.Provider = &radosgwProvider{}

// radosgwProvider defines the provider implementation.
type radosgwProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// radosgwProviderModel describes the provider data model.
type radosgwProviderModel struct {
	Endpoint        types.String `tfsdk:"endpoint"`
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
}

func (p *radosgwProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "radosgw"
	resp.Version = p.version
}

func (p *radosgwProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Radosgw admin endpoint url to use",
				Optional:            true,
			},
			"access_key_id": schema.StringAttribute{
				Optional: true,
			},
			"secret_access_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}

func (p *radosgwProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config radosgwProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if config.Endpoint.IsUnknown() { // TODO: what does "unknown" mean?
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Unknown radosgw admin endpoint",
			"Unknown radosgw admin endpoint url.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	var endpoint string
	accessKeyID := os.Getenv("ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("SECRET_ACCESS_KEY")

	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}
	if !config.AccessKeyID.IsNull() {
		accessKeyID = config.AccessKeyID.ValueString()
	}
	if !config.SecretAccessKey.IsNull() {
		secretAccessKey = config.SecretAccessKey.ValueString()
	}

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Missing radosgw admin endpoint url",
			"Radosgw admin endpoint url is missing or empty.",
		)
	}

	if accessKeyID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_key_id"),
			"Missing radosgw admin access key",
			"Radosgw admin access key is missing or empty.  Set it in the configuration or via the ACCESS_KEY_ID environment variable.",
		)
	}

	if secretAccessKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("secret_access_key"),
			"Missing radosgw admin access secret",
			"Radosgw admin access secret is missing or empty.  Set it in the configuration or via the SECRET_ACCESS_KEY environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "radosgw_endpoint", endpoint)
	ctx = tflog.SetField(ctx, "radosgw_access_key_id", accessKeyID)
	ctx = tflog.SetField(ctx, "radosgw_secret_access_key", secretAccessKey)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "radosgw_secret_access_key")
	tflog.Debug(ctx, "creating radosgw admin client")

	client, err := admin.New(endpoint, accessKeyID, secretAccessKey, http.DefaultClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create radosgw admin client",
			"An unexpected error happened creating the radosgw admin client.\n\nClient error: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "configured radosgw admin client", map[string]any{"success": true})
}

func (p *radosgwProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewUserResource,
		NewSubuserResource,
		NewKeyResource,
	}
}

func (p *radosgwProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewBucketsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &radosgwProvider{
			version: version,
		}
	}
}
