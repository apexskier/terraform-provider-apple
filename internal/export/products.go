// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"fmt"
	"sort"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// exportProducts writes an app's subscriptions and one-time purchases.
//
// Every product carries a product_id that Apple never releases, even after the
// product is deleted, and product_id forces replacement. That is the reason
// to export rather than hand-write these: a mistyped identifier plans a
// destroy that burns the real one for good.
func (e *exporter) exportProducts(f *file, app models.App) error {
	bundle := app.Attributes.BundleID
	f.comment(fmt.Sprintf("Subscriptions and in-app purchases of %s (app ID %s).\nproduct_id forces replacement, and Apple never releases a product identifier: do not edit one.", app.Attributes.Name, app.ID))

	groups, err := e.src.GetSubscriptionGroups(app.ID)
	if err != nil {
		if err := e.failed(bundle+"'s subscription groups", err); err != nil {
			return err
		}
	}
	for _, group := range groups {
		if err := e.exportSubscriptionGroup(f, app, group); err != nil {
			return err
		}
	}

	purchases, err := e.src.GetInAppPurchases(app.ID)
	if err != nil {
		if err := e.failed(bundle+"'s in-app purchases", err); err != nil {
			return err
		}
	}
	for _, purchase := range purchases {
		if err := e.exportInAppPurchase(f, app, purchase); err != nil {
			return err
		}
	}

	if len(groups) > 0 || len(purchases) > 0 {
		e.note(Skipped, "", "%s: App Review screenshots of subscriptions and in-app purchases were left out. Apple keeps the bytes, not the file, so they need the image on disk; add an apple_subscription_review_screenshot or apple_in_app_purchase_review_screenshot pointing at it.", bundle)
	}
	return nil
}

func (e *exporter) exportSubscriptionGroup(f *file, app models.App, group models.SubscriptionGroup) error {
	name := group.Attributes.ReferenceName
	b, addr := e.resource(f, "apple_subscription_group", group.ID, app.ID+"/"+group.ID, name)
	b.str("app_id", app.ID)
	b.str("reference_name", name)

	localizations, err := e.src.GetSubscriptionGroupLocalizations(group.ID)
	if err != nil {
		if err := e.failed(fmt.Sprintf("the localizations of subscription group %q", name), err); err != nil {
			return err
		}
	}
	for _, localization := range localizations {
		locale := localization.Attributes.Locale
		l, _ := e.resource(f, "apple_subscription_group_localization", localization.ID,
			group.ID+"/"+localization.ID, name, locale)
		l.set("group_id", ref(addr.traversal("id")))
		l.str("locale", locale)
		l.str("name", localization.Attributes.Name)
		l.optStr("custom_app_name", localization.Attributes.CustomAppName)
	}

	subscriptions, err := e.src.GetSubscriptions(group.ID)
	if err != nil {
		return e.failed(fmt.Sprintf("the subscriptions in group %q", name), err)
	}
	for _, subscription := range subscriptions {
		if err := e.exportSubscription(f, addr, subscription); err != nil {
			return err
		}
	}
	return nil
}

func (e *exporter) exportSubscription(f *file, group address, subscription models.Subscription) error {
	productID := subscription.Attributes.ProductID
	attributes := subscription.Attributes

	b, addr := e.resource(f, "apple_subscription", subscription.ID, subscription.ID, productID)
	b.set("group_id", ref(group.traversal("id")))
	b.str("product_id", productID)
	b.str("name", attributes.Name)
	if attributes.SubscriptionPeriod != nil {
		b.str("subscription_period", string(*attributes.SubscriptionPeriod))
	}
	b.optBool("family_sharable", attributes.FamilySharable)
	if attributes.GroupLevel != nil {
		level := int64(*attributes.GroupLevel)
		b.optInt("group_level", &level)
	}
	b.optStr("review_note", attributes.ReviewNote)

	localizations, err := e.src.GetSubscriptionLocalizations(subscription.ID)
	if err != nil {
		if err := e.failed("the localizations of "+productID, err); err != nil {
			return err
		}
	}
	for _, localization := range localizations {
		locale := localization.Attributes.Locale
		l, _ := e.resource(f, "apple_subscription_localization", localization.ID,
			subscription.ID+"/"+localization.ID, productID, locale)
		l.set("subscription_id", ref(addr.traversal("id")))
		l.str("locale", locale)
		l.str("name", localization.Attributes.Name)
		l.optStr("description", localization.Attributes.Description)
	}

	// Availability comes before prices: Apple refuses a price on a
	// subscription that has none, so the price schedule depends on it.
	availability, err := e.exportSubscriptionAvailability(f, subscription)
	if err != nil {
		return err
	}

	priceSchedule, err := e.exportSubscriptionPriceSchedule(f, subscription, addr, availability)
	if err != nil {
		return err
	}

	// Offers come after both: an offer is refused in a territory the
	// subscription is not available in, and a paid offer is a price.
	return e.exportSubscriptionIntroductoryOffers(f, subscription, addr, availability, priceSchedule)
}

