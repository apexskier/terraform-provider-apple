// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                   = &subscriptionIntroductoryOfferResource{}
	_ resource.ResourceWithConfigure      = &subscriptionIntroductoryOfferResource{}
	_ resource.ResourceWithImportState    = &subscriptionIntroductoryOfferResource{}
	_ resource.ResourceWithValidateConfig = &subscriptionIntroductoryOfferResource{}
)

func NewSubscriptionIntroductoryOfferResource() resource.Resource {
	return &subscriptionIntroductoryOfferResource{}
}

type subscriptionIntroductoryOfferResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *subscriptionIntroductoryOfferResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription_introductory_offer"
}

// introductoryOfferDescriptions holds the attribute descriptions the singular
// resource and the schedule share.
var introductoryOfferDescriptions = map[string]string{
	"territory_id": "The three-letter Apple territory code the offer applies in, for example `USA`, `GBR` " +
		"or `EGY`. An introductory offer is always for one territory, and a territory holds one " +
		"introductory offer at a time.",
	"offer_mode": "What the customer pays during the offer: `FREE_TRIAL` (nothing), `PAY_AS_YOU_GO` (a " +
		"reduced price every billing period for `number_of_periods` periods) or `PAY_UP_FRONT` (a " +
		"reduced price once, for the whole `duration`).",
	"duration": "How long one offer period lasts: `THREE_DAYS`, `ONE_WEEK`, `TWO_WEEKS`, `ONE_MONTH`, " +
		"`TWO_MONTHS`, `THREE_MONTHS`, `SIX_MONTHS` or `ONE_YEAR`. Apple restricts the combinations: " +
		"for `PAY_AS_YOU_GO` it is the subscription's own period, and it validates the rest itself.",
	"number_of_periods": "How many `duration` periods the offer lasts. `1` for `FREE_TRIAL` and " +
		"`PAY_UP_FRONT`; the number of discounted billing periods for `PAY_AS_YOU_GO`.",
	"price_point_id": "The price point the customer pays during a `PAY_AS_YOU_GO` or `PAY_UP_FRONT` " +
		"offer. Required for those modes and rejected for `FREE_TRIAL`. Obtain it from the " +
		"`apple_subscription_price_points` data source for this subscription and territory.",
	"start_date": "The first day the offer is available to new subscribers, as a plain date in " +
		"`YYYY-MM-DD` form. Omit it for an offer that starts immediately; Apple then records the day " +
		"the offer was created as its start date.",
	"end_date": "The last day the offer is available to new subscribers, as a plain date in " +
		"`YYYY-MM-DD` form. Omit it for an offer with no end. This is the only attribute Apple allows " +
		"to change on an existing offer.",
	"target_subscription_plan_type": "Which plan the offer applies to: `MONTHLY` or `UPFRONT`. Omit it " +
		"for an ordinary subscription.",
}

