package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/1Password/terraform-provider-onepassword/v2/internal/onepassword"
	"github.com/1Password/terraform-provider-onepassword/v2/internal/onepassword/model"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &OnePasswordVaultResource{}
var _ resource.ResourceWithImportState = &OnePasswordVaultResource{}

func NewOnePasswordVaultResource() resource.Resource {
	return &OnePasswordVaultResource{}
}

// OnePasswordVaultResource defines the resource implementation.
type OnePasswordVaultResource struct {
	client onepassword.Client
}

// OnePasswordVaultResourceModel describes the resource data model.
type OnePasswordVaultResourceModel struct {
	ID          types.String `tfsdk:"id"`
	UUID        types.String `tfsdk:"uuid"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func (r *OnePasswordVaultResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault"
}

func (r *OnePasswordVaultResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a 1Password Vault.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The Terraform resource identifier for this vault in the format `vaults/<vault_id>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "The UUID of the vault.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the vault.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the vault.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
		},
	}
}

func (r *OnePasswordVaultResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(onepassword.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected onepassword.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *OnePasswordVaultResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan OnePasswordVaultResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vault := &model.Vault{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}

	createdVault, err := r.client.CreateVault(ctx, vault)
	if err != nil {
		resp.Diagnostics.AddError(
			"1Password Vault create error",
			fmt.Sprintf("Error creating 1Password vault, got error: %s", err),
		)
		return
	}

	plan.ID = types.StringValue(vaultTerraformID(createdVault))
	plan.UUID = types.StringValue(createdVault.ID)
	plan.Name = types.StringValue(createdVault.Name)
	plan.Description = types.StringValue(createdVault.Description)

	tflog.Trace(ctx, "created a vault resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OnePasswordVaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state OnePasswordVaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vaultUUID := vaultUUIDFromTerraformID(state.ID.ValueString())
	vault, err := r.client.GetVault(ctx, vaultUUID)
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"1Password Vault read error",
			fmt.Sprintf("Could not get vault '%s', got error: %s", vaultUUID, err),
		)
		return
	}

	state.ID = types.StringValue(vaultTerraformID(vault))
	state.UUID = types.StringValue(vault.ID)
	state.Name = types.StringValue(vault.Name)
	state.Description = types.StringValue(vault.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *OnePasswordVaultResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan OnePasswordVaultResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vault := &model.Vault{
		ID:          plan.UUID.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}

	updatedVault, err := r.client.UpdateVault(ctx, vault)
	if err != nil {
		resp.Diagnostics.AddError(
			"1Password Vault update error",
			fmt.Sprintf("Could not update vault '%s', got error: %s", plan.UUID.ValueString(), err),
		)
		return
	}

	plan.ID = types.StringValue(vaultTerraformID(updatedVault))
	plan.UUID = types.StringValue(updatedVault.ID)
	plan.Name = types.StringValue(updatedVault.Name)
	plan.Description = types.StringValue(updatedVault.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OnePasswordVaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state OnePasswordVaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteVault(ctx, state.UUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"1Password Vault delete error",
			fmt.Sprintf("Could not delete vault '%s', got error: %s", state.UUID.ValueString(), err),
		)
		return
	}
}

func (r *OnePasswordVaultResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	vaultUUID := req.ID
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), fmt.Sprintf("vaults/%s", vaultUUID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("uuid"), vaultUUID)...)
}

func vaultUUIDFromTerraformID(tfID string) string {
	elements := strings.Split(tfID, "/")
	if len(elements) != 2 {
		return ""
	}
	return elements[1]
}