// exportSubscriptionPriceSchedule writes the subscription's prices, returning
// the schedule's address, or nil when the subscription has none.
func (e *exporter) exportSubscriptionPriceSchedule(f *file, subscription models.Subscription, addr address, availability *address) (*address, error) {
	productID := subscription.Attributes.ProductID

	prices, err := e.src.GetSubscriptionPrices(subscription.ID)
	if err != nil {
		return nil, e.failed("the prices of "+productID, err)
	}
	if len(prices) == 0 {
		return nil, nil
	}

	// One price schedule, never an apple_subscription_price per price: the
	// two write the same set and would take each other's prices off.
	byPoint := map[string]models.SubscriptionPrice{}
	for _, price := range prices {
		if price.Relationships == nil || price.Relationships.SubscriptionPricePoint == nil {
			continue
		}
		if point := price.Relationships.SubscriptionPricePoint.Data.ID; point != "" {
			byPoint[point] = price
		}
	}
	objects := make([]map[string]cty.Value, 0, len(byPoint))
	for _, point := range sortedKeys(byPoint) {
		price := byPoint[point]
		object := map[string]cty.Value{
			"price_point_id": cty.StringVal(point),
			"start_date":     optString(price.Attributes.StartDate),
			"territory_id":   cty.NullVal(cty.String),
			"plan_type":      cty.NullVal(cty.String),
		}
		if price.Relationships.Territory != nil && price.Relationships.Territory.Data.ID != "" {
			object["territory_id"] = cty.StringVal(price.Relationships.Territory.Data.ID)
		}
		if price.Attributes.PlanType != nil {
			object["plan_type"] = cty.StringVal(string(*price.Attributes.PlanType))
		}
		objects = append(objects, object)
	}

	s, saddr := e.resource(f, "apple_subscription_price_schedule", subscription.ID, subscription.ID, productID)
	s.set("subscription_id", ref(addr.traversal("id")))
	s.objects("prices", objects)
	if availability != nil {
		s.body.SetAttributeRaw("depends_on", tupleOfTraversals(availability.traversal()))
	}
	e.note(Attention, saddr.String(),
		"Every price Apple holds for %s is listed, past ones included. Apple deletes only future price changes, so to change a price add a new one with a later start_date rather than editing or removing a live one.", productID)

	return &saddr, nil
}

