// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                   = &subscriptionIntroductoryOfferScheduleResource{}
	_ resource.ResourceWithConfigure      = &subscriptionIntroductoryOfferScheduleResource{}
	_ resource.ResourceWithImportState    = &subscriptionIntroductoryOfferScheduleResource{}
	_ resource.ResourceWithValidateConfig = &subscriptionIntroductoryOfferScheduleResource{}
)

func NewSubscriptionIntroductoryOfferScheduleResource() resource.Resource {
	return &subscriptionIntroductoryOfferScheduleResource{}
}

type subscriptionIntroductoryOfferScheduleResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *subscriptionIntroductoryOfferScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_introductory_offer_schedule"
}

// Schema defines the schema for the resource.
func (r *subscriptionIntroductoryOfferScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sets every introductory offer of an auto-renewable subscription.\n\n" +
			"This is the bulk counterpart to `apple_subscription_introductory_offer`. Apple keeps one " +
			"introductory offer per territory, so a free trial in all 175 App Store storefronts through " +
			"that resource means 175 resources and 175 API calls on apply. This resource creates new " +
			"offers in a single `PATCH /v1/subscriptions/{id}` carrying them inline, and reads them all " +
			"back in one listing. Removing an offer and changing an `end_date` still take one request " +
			"each, because Apple publishes no bulk form for either.\n\n" +
			"An offer is matched to the one Apple holds by its territory, mode, duration, number of " +
			"periods and price point (plus `start_date` and `target_subscription_plan_type` when they are " +
			"set). A change to any of those removes the old offer and creates a new one; a change to " +
			"`end_date` alone updates the offer in place.\n\n" +
			"Apple records a start date on every offer, the day it was created when none was given, and " +
			"an import adopts it. After importing, set each `start_date` to Apple's value, or the first " +
			"plan proposes removing them — an update that sends no requests, since an unset start date " +
			"matches any.\n\n" +
			"~> **An `apple_subscription_availability` covering every territory has to exist first.** " +
			"Nothing in an offer references one, so declare the ordering with `depends_on`.\n\n" +
			"Do not manage the same subscription with both this resource and " +
			"`apple_subscription_introductory_offer`: every write here removes the offers it does not " +
			"list, so an offer only the other resource knows about would be removed behind its back. " +
			"`terraform destroy` deletes every offer this resource lists.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the subscription these offers belong to. A subscription has " +
					"no separate record for its set of offers, so this mirrors `subscription_id`.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_subscription` these offers apply to. Cannot be " +
					"changed after creation.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"offers": schema.SetNestedAttribute{
				MarkdownDescription: "The complete set of introductory offers the subscription should have, " +
					"at most one per territory at a time. A set, because Apple returns them in no " +
					"particular order.",
				Required: true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"territory_id": schema.StringAttribute{
							MarkdownDescription: introductoryOfferDescriptions["territory_id"],
							Required:            true,
							Validators:          []validator.String{TerritoryValidator},
						},
						"offer_mode": schema.StringAttribute{
							MarkdownDescription: introductoryOfferDescriptions["offer_mode"],
							Required:            true,
							Validators:          []validator.String{OfferModeValidator},
						},
						"duration": schema.StringAttribute{
							MarkdownDescription: introductoryOfferDescriptions["duration"],
							Required:            true,
							Validators:          []validator.String{OfferDurationValidator},
						},
						"number_of_periods": schema.Int64Attribute{
							MarkdownDescription: introductoryOfferDescriptions["number_of_periods"],
							Required:            true,
							Validators:          []validator.Int64{int64validator.AtLeast(1)},
						},
						"price_point_id": schema.StringAttribute{
							MarkdownDescription: introductoryOfferDescriptions["price_point_id"],
							Optional:            true,
						},
						"start_date": schema.StringAttribute{
							MarkdownDescription: introductoryOfferDescriptions["start_date"],
							Optional:            true,
							Validators:          []validator.String{StartDateValidator},
						},
						"end_date": schema.StringAttribute{
							MarkdownDescription: introductoryOfferDescriptions["end_date"],
							Optional:            true,
							Validators:          []validator.String{StartDateValidator},
						},
						"target_subscription_plan_type": schema.StringAttribute{
							MarkdownDescription: introductoryOfferDescriptions["target_subscription_plan_type"],
							Optional:            true,
							Validators:          []validator.String{PlanTypeValidator},
						},
					},
				},
			},
		},
	}
}