// Schema defines the schema for the resource.
func (r *subscriptionIntroductoryOfferResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages one introductory offer of an auto-renewable subscription, in one territory.\n\n" +
			"An introductory offer is what a new subscriber pays at first — a free trial, a reduced price " +
			"per period, or a reduced price paid once — before the subscription's own price applies. " +
			"Apple keeps one record per territory.\n\n" +
			"Only `end_date` changes in place; every other attribute forces replacement, because Apple's " +
			"update request carries nothing else.\n\n" +
			"~> **Use `apple_subscription_introductory_offer_schedule` for more than a handful of " +
			"territories.** This resource is one `POST` per territory, so a free trial in all 175 " +
			"storefronts is 175 resources. The schedule writes them in one request. Do not point both " +
			"at the same subscription: the schedule removes any offer it does not list.\n\n" +
			"~> **An `apple_subscription_availability` covering the territory has to exist first.** " +
			"Nothing in an offer references one, so declare the ordering with `depends_on`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the offer.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `apple_subscription` this offer belongs to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"territory_id": schema.StringAttribute{
				MarkdownDescription: introductoryOfferDescriptions["territory_id"],
				Required:            true,
				Validators:          []validator.String{TerritoryValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"offer_mode": schema.StringAttribute{
				MarkdownDescription: introductoryOfferDescriptions["offer_mode"],
				Required:            true,
				Validators:          []validator.String{OfferModeValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"duration": schema.StringAttribute{
				MarkdownDescription: introductoryOfferDescriptions["duration"],
				Required:            true,
				Validators:          []validator.String{OfferDurationValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"number_of_periods": schema.Int64Attribute{
				MarkdownDescription: introductoryOfferDescriptions["number_of_periods"],
				Required:            true,
				Validators:          []validator.Int64{int64validator.AtLeast(1)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"price_point_id": schema.StringAttribute{
				MarkdownDescription: introductoryOfferDescriptions["price_point_id"],
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"start_date": schema.StringAttribute{
				MarkdownDescription: introductoryOfferDescriptions["start_date"] + "\n\n" +
					"Computed when it is not configured, so a start date Apple fills in is recorded " +
					"rather than planned away.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{StartDateValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"end_date": schema.StringAttribute{
				MarkdownDescription: introductoryOfferDescriptions["end_date"],
				Optional:            true,
				Validators:          []validator.String{EndDateValidator},
			},
			"target_subscription_plan_type": schema.StringAttribute{
				MarkdownDescription: introductoryOfferDescriptions["target_subscription_plan_type"] + "\n\n" +
					"Computed when it is not configured, because Apple may report a plan type on an " +
					"offer that never named one.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{PlanTypeValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// entry views the resource as a schedule entry, so both resources share the
// conversion to the client's input and the error wording.
func (m subscriptionIntroductoryOfferModel) entry() subscriptionIntroductoryOfferScheduleEntry {
	return subscriptionIntroductoryOfferScheduleEntry{
		TerritoryID:                m.TerritoryID,
		OfferMode:                  m.OfferMode,
		Duration:                   m.Duration,
		NumberOfPeriods:            m.NumberOfPeriods,
		PricePointID:               m.PricePointID,
		StartDate:                  m.StartDate,
		EndDate:                    m.EndDate,
		TargetSubscriptionPlanType: m.TargetSubscriptionPlanType,
	}
}

// ValidateConfig checks the price point against the offer mode.
func (r *subscriptionIntroductoryOfferResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config subscriptionIntroductoryOfferModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.OfferMode.IsUnknown() || config.PricePointID.IsUnknown() {
		return
	}
	if problem := introductoryOfferPricePointProblem(config.OfferMode.ValueString(), !config.PricePointID.IsNull()); problem != "" {
		resp.Diagnostics.AddAttributeError(path.Root("price_point_id"), "Price Point Does Not Fit The Offer Mode", problem)
	}
}

// Create a new resource.
func (r *subscriptionIntroductoryOfferResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating subscription introductory offer resource")

	var plan subscriptionIntroductoryOfferModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := plan.SubscriptionID.ValueString()
	ctx = tflog.SetField(ctx, "subscription_id", subscriptionID)
	ctx = tflog.SetField(ctx, "territory_id", plan.TerritoryID.ValueString())

	offer, err := r.client.CreateSubscriptionIntroductoryOffer(subscriptionID, introductoryOfferInput(plan.entry()), nil)
	if err != nil {
		addIntroductoryOfferWriteError(&resp.Diagnostics, subscriptionID, err,
			fmt.Sprintf("Could not create the %s introductory offer in territory '%s' for subscription '%s'",
				plan.OfferMode.ValueString(), plan.TerritoryID.ValueString(), subscriptionID))
		tflog.Error(ctx, "Failed to create subscription introductory offer", map[string]interface{}{"error": err.Error()})
		return
	}

	plan.ID = types.StringValue(offer.ID)
	resolveIntroductoryOfferComputed(&plan, offer)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
//
// Apple publishes no GET for a single introductory offer, so this lists the
// subscription's offers in the offer's territory and scans them.
func (r *subscriptionIntroductoryOfferResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading subscription introductory offer resource")

	var state subscriptionIntroductoryOfferModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "offer_id", state.ID.ValueString())

	offer, err := r.client.GetSubscriptionIntroductoryOffer(state.SubscriptionID.ValueString(), state.TerritoryID.ValueString(), state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription introductory offer not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Subscription Introductory Offer",
			fmt.Sprintf("Could not read introductory offer '%s' on subscription '%s': %s",
				state.ID.ValueString(), state.SubscriptionID.ValueString(), err.Error()),
		)
		return
	}

	// An end date Apple reports against a configuration that holds none is not
	// adopted, the rule apple_app_settings follows for its optional URLs.
	priorEndDate := state.EndDate
	refreshIntroductoryOffer(&state, offer)
	if priorEndDate.IsNull() {
		state.EndDate = priorEndDate
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update changes the end date, the only member of Apple's update request.
// Every other attribute forces replacement.
func (r *subscriptionIntroductoryOfferResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating subscription introductory offer resource")

	var plan subscriptionIntroductoryOfferModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.UpdateSubscriptionIntroductoryOfferEndDate(plan.ID.ValueString(), stringOrNil(plan.EndDate), nil); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Subscription Introductory Offer",
			fmt.Sprintf("Could not set the end date of introductory offer '%s': %s", plan.ID.ValueString(), err.Error()),
		)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *subscriptionIntroductoryOfferResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting subscription introductory offer resource")

	var state subscriptionIntroductoryOfferModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteSubscriptionIntroductoryOffer(state.ID.ValueString(), nil); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Subscription introductory offer already deleted")
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Subscription Introductory Offer",
			fmt.Sprintf("Could not delete introductory offer '%s': %s", state.ID.ValueString(), err.Error()),
		)
	}
}

// Configure adds the provider configured client to the resource.
func (r *subscriptionIntroductoryOfferResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an existing introductory offer.
//
// The composite "<subscription_id>/<offer_id>" form is the only one accepted:
// Apple has no endpoint that reads an offer by ID, so the parent subscription
// is what makes the offer findable at all.
func (r *subscriptionIntroductoryOfferResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing subscription introductory offer resource", map[string]interface{}{"import_id": req.ID})

	subscriptionID, offerID, found := strings.Cut(req.ID, "/")
	if !found || subscriptionID == "" || offerID == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Introductory offer import ID '%s' is not in the expected form.\n\n"+
				"Use \"<subscription_id>/<offer_id>\".\n\n"+
				"The subscription ID is required because Apple publishes no GET for a single "+
				"introductory offer — an offer is only reachable through the collection hanging off its "+
				"subscription.", req.ID),
		)
		return
	}

	offer, err := r.client.GetSubscriptionIntroductoryOffer(subscriptionID, "", offerID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Subscription Introductory Offer",
			fmt.Sprintf("Could not import introductory offer '%s' on subscription '%s': %s",
				offerID, subscriptionID, err.Error()),
		)
		return
	}

	state := subscriptionIntroductoryOfferModel{
		ID:             types.StringValue(offer.ID),
		SubscriptionID: types.StringValue(subscriptionID),
	}
	refreshIntroductoryOffer(&state, offer)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// resolveIntroductoryOfferComputed fills in the computed attributes a plan left
// unknown, from Apple's response, and nothing else: every other attribute is
// configuration the result has to repeat exactly. A value Apple did not report
// is null, never unknown -- an unknown left in state fails the apply outright.
func resolveIntroductoryOfferComputed(model *subscriptionIntroductoryOfferModel, offer *models.SubscriptionIntroductoryOffer) {
	if model.StartDate.IsUnknown() {
		model.StartDate = stringOrNull(offer.Attributes.StartDate)
	}
	if model.TargetSubscriptionPlanType.IsUnknown() {
		model.TargetSubscriptionPlanType = types.StringNull()
		if planType := offer.Attributes.TargetSubscriptionPlanType; planType != nil {
			model.TargetSubscriptionPlanType = types.StringValue(string(*planType))
		}
	}
}

// refreshIntroductoryOffer copies Apple's reported offer into a model, for a
// read or an import.
func refreshIntroductoryOffer(model *subscriptionIntroductoryOfferModel, offer *models.SubscriptionIntroductoryOffer) {
	entry := introductoryOfferEntryFromAPI(*offer)

	model.TerritoryID = entry.TerritoryID
	model.OfferMode = entry.OfferMode
	model.Duration = entry.Duration
	model.NumberOfPeriods = entry.NumberOfPeriods
	model.PricePointID = entry.PricePointID
	model.StartDate = entry.StartDate
	model.EndDate = entry.EndDate
	model.TargetSubscriptionPlanType = entry.TargetSubscriptionPlanType
}

// addIntroductoryOfferWriteError maps Apple's refusal of an offer write onto a
// diagnostic that names the likely cause.
func addIntroductoryOfferWriteError(diags *diag.Diagnostics, subscriptionID string, err error, summary string) {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
		diags.AddError(
			"Subscription or Price Point Not Found",
			fmt.Sprintf("%s: %s\n\n"+
				"A price point ID is only valid for the subscription it was read from — it encodes the "+
				"subscription, the territory and the tier together. Read it from the "+
				"apple_subscription_price_points data source scoped to subscription '%s'.",
				summary, errMsg, subscriptionID),
		)
	case strings.Contains(errMsg, "processing the pricing information"):
		diags.AddError(
			"Error Writing Subscription Introductory Offer",
			fmt.Sprintf("%s: %s\n\n"+
				"Apple answers with this message when the subscription has no availability record, which "+
				"it requires before any pricing. Add an apple_subscription_availability for subscription "+
				"'%s' covering the offer's territory, and make this resource depend on it.",
				summary, errMsg, subscriptionID),
		)
	default:
		diags.AddError(
			"Error Writing Subscription Introductory Offer",
			fmt.Sprintf("%s: %s\n\n"+
				"A territory holds one introductory offer at a time, so an offer whose dates overlap "+
				"another in the same territory is refused. The subscription also has to be available in "+
				"the territory, and Apple validates the duration and number of periods against the offer "+
				"mode and the subscription's period.", summary, errMsg),
		)
	}
}