// exportSubscriptionIntroductoryOffers mirrors the introductory offer
// schedule's ImportState: every offer Apple holds, with Apple's own values, and
// a free trial's price point left out because the import records it null.
//
// One schedule, never an apple_subscription_introductory_offer per offer: the
// schedule removes any offer it does not list, so the two would fight.
func (e *exporter) exportSubscriptionIntroductoryOffers(f *file, subscription models.Subscription, subscriptionAddr address, prerequisites ...*address) error {
	productID := subscription.Attributes.ProductID

	offers, err := e.src.GetSubscriptionIntroductoryOffers(subscription.ID, nil)
	if err != nil {
		return e.failed("the introductory offers of "+productID, err)
	}
	if len(offers) == 0 {
		return nil
	}

	objects := make([]map[string]cty.Value, 0, len(offers))
	for _, offer := range offers {
		attributes := offer.Attributes
		object := map[string]cty.Value{
			"territory_id":                  cty.NullVal(cty.String),
			"offer_mode":                    cty.NullVal(cty.String),
			"duration":                      cty.NullVal(cty.String),
			"number_of_periods":             cty.NullVal(cty.Number),
			"price_point_id":                cty.NullVal(cty.String),
			"start_date":                    optString(attributes.StartDate),
			"end_date":                      optString(attributes.EndDate),
			"target_subscription_plan_type": cty.NullVal(cty.String),
		}
		if offer.Relationships != nil {
			if t := offer.Relationships.Territory; t != nil && t.Data.ID != "" {
				object["territory_id"] = cty.StringVal(t.Data.ID)
			}
			if p := offer.Relationships.SubscriptionPricePoint; p != nil && p.Data.ID != "" {
				object["price_point_id"] = cty.StringVal(p.Data.ID)
			}
		}
		if attributes.OfferMode != nil {
			object["offer_mode"] = cty.StringVal(string(*attributes.OfferMode))
		}
		if attributes.Duration != nil {
			object["duration"] = cty.StringVal(string(*attributes.Duration))
		}
		if attributes.NumberOfPeriods != nil {
			object["number_of_periods"] = cty.NumberIntVal(int64(*attributes.NumberOfPeriods))
		}
		if attributes.TargetSubscriptionPlanType != nil {
			object["target_subscription_plan_type"] = cty.StringVal(string(*attributes.TargetSubscriptionPlanType))
		}
		objects = append(objects, object)
	}
	// Apple's order carries no meaning for a set; sorting keeps the output
	// stable from one export to the next.
	sortKey := func(object map[string]cty.Value) string {
		key := ""
		for _, name := range []string{"territory_id", "start_date"} {
			if v := object[name]; !v.IsNull() {
				key += v.AsString()
			}
			key += "\x00"
		}
		return key
	}
	sort.SliceStable(objects, func(i, j int) bool {
		return sortKey(objects[i]) < sortKey(objects[j])
	})

	b, _ := e.resource(f, "apple_subscription_introductory_offer_schedule", subscription.ID, subscription.ID, productID)
	b.set("subscription_id", ref(subscriptionAddr.traversal("id")))
	b.objects("offers", objects)

	var dependencies []hcl.Traversal
	for _, prerequisite := range prerequisites {
		if prerequisite != nil {
			dependencies = append(dependencies, prerequisite.traversal())
		}
	}
	if len(dependencies) > 0 {
		b.body.SetAttributeRaw("depends_on", tupleOfTraversals(dependencies...))
	}
	return nil
}

func (e *exporter) exportSubscriptionAvailability(f *file, subscription models.Subscription) (*address, error) {
	productID := subscription.Attributes.ProductID

	availability, err := e.src.GetSubscriptionAvailability(subscription.ID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, e.failed("the availability of "+productID, err)
	}
	territories, err := e.src.GetSubscriptionAvailableTerritories(availability.ID)
	if err != nil {
		return nil, e.failed("the territories of "+productID, err)
	}

	b, addr := e.resource(f, "apple_subscription_availability", availability.ID, subscription.ID, productID)
	b.set("subscription_id", e.idOf("apple_subscription", subscription.ID))
	b.set("available_in_new_territories", literal(cty.BoolVal(boolOr(availability.Attributes.AvailableInNewTerritories, false))))
	b.strings("available_territories", territoryCodes(territories))
	return &addr, nil
}