// ValidateConfig checks each offer's price point against its mode.
func (r *subscriptionIntroductoryOfferScheduleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// Read through types.Set: a set built by a for expression over a data
	// source is unknown at validation time, which a Go slice cannot hold.
	var set types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("offers"), &set)...)
	if resp.Diagnostics.HasError() || set.IsNull() || set.IsUnknown() {
		return
	}

	for _, element := range set.Elements() {
		// A whole element can be unknown inside a known set, and a struct
		// cannot hold that. There is nothing to check in it yet; validation
		// runs again during plan, once it is known.
		if element.IsUnknown() || element.IsNull() {
			continue
		}
		object, ok := element.(types.Object)
		if !ok {
			resp.Diagnostics.AddAttributeError(path.Root("offers"), "Unexpected Offer Type",
				fmt.Sprintf("Expected each offer to be an object, got %T. Please report this issue to the provider developers.", element))
			return
		}

		var offer subscriptionIntroductoryOfferScheduleEntry
		resp.Diagnostics.Append(object.As(ctx, &offer, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		if offer.OfferMode.IsUnknown() || offer.PricePointID.IsUnknown() {
			continue
		}
		if problem := introductoryOfferPricePointProblem(offer.OfferMode.ValueString(), !offer.PricePointID.IsNull()); problem != "" {
			resp.Diagnostics.AddAttributeError(
				path.Root("offers"),
				"Price Point Does Not Fit The Offer Mode",
				fmt.Sprintf("The %s offer in territory '%s': %s",
					offer.OfferMode.ValueString(), offer.TerritoryID.ValueString(), problem),
			)
		}
	}
}

// Create a new resource.
func (r *subscriptionIntroductoryOfferScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating subscription introductory offer schedule resource")

	var plan subscriptionIntroductoryOfferScheduleModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if touched := r.write(ctx, &plan, nil, &resp.Diagnostics); resp.Diagnostics.HasError() {
		if touched {
			r.recordCurrent(ctx, plan, &resp.State, &resp.Diagnostics)
		}
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *subscriptionIntroductoryOfferScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading subscription introductory offer schedule resource")

	var state subscriptionIntroductoryOfferScheduleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := state.SubscriptionID.ValueString()
	ctx = tflog.SetField(ctx, "subscription_id", subscriptionID)

	offers, err := r.client.GetSubscriptionIntroductoryOffers(subscriptionID, nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription not found, removing introductory offer schedule from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Subscription Introductory Offer Schedule",
			fmt.Sprintf("Could not read the introductory offers of subscription '%s': %s", subscriptionID, err.Error()),
		)
		return
	}

	// There is no record of its own to delete, so a subscription with no
	// offers left is the only way this resource can cease to exist.
	if len(offers) == 0 {
		tflog.Info(ctx, "Subscription has no introductory offers, removing schedule from state")
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(subscriptionID)
	state.Offers = reconcileIntroductoryOffers(state.Offers, offers)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update brings Apple's offers in line with the configured set.
func (r *subscriptionIntroductoryOfferScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating subscription introductory offer schedule resource")

	var plan subscriptionIntroductoryOfferScheduleModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior subscriptionIntroductoryOfferScheduleModel
	diags = req.State.Get(ctx, &prior)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if touched := r.write(ctx, &plan, prior.Offers, &resp.Diagnostics); resp.Diagnostics.HasError() {
		if touched {
			r.recordCurrent(ctx, plan, &resp.State, &resp.Diagnostics)
		}
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes every offer the schedule lists.
func (r *subscriptionIntroductoryOfferScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting subscription introductory offer schedule resource")

	var state subscriptionIntroductoryOfferScheduleModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := state.SubscriptionID.ValueString()

	offers, err := r.client.GetSubscriptionIntroductoryOffers(subscriptionID, nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription already deleted, taking its introductory offers with it")
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Subscription Introductory Offer Schedule",
			fmt.Sprintf("Could not read the introductory offers of subscription '%s': %s", subscriptionID, err.Error()),
		)
		return
	}

	// Only the offers state names are deleted. Read adds an offer created by hand
	// to state, so after a refresh that includes it.
	for _, kept := range planIntroductoryOffers(state.Offers, offers, nil).Keep {
		r.deleteOffer(kept.ID, &resp.Diagnostics)
	}
}

// Configure adds the provider configured client to the resource.
func (r *subscriptionIntroductoryOfferScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apple.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *apple.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// ImportState imports the introductory offers of a subscription.
//
// The import ID is the subscription ID: the offers are reachable only through
// the collection hanging off it, and this resource has no record of its own.
func (r *subscriptionIntroductoryOfferScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing subscription introductory offer schedule resource", map[string]interface{}{"import_id": req.ID})

	offers, err := r.client.GetSubscriptionIntroductoryOffers(req.ID, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Subscription Introductory Offer Schedule",
			fmt.Sprintf("Could not read the introductory offers of subscription '%s': %s\n\n"+
				"The import ID is the subscription ID — its introductory offers are only reachable "+
				"through the collection hanging off it.", req.ID, err.Error()),
		)
		return
	}

	if len(offers) == 0 {
		resp.Diagnostics.AddError(
			"Subscription Has No Introductory Offers",
			fmt.Sprintf("Subscription '%s' has no introductory offers to import, so there is nothing for "+
				"this resource to adopt. Apply the configuration instead of importing it.", req.ID),
		)
		return
	}

	state := subscriptionIntroductoryOfferScheduleModel{
		ID:             types.StringValue(req.ID),
		SubscriptionID: types.StringValue(req.ID),
		Offers:         reconcileIntroductoryOffers(nil, offers),
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// write brings Apple's offers in line with the plan, and reports whether it
// changed anything at Apple before any error it hit.
//
// prior is the state the plan replaces, nil on create. It decides which end
// dates a write may clear: see planIntroductoryOffers.
//
// Offers no longer configured are deleted first: a territory holds one
// introductory offer at a time, so a replacement offer in the same territory
// would be refused while the old one still existed. End dates are then changed
// in place, and whatever is new is created in one PATCH.
func (r *subscriptionIntroductoryOfferScheduleResource) write(ctx context.Context, plan *subscriptionIntroductoryOfferScheduleModel, prior []subscriptionIntroductoryOfferScheduleEntry, diags *diag.Diagnostics) (touched bool) {
	subscriptionID := plan.SubscriptionID.ValueString()
	ctx = tflog.SetField(ctx, "subscription_id", subscriptionID)

	existing, err := r.client.GetSubscriptionIntroductoryOffers(subscriptionID, nil)
	if err != nil {
		diags.AddError(
			"Error Writing Subscription Introductory Offer Schedule",
			fmt.Sprintf("Could not read the introductory offers of subscription '%s': %s", subscriptionID, err.Error()),
		)
		return false
	}

	changes := planIntroductoryOffers(plan.Offers, existing, managedEndDateOffers(prior, existing))
	tflog.Debug(ctx, "Writing subscription introductory offer schedule", map[string]interface{}{
		"keep":   len(changes.Keep),
		"remove": len(changes.Remove),
		"create": len(changes.Create),
	})

	for _, id := range changes.Remove {
		if r.deleteOffer(id, diags) {
			touched = true
		}
	}
	if diags.HasError() {
		return touched
	}

	keepIDs := make([]string, 0, len(changes.Keep))
	for _, kept := range changes.Keep {
		keepIDs = append(keepIDs, kept.ID)
		if !kept.EndDateChanged {
			continue
		}
		if _, err := r.client.UpdateSubscriptionIntroductoryOfferEndDate(kept.ID, stringOrNil(kept.Entry.EndDate), nil); err != nil {
			diags.AddError(
				"Error Writing Subscription Introductory Offer Schedule",
				fmt.Sprintf("Could not set the end date of the introductory offer in territory '%s': %s",
					kept.Entry.TerritoryID.ValueString(), err.Error()),
			)
			return touched
		}
		touched = true
	}

	if len(changes.Create) > 0 {
		inputs := make([]apple.SubscriptionIntroductoryOfferInput, 0, len(changes.Create))
		for _, entry := range changes.Create {
			inputs = append(inputs, introductoryOfferInput(entry))
		}

		if _, err := r.client.AddSubscriptionIntroductoryOffers(subscriptionID, keepIDs, inputs, nil); err != nil {
			addIntroductoryOfferWriteError(diags, subscriptionID, err,
				fmt.Sprintf("Could not create %d introductory offer(s) for subscription '%s'", len(inputs), subscriptionID))
			tflog.Error(ctx, "Failed to write subscription introductory offer schedule", map[string]interface{}{"error": err.Error()})
			return touched
		}
	}

	plan.ID = types.StringValue(subscriptionID)
	return true
}

// recordCurrent writes what Apple holds after a write that failed partway.
//
// Offers are deleted before their replacements are created, because a
// territory holds one at a time, so a create that fails afterwards leaves
// Apple with fewer offers than either the prior state or the plan describes.
// Recording the plan would claim offers that do not exist; keeping the prior
// state would claim ones already deleted. The next plan then shows exactly
// what is missing. A subscription left with no offers at all is recorded as
// gone, the way Read would find it.
func (r *subscriptionIntroductoryOfferScheduleResource) recordCurrent(ctx context.Context, plan subscriptionIntroductoryOfferScheduleModel, state *tfsdk.State, diags *diag.Diagnostics) {
	subscriptionID := plan.SubscriptionID.ValueString()

	offers, err := r.client.GetSubscriptionIntroductoryOffers(subscriptionID, nil)
	if err != nil {
		diags.AddWarning(
			"Could Not Record Introductory Offers After A Failed Write",
			fmt.Sprintf("Reading the introductory offers of subscription '%s' after the failed write also "+
				"failed: %s\n\nRun terraform plan -refresh-only to see what Apple now holds.", subscriptionID, err.Error()),
		)
		return
	}

	if len(offers) == 0 {
		state.RemoveResource(ctx)
		return
	}

	current := subscriptionIntroductoryOfferScheduleModel{
		ID:             types.StringValue(subscriptionID),
		SubscriptionID: plan.SubscriptionID,
		Offers:         reconcileIntroductoryOffers(plan.Offers, offers),
	}
	diags.Append(state.Set(ctx, current)...)
}

// deleteOffer deletes one offer, treating one already gone as deleted, and
// reports whether Apple deleted it.
func (r *subscriptionIntroductoryOfferScheduleResource) deleteOffer(id string, diags *diag.Diagnostics) bool {
	err := r.client.DeleteSubscriptionIntroductoryOffer(id, nil)
	if err == nil {
		return true
	}
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
		return false
	}

	diags.AddError(
		"Error Deleting Subscription Introductory Offer",
		fmt.Sprintf("Could not delete introductory offer '%s': %s", id, err.Error()),
	)
	return false
}
