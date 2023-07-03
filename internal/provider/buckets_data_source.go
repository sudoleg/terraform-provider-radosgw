// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &bucketsDataSource{}

func NewBucketsDataSource() datasource.DataSource {
	return &bucketsDataSource{}
}

// bucketsDataSource defines the data source implementation.
type bucketsDataSource struct {
	client *admin.API
}

func (d *bucketsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_buckets"
}

func (d *bucketsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Buckets data source",

		Attributes: map[string]schema.Attribute{
			"buckets": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"bucket": schema.StringAttribute{
							Computed: true,
						},
						"owner": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func (d *bucketsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = client
}

type bucketsDataSourceModel struct {
	Buckets []bucketsModel `tfsdk:"buckets"`
}

type bucketsModel struct {
	Bucket types.String `tfsdk:"bucket"`
	Owner  types.String `tfsdk:"owner"`
}

func (d *bucketsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state bucketsDataSourceModel

	buckets, err := d.client.ListBucketsWithStat(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to list buckets",
			err.Error(),
		)
		return
	}

	// Map response body to model
	for _, bucket := range buckets {
		bucketState := bucketsModel{
			Bucket: types.StringValue(bucket.Bucket),
			Owner:  types.StringValue(bucket.Owner),
		}

		state.Buckets = append(state.Buckets, bucketState)
	}

	// Set state
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