func (e *exporter) exportInAppPurchase(f *file, app models.App, purchase models.InAppPurchase) error {
	productID := purchase.Attributes.ProductID
	attributes := purchase.Attributes
	if attributes.InAppPurchaseType == nil {
		e.note(Skipped, "", "In-app purchase %s reports no type, so it was left out.", productID)
		return nil
	}

	b, addr := e.resource(f, "apple_in_app_purchase", purchase.ID, app.ID+"/"+purchase.ID, productID)
	b.str("app_id", app.ID)
	b.str("product_id", productID)
	b.str("name", attributes.Name)
	b.str("in_app_purchase_type", string(*attributes.InAppPurchaseType))
	b.optBool("family_sharable", attributes.FamilySharable)
	b.optStr("review_note", attributes.ReviewNote)

	// Localizations hang off a version of the purchase. The newest is the one
	// App Store Connect shows and the one the next edit lands on.
	versions, err := e.src.GetInAppPurchaseVersions(purchase.ID)
	if err != nil && !isNotFound(err) {
		if err := e.failed("the versions of "+productID, err); err != nil {
			return err
		}
	}
	if latest := latestPurchaseVersion(versions); latest != nil {
		localizations, err := e.src.GetInAppPurchaseLocalizations(latest.ID)
		if err != nil {
			if err := e.failed("the localizations of "+productID, err); err != nil {
				return err
			}
		}
		for _, localization := range localizations {
			locale := localization.Attributes.Locale
			l, _ := e.resource(f, "apple_in_app_purchase_localization", localization.ID,
				purchase.ID+"/"+localization.ID, productID, locale)
			l.set("in_app_purchase_id", ref(addr.traversal("id")))
			l.str("locale", locale)
			l.str("name", localization.Attributes.Name)
			l.optStr("description", localization.Attributes.Description)
		}
	}

	schedule, prices, err := e.src.GetInAppPurchasePrices(purchase.ID)
	switch {
	case err != nil && isNotFound(err):
	case err != nil:
		if err := e.failed("the price schedule of "+productID, err); err != nil {
			return err
		}
	default:
		e.exportPurchasePrices(f, addr, productID, purchase.ID, schedule, prices)
	}

	availability, err := e.src.GetInAppPurchaseAvailability(purchase.ID)
	switch {
	case err != nil && isNotFound(err):
	case err != nil:
		return e.failed("the availability of "+productID, err)
	default:
		territories, err := e.src.GetInAppPurchaseAvailableTerritories(availability.ID)
		if err != nil {
			return e.failed("the territories of "+productID, err)
		}
		a, _ := e.resource(f, "apple_in_app_purchase_availability", availability.ID, purchase.ID, productID)
		a.set("in_app_purchase_id", ref(addr.traversal("id")))
		a.set("available_in_new_territories", literal(cty.BoolVal(boolOr(availability.Attributes.AvailableInNewTerritories, false))))
		a.strings("available_territories", territoryCodes(territories))
	}

	return nil
}

func (e *exporter) exportPurchasePrices(f *file, purchase address, productID, purchaseID string,
	schedule *models.InAppPurchasePriceSchedule, prices []models.InAppPurchasePrice) {
	base := ""
	if schedule.Relationships != nil {
		base = relationshipID(schedule.Relationships.BaseTerritory)
	}
	if base == "" {
		e.note(Skipped, "", "The price schedule of %s reports no base territory, which the import requires, so it was left out.", productID)
		return
	}

	byPoint := map[string]models.InAppPurchasePrice{}
	for _, price := range prices {
		if price.Relationships == nil {
			continue
		}
		if point := relationshipID(price.Relationships.InAppPurchasePricePoint); point != "" {
			byPoint[point] = price
		}
	}
	objects := make([]map[string]cty.Value, 0, len(byPoint))
	for _, point := range sortedKeys(byPoint) {
		price := byPoint[point]
		objects = append(objects, map[string]cty.Value{
			"price_point_id": cty.StringVal(point),
			"start_date":     optString(price.Attributes.StartDate),
			"end_date":       optString(price.Attributes.EndDate),
		})
	}

	b, _ := e.resource(f, "apple_in_app_purchase_price_schedule", schedule.ID, purchaseID, productID)
	b.set("in_app_purchase_id", ref(purchase.traversal("id")))
	b.str("base_territory", base)
	b.objects("prices", objects)
}

func latestPurchaseVersion(versions []models.InAppPurchaseVersion) *models.InAppPurchaseVersion {
	var latest *models.InAppPurchaseVersion
	for i := range versions {
		version := &versions[i]
		if latest == nil || versionNumber(version) > versionNumber(latest) {
			latest = version
		}
	}
	return latest
}

func versionNumber(version *models.InAppPurchaseVersion) int {
	if version.Attributes.Version == nil {
		return -1
	}
	return *version.Attributes.Version
}

func territoryCodes(territories []models.Territory) []string {
	codes := make([]string, 0, len(territories))
	for _, territory := range territories {
		codes = append(codes, territory.ID)
	}
	sort.Strings(codes)
	return codes
}

func boolOr(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}
