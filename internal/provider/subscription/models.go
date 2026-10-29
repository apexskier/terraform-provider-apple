// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package subscription contains the auto-renewable subscription resources and
// data sources for the Apple Terraform provider: subscription groups and their
// localizations, subscriptions, their localizations, and their prices.
//
// These are App Store Connect resources rather than Developer Portal ones, and
// they differ from the rest of the provider in two ways that shape every type
// here. They hang off an app record, which Apple's API cannot create -- "Don't
// use this API to create new apps; instead, create new apps on the App Store
// Connect website" -- so the app is always looked up, never managed. And they
// nest: a group holds subscriptions, a subscription holds localizations and
// prices, and Apple publishes no top-level collection for any of them, so the
// parent has to be known before a child can be read.
package subscription

import (
	"regexp"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// subscriptionGroupModel maps the subscription group schema for both the
// resource and the data source.
type subscriptionGroupModel struct {
	ID            types.String `tfsdk:"id"`
	AppID         types.String `tfsdk:"app_id"`
	ReferenceName types.String `tfsdk:"reference_name"`
}

// subscriptionGroupLocalizationModel maps the subscription group localization
// schema.
type subscriptionGroupLocalizationModel struct {
	ID            types.String `tfsdk:"id"`
	GroupID       types.String `tfsdk:"group_id"`
	Locale        types.String `tfsdk:"locale"`
	Name          types.String `tfsdk:"name"`
	CustomAppName types.String `tfsdk:"custom_app_name"`
	State         types.String `tfsdk:"state"`
}

// subscriptionModel maps the subscription schema for both the resource and the
// data source.
type subscriptionModel struct {
	ID                 types.String `tfsdk:"id"`
	GroupID            types.String `tfsdk:"group_id"`
	Name               types.String `tfsdk:"name"`
	ProductID          types.String `tfsdk:"product_id"`
	SubscriptionPeriod types.String `tfsdk:"subscription_period"`
	FamilySharable     types.Bool   `tfsdk:"family_sharable"`
	GroupLevel         types.Int64  `tfsdk:"group_level"`
	ReviewNote         types.String `tfsdk:"review_note"`
	State              types.String `tfsdk:"state"`
}

// subscriptionLocalizationModel maps the subscription localization schema.
type subscriptionLocalizationModel struct {
	ID             types.String `tfsdk:"id"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
	Locale         types.String `tfsdk:"locale"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	State          types.String `tfsdk:"state"`
}

// subscriptionPriceModel maps the subscription price schema.
type subscriptionPriceModel struct {
	ID                   types.String `tfsdk:"id"`
	SubscriptionID       types.String `tfsdk:"subscription_id"`
	PricePointID         types.String `tfsdk:"price_point_id"`
	TerritoryID          types.String `tfsdk:"territory_id"`
	StartDate            types.String `tfsdk:"start_date"`
	PreserveCurrentPrice types.Bool   `tfsdk:"preserve_current_price"`
	PlanType             types.String `tfsdk:"plan_type"`
	Preserved            types.Bool   `tfsdk:"preserved"`
}

// subscriptionPriceScheduleModel maps the bulk price schedule: every manual
// price of one subscription, written in a single request.
type subscriptionPriceScheduleModel struct {
	ID             types.String                     `tfsdk:"id"`
	SubscriptionID types.String                     `tfsdk:"subscription_id"`
	Prices         []subscriptionSchedulePriceModel `tfsdk:"prices"`
}

// subscriptionSchedulePriceModel is one price inside that schedule.
//
// It carries no id and no "preserved": both are computed, and a computed
// attribute inside a set makes the element itself unknown at plan time, which
// leaves Terraform unable to match a planned element against the one in state.
// The per-territory apple_subscription_price resource reports them instead.
type subscriptionSchedulePriceModel struct {
	PricePointID         types.String `tfsdk:"price_point_id"`
	TerritoryID          types.String `tfsdk:"territory_id"`
	StartDate            types.String `tfsdk:"start_date"`
	PreserveCurrentPrice types.Bool   `tfsdk:"preserve_current_price"`
	PlanType             types.String `tfsdk:"plan_type"`
}

// subscriptionPricePointModel maps one entry of Apple's price catalogue.
type subscriptionPricePointModel struct {
	ID            types.String `tfsdk:"id"`
	CustomerPrice types.String `tfsdk:"customer_price"`
	Proceeds      types.String `tfsdk:"proceeds"`
	ProceedsYear2 types.String `tfsdk:"proceeds_year_2"`
	TerritoryID   types.String `tfsdk:"territory_id"`
}

// subscriptionAvailabilityModel maps the subscription availability schema.
type subscriptionAvailabilityModel struct {
	ID                        types.String   `tfsdk:"id"`
	SubscriptionID            types.String   `tfsdk:"subscription_id"`
	AvailableInNewTerritories types.Bool     `tfsdk:"available_in_new_territories"`
	AvailableTerritories      []types.String `tfsdk:"available_territories"`
}

// subscriptionIntroductoryOfferModel maps the per-territory introductory offer
// schema.
type subscriptionIntroductoryOfferModel struct {
	ID                         types.String `tfsdk:"id"`
	SubscriptionID             types.String `tfsdk:"subscription_id"`
	TerritoryID                types.String `tfsdk:"territory_id"`
	OfferMode                  types.String `tfsdk:"offer_mode"`
	Duration                   types.String `tfsdk:"duration"`
	NumberOfPeriods            types.Int64  `tfsdk:"number_of_periods"`
	PricePointID               types.String `tfsdk:"price_point_id"`
	StartDate                  types.String `tfsdk:"start_date"`
	EndDate                    types.String `tfsdk:"end_date"`
	TargetSubscriptionPlanType types.String `tfsdk:"target_subscription_plan_type"`
}

// subscriptionIntroductoryOfferScheduleModel maps the bulk schedule: every
// introductory offer of one subscription.
type subscriptionIntroductoryOfferScheduleModel struct {
	ID             types.String                                 `tfsdk:"id"`
	SubscriptionID types.String                                 `tfsdk:"subscription_id"`
	Offers         []subscriptionIntroductoryOfferScheduleEntry `tfsdk:"offers"`
}

// subscriptionIntroductoryOfferScheduleEntry is one offer inside that schedule.
//
// It carries no id, for the reason subscriptionSchedulePriceModel carries none:
// a computed attribute inside a set makes the element unknown at plan time.
type subscriptionIntroductoryOfferScheduleEntry struct {
	TerritoryID                types.String `tfsdk:"territory_id"`
	OfferMode                  types.String `tfsdk:"offer_mode"`
	Duration                   types.String `tfsdk:"duration"`
	NumberOfPeriods            types.Int64  `tfsdk:"number_of_periods"`
	PricePointID               types.String `tfsdk:"price_point_id"`
	StartDate                  types.String `tfsdk:"start_date"`
	EndDate                    types.String `tfsdk:"end_date"`
	TargetSubscriptionPlanType types.String `tfsdk:"target_subscription_plan_type"`
}

// subscriptionGroupsDataSourceModel maps the listing data source for groups.
type subscriptionGroupsDataSourceModel struct {
	// Scope
	AppID types.String `tfsdk:"app_id"`

	// Filter configuration
	ReferenceNamePattern types.String `tfsdk:"reference_name_pattern"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	SubscriptionGroups []subscriptionGroupModel `tfsdk:"subscription_groups"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// subscriptionsDataSourceModel maps the listing data source for subscriptions.
type subscriptionsDataSourceModel struct {
	// Scope
	GroupID types.String `tfsdk:"group_id"`

	// Filter configuration
	NamePattern      types.String `tfsdk:"name_pattern"`
	ProductIDPattern types.String `tfsdk:"product_id_pattern"`
	State            types.String `tfsdk:"state"`
	Period           types.String `tfsdk:"subscription_period"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	Subscriptions []subscriptionModel `tfsdk:"subscriptions"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// subscriptionPricePointsDataSourceModel maps the price point catalogue lookup.
type subscriptionPricePointsDataSourceModel struct {
	// Scope
	SubscriptionID types.String `tfsdk:"subscription_id"`

	// Filter configuration. Territories are passed to Apple rather than
	// applied in memory: the unfiltered catalogue covers every territory the
	// App Store sells in.
	Territories   []types.String `tfsdk:"territories"`
	CustomerPrice types.String   `tfsdk:"customer_price"`

	// Result control
	Limit types.Int64 `tfsdk:"limit"`

	// Output
	PricePoints []subscriptionPricePointModel `tfsdk:"price_points"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// subscriptionPricePointEqualizationsDataSourceModel maps the equalization
// lookup: one base price point in, the equivalent point per territory out.
type subscriptionPricePointEqualizationsDataSourceModel struct {
	// Scope
	PricePointID types.String `tfsdk:"price_point_id"`

	// Filter configuration. Applied by Apple, like the catalogue read -- but
	// unlike it, omitting this is the ordinary case: the endpoint returns one
	// record per territory rather than the whole catalogue.
	Territories []types.String `tfsdk:"territories"`

	// Output. price_point_ids is the same deliberate deviation
	// apple_territories.ids is, and exists for the same reason: the nested
	// list is unusable in a for_each over territories without a flatten and a
	// one-element index expression every caller would have to repeat.
	PricePoints   []subscriptionPricePointModel `tfsdk:"price_points"`
	PricePointIDs map[string]types.String       `tfsdk:"price_point_ids"`

	// Computed metadata
	TotalCount types.Int64 `tfsdk:"total_count"`
}

// Subscription validators.
var (
	// ProductIDValidator validates a subscription product identifier.
	//
	// Apple documents no format beyond uniqueness across the account and a
	// 100-character ceiling, and the portal accepts alphanumerics with dots,
	// underscores and hyphens. This is deliberately permissive: rejecting an
	// identifier Apple would have accepted is worse than passing it through
	// and surfacing Apple's own error.
	ProductIDValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`),
		"Product ID must start with a letter or digit and contain only letters, digits, dots, underscores and hyphens",
	)

	// SubscriptionPeriodValidator validates the subscription duration.
	SubscriptionPeriodValidator = stringvalidator.OneOf(models.ValidSubscriptionPeriods...)

	// PlanTypeValidator validates the price plan type.
	PlanTypeValidator = stringvalidator.OneOf(models.ValidSubscriptionPlanTypes...)

	// OfferModeValidator validates an introductory offer mode.
	OfferModeValidator = stringvalidator.OneOf(models.ValidSubscriptionOfferModes...)

	// OfferDurationValidator validates an introductory offer duration.
	OfferDurationValidator = stringvalidator.OneOf(models.ValidSubscriptionOfferDurations...)

	// LocaleValidator validates an App Store locale code.
	//
	// Apple's locales are BCP 47-ish but not uniformly so: "en-US", "es-MX",
	// "ar-SA", but also bare "ar" and "no". Both shapes are accepted.
	LocaleValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`),
		"Locale must be an App Store locale code such as en-US, es-MX, ar-SA or ar",
	)

	// StartDateValidator validates the plain date Apple expects.
	//
	// subscriptionPrices takes a date, not a timestamp: "2026-01-01", never
	// "2026-01-01T00:00:00Z".
	StartDateValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`),
		"Start date must be a plain date in YYYY-MM-DD form, not a timestamp",
	)

	// EndDateValidator is StartDateValidator for an end date, so the error
	// names the attribute the user set.
	EndDateValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`),
		"End date must be a plain date in YYYY-MM-DD form, not a timestamp",
	)

	// TerritoryValidator validates a territory code.
	//
	// Apple identifies territories by three-letter uppercase code -- USA, GBR,
	// EGY -- not the two-letter ISO 3166-1 alpha-2 codes.
	TerritoryValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[A-Z]{3}$`),
		"Territory must be a three-letter uppercase Apple territory code such as USA, GBR or EGY",
	)

	// PriceValidator validates a customer price filter.
	//
	// Apple reports prices as decimal strings without a currency symbol, since
	// the currency is a property of the territory: "9.99", "1200", "0.99".
	PriceValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^\d+(\.\d+)?$`),
		"Customer price must be a plain decimal number without a currency symbol, such as 9.99",
	)

	// GroupSortByValidator validates sort field options for groups.
	GroupSortByValidator = stringvalidator.OneOf("reference_name", "id")

	// SubscriptionSortByValidator validates sort field options for subscriptions.
	SubscriptionSortByValidator = stringvalidator.OneOf("name", "product_id", "state", "group_level")

	// SortOrderValidator validates sort order options.
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")

	// StateValidator validates a subscription state filter.
	StateValidator = stringvalidator.OneOf(
		string(models.SubscriptionStateMissingMetadata),
		string(models.SubscriptionStateReadyToSubmit),
		string(models.SubscriptionStateWaitingForReview),
		string(models.SubscriptionStateInReview),
		string(models.SubscriptionStateDeveloperActionNeeded),
		string(models.SubscriptionStatePendingBinaryApproval),
		string(models.SubscriptionStateApproved),
		string(models.SubscriptionStateDeveloperRemovedFromSale),
		string(models.SubscriptionStateRemovedFromSale),
		string(models.SubscriptionStateRejected),
	)
)

// GetProductIDValidator returns validators for subscription product IDs.
func GetProductIDValidator() []validator.String {
	return []validator.String{
		ProductIDValidator,
		stringvalidator.LengthBetween(1, 100),
	}
}

// GetNameValidator returns validators for subscription and group names.
//
// Apple caps the customer-facing name at 30 characters, which is shorter than
// most Apple name fields and is enforced at creation rather than at review.
//
// The limit is counted in characters, so this uses UTF8LengthBetween rather
// than LengthBetween: the latter counts bytes, which rejects a perfectly legal
// Arabic or Japanese name at roughly half the length Apple actually allows.
func GetNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 30),
	}
}

// GetReferenceNameValidator returns validators for group reference names.
//
// Counted in characters, not bytes -- see GetNameValidator.
func GetReferenceNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 64),
	}
}

// GetDescriptionValidator returns validators for localization descriptions.
//
// Counted in characters, not bytes -- see GetNameValidator.
func GetDescriptionValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 45),
	}
}

// GetCustomAppNameValidator returns validators for a group localization's
// custom app name.
//
// Counted in characters, not bytes -- see GetNameValidator.
func GetCustomAppNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 30),
	}
}

// GetPatternValidator returns validators for regex pattern fields.
func GetPatternValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 255),
	}
}

// stringOrNil converts an optional Terraform string to the pointer the API
// models use, so an unset attribute is omitted from the request rather than
// sent as an empty string.
func stringOrNil(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// boolOrNil converts an optional Terraform bool to a pointer.
func boolOrNil(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// intOrNil converts an optional Terraform int64 to an *int.
func intOrNil(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := int(v.ValueInt64())
	return &i
}

// stringOrNull converts an optional API string pointer back to Terraform,
// mapping a missing value to null rather than to "".
func stringOrNull(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

// territoryID reads the territory out of a relationship, returning null when
// Apple did not populate the linkage.
func territoryID(rel *models.ResourceIdentifier) types.String {
	if rel == nil || rel.Data.ID == "" {
		return types.StringNull()
	}
	return types.StringValue(rel.Data.ID)
}
