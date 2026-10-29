// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Subscriptions hang off an App Store Connect app record, and Apple's API
// cannot create one: "Don't use this API to create new apps; instead, create
// new apps on the App Store Connect website." These tests therefore need an app
// that already exists, named by APPLE_TEST_APP_ID, and skip without it.
const testAccAppIDEnvVar = "APPLE_TEST_APP_ID"

// testAccPreCheckSubscription skips unless credentials and a test app are both
// available.
func testAccPreCheckSubscription(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	if os.Getenv(testAccAppIDEnvVar) == "" {
		t.Skipf("skipping subscription acceptance test: %s must name an existing App Store Connect app. "+
			"Apple's API cannot create an app record, so one has to be made in App Store Connect by hand.",
			testAccAppIDEnvVar)
	}
}

func testAccAppID() string {
	return os.Getenv(testAccAppIDEnvVar)
}

// testAccSubscriptionProductID returns a product identifier unique to this run.
//
// Identifiers elsewhere in this suite are fixed, matching the existing tests.
// Subscriptions are the exception on purpose: Apple never releases a product
// identifier, not even one belonging to a subscription that was deleted before
// it was ever approved. A fixed identifier would therefore pass exactly once
// per account and fail on every run afterwards, the way the device tests do.
// Randomising costs nothing -- the subscription itself is deletable -- and
// keeps the suite repeatable.
func testAccSubscriptionProductID() string {
	return fmt.Sprintf("com.test.terraform.sub%s", strings.ToLower(acctest.RandString(8)))
}

// testAccProductName returns a display name unique to this run.
//
// Apple enforces name uniqueness across the live products of an app -- "This
// name is already being used by another in-app purchase associated with this
// app", where "in-app purchase" covers subscriptions too. A fixed name
// therefore collides with whatever an earlier failed test left behind, and
// with the test that ran a second before if Apple has not finished deleting
// its subscription yet. That is a weaker constraint than the one on product
// identifiers, which Apple never releases at all, but it needs the same fix.
//
// Names are capped at 30 characters, so base must leave room for " " plus six.
func testAccProductName(base string) string {
	return fmt.Sprintf("%s %s", base, acctest.RandString(6))
}

func TestAccSubscriptionGroupResource_basic(t *testing.T) {
	referenceName := "Terraform Test " + acctest.RandString(6)
	renamed := "Terraform Renamed " + acctest.RandString(6)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSubscriptionGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionGroupConfig(referenceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionGroupExists("apple_subscription_group.test"),
					resource.TestCheckResourceAttr("apple_subscription_group.test", "reference_name", referenceName),
					resource.TestCheckResourceAttr("apple_subscription_group.test", "app_id", testAccAppID()),
					resource.TestCheckResourceAttrSet("apple_subscription_group.test", "id"),
				),
			},
			// The reference name is the only attribute Apple's update request
			// accepts, so this is the whole of the in-place update path.
			{
				Config: testAccSubscriptionGroupConfig(renamed),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionGroupExists("apple_subscription_group.test"),
					resource.TestCheckResourceAttr("apple_subscription_group.test", "reference_name", renamed),
				),
			},
			// Only the composite form yields complete state: Apple never
			// reports which app a group belongs to.
			{
				ResourceName:      "apple_subscription_group.test",
				ImportState:       true,
				ImportStateIdFunc: testAccSubscriptionGroupImportID("apple_subscription_group.test"),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccSubscriptionGroupResource_importRejectsBareID(t *testing.T) {
	referenceName := "Terraform Import " + acctest.RandString(6)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSubscriptionGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionGroupConfig(referenceName),
				Check:  testAccCheckSubscriptionGroupExists("apple_subscription_group.test"),
			},
			{
				ResourceName:  "apple_subscription_group.test",
				ImportState:   true,
				ImportStateId: "21451234",
				ExpectError:   regexp.MustCompile("Invalid Import ID"),
			},
		},
	})
}

// TestAccSubscriptionGroupLocalizationResource_basic covers the customer-facing
// half of a group: the heading a customer reads above the list of plans, as
// opposed to the internal reference_name on the group itself.
func TestAccSubscriptionGroupLocalizationResource_basic(t *testing.T) {
	referenceName := "Terraform Loc " + acctest.RandString(6)
	name := testAccProductName("Premium")
	renamed := testAccProductName("Premium Plus")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionGroupLocalizationDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionGroupLocalizationConfig(referenceName, name, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionGroupLocalizationExists("apple_subscription_group_localization.test"),
					resource.TestCheckResourceAttr("apple_subscription_group_localization.test", "locale", "en-US"),
					resource.TestCheckResourceAttr("apple_subscription_group_localization.test", "name", name),
					resource.TestCheckNoResourceAttr("apple_subscription_group_localization.test", "custom_app_name"),
					resource.TestCheckResourceAttrSet("apple_subscription_group_localization.test", "state"),
				),
			},
			// Both name and custom_app_name update in place: they are the only
			// two members of Apple's update request.
			{
				Config: testAccSubscriptionGroupLocalizationConfig(referenceName, renamed, "Example App"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionGroupLocalizationExists("apple_subscription_group_localization.test"),
					resource.TestCheckResourceAttr("apple_subscription_group_localization.test", "name", renamed),
					resource.TestCheckResourceAttr("apple_subscription_group_localization.test", "custom_app_name", "Example App"),
				),
			},
			// A bare ID suffices here, unlike apple_subscription_group: Apple
			// does report which group a localization belongs to.
			{
				ResourceName:      "apple_subscription_group_localization.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "apple_subscription_group_localization.test",
				ImportState:       true,
				ImportStateIdFunc: testAccSubscriptionGroupLocalizationImportID("apple_subscription_group_localization.test"),
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccSubscriptionGroupLocalizationResource_requiresReplace covers the
// locale being immutable: it identifies the record, and Apple's update request
// has no locale member.
//
// The replacement locale is en-GB, which the app named by APPLE_TEST_APP_ID has
// to be localized into -- Apple rejects a localization for a locale the app
// does not support. Change it if that app carries a different second locale.
func TestAccSubscriptionGroupLocalizationResource_requiresReplace(t *testing.T) {
	referenceName := "Terraform LocRep " + acctest.RandString(6)
	name := testAccProductName("Premium")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionGroupLocalizationDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionGroupLocalizationLocaleConfig(referenceName, name, "en-US"),
				Check:  testAccCheckSubscriptionGroupLocalizationExists("apple_subscription_group_localization.test"),
			},
			{
				Config: testAccSubscriptionGroupLocalizationLocaleConfig(referenceName, name, "en-GB"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionGroupLocalizationExists("apple_subscription_group_localization.test"),
					resource.TestCheckResourceAttr("apple_subscription_group_localization.test", "locale", "en-GB"),
				),
			},
		},
	})
}

func TestAccSubscriptionResource_basic(t *testing.T) {
	referenceName := "Terraform Sub " + acctest.RandString(6)
	productID := testAccSubscriptionProductID()
	name := testAccProductName("Pro Monthly")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionConfig(referenceName, productID, name, "ONE_MONTH"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttr("apple_subscription.test", "product_id", productID),
					resource.TestCheckResourceAttr("apple_subscription.test", "name", name),
					resource.TestCheckResourceAttr("apple_subscription.test", "subscription_period", "ONE_MONTH"),
					// A subscription with neither a localization nor a price is
					// incomplete, and Apple says so.
					resource.TestCheckResourceAttr("apple_subscription.test", "state", "MISSING_METADATA"),
					resource.TestCheckResourceAttrPair(
						"apple_subscription.test", "group_id",
						"apple_subscription_group.test", "id",
					),
				),
			},
			// Both the name and the period are updatable in place. The product
			// ID is not, and is covered separately.
			{
				Config: testAccSubscriptionConfig(referenceName, productID, "Pro Yearly", "ONE_YEAR"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttr("apple_subscription.test", "name", "Pro Yearly"),
					resource.TestCheckResourceAttr("apple_subscription.test", "subscription_period", "ONE_YEAR"),
				),
			},
			// A bare ID suffices: the provider reads with include=group.
			{
				ResourceName:      "apple_subscription.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccSubscriptionResource_completesMetadata covers the reason localizations
// and prices are separate resources: only once both exist does Apple move the
// subscription out of MISSING_METADATA.
func TestAccSubscriptionResource_completesMetadata(t *testing.T) {
	referenceName := "Terraform Full " + acctest.RandString(6)
	productID := testAccSubscriptionProductID()
	name := testAccProductName("Pro Monthly")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionCompleteConfig(referenceName, productID, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttr("apple_subscription_localization.test", "locale", "en-US"),
					resource.TestCheckResourceAttr("apple_subscription_localization.test", "name", name),
					resource.TestCheckResourceAttrSet("apple_subscription_localization.test", "state"),
					resource.TestCheckResourceAttrSet("apple_subscription_price.test", "id"),
					resource.TestCheckResourceAttrSet("apple_subscription_price.test", "price_point_id"),
					// The price point catalogue must have yielded something, or
					// the price could not have been created at all.
					resource.TestCheckResourceAttrSet(
						"data.apple_subscription_price_points.test", "price_points.0.customer_price"),
					resource.TestCheckResourceAttr("apple_subscription_availability.test", "available_territories.#", "1"),
					resource.TestCheckResourceAttr("apple_subscription_availability.test", "available_territories.0", "USA"),
					// Defaulted rather than configured, matching App Store
					// Connect's own default.
					resource.TestCheckResourceAttr("apple_subscription_availability.test", "available_in_new_territories", "true"),
				),
			},
			{
				ResourceName:      "apple_subscription_localization.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "apple_subscription_price.test",
				ImportState:       true,
				ImportStateIdFunc: testAccSubscriptionPriceImportID("apple_subscription_price.test"),
				ImportStateVerify: true,
				// preserve_current_price is an instruction rather than a
				// property: Apple reports only the outcome, through
				// "preserved", so import cannot recover it.
				ImportStateVerifyIgnore: []string{"preserve_current_price"},
			},
			// The availability imports by the subscription ID: a subscription
			// has exactly one, and Apple publishes no collection of them.
			{
				ResourceName:      "apple_subscription_availability.test",
				ImportState:       true,
				ImportStateIdFunc: testAccSubscriptionParentID("apple_subscription_availability.test"),
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccSubscriptionResource_requiresReplace covers the product ID being
// immutable. Note that this burns two identifiers rather than one.
func TestAccSubscriptionResource_requiresReplace(t *testing.T) {
	referenceName := "Terraform Replace " + acctest.RandString(6)
	before := testAccSubscriptionProductID()
	after := testAccSubscriptionProductID()
	name := testAccProductName("Pro Monthly")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionConfig(referenceName, before, name, "ONE_MONTH"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttr("apple_subscription.test", "product_id", before),
				),
			},
			{
				Config: testAccSubscriptionConfig(referenceName, after, name, "ONE_MONTH"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					resource.TestCheckResourceAttr("apple_subscription.test", "product_id", after),
				),
			},
		},
	})
}

// TestAccSubscriptionResource_validation exercises the schema validators. It
// creates nothing at Apple: every step fails at plan time.
func TestAccSubscriptionResource_validation(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError *regexp.Regexp
	}{
		{
			name: "invalid subscription period",
			config: `
resource "apple_subscription" "test" {
  group_id            = "12345"
  product_id          = "com.test.terraform.invalid"
  name                = "Invalid"
  subscription_period = "TWO_WEEKS"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		},
		{
			name: "name longer than Apple's 30 character limit",
			config: `
resource "apple_subscription" "test" {
  group_id   = "12345"
  product_id = "com.test.terraform.invalid"
  name       = "A name that is far too long to be accepted by Apple"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Length`),
		},
		{
			name: "product id with a space",
			config: `
resource "apple_subscription" "test" {
  group_id   = "12345"
  product_id = "com.test.terraform invalid"
  name       = "Invalid"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		},
		{
			name: "start date given as a timestamp rather than a date",
			config: `
resource "apple_subscription_price" "test" {
  subscription_id = "12345"
  price_point_id  = "abcdef"
  start_date      = "2027-01-01T00:00:00Z"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		},
		{
			name: "two-letter territory code instead of Apple's three-letter one",
			config: `
resource "apple_subscription_price" "test" {
  subscription_id = "12345"
  price_point_id  = "abcdef"
  territory_id    = "US"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		},
		{
			name: "malformed locale",
			config: `
resource "apple_subscription_localization" "test" {
  subscription_id = "12345"
  locale          = "English"
  name            = "Pro"
}`,
			expectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccProviderConfig + tt.config,
						PlanOnly:    true,
						ExpectError: tt.expectError,
					},
				},
			})
		})
	}
}

// --- configurations ---

func testAccSubscriptionGroupConfig(referenceName string) string {
	return testAccProviderConfig + fmt.Sprintf(`
resource "apple_subscription_group" "test" {
  app_id         = %[1]q
  reference_name = %[2]q
}
`, testAccAppID(), referenceName)
}

// testAccSubscriptionGroupLocalizationConfig builds a group and one
// localization. An empty customAppName omits the attribute entirely rather than
// setting it to "", which is a different thing to Apple.
func testAccSubscriptionGroupLocalizationConfig(referenceName, name, customAppName string) string {
	custom := ""
	if customAppName != "" {
		custom = fmt.Sprintf("\n  custom_app_name = %q\n", customAppName)
	}

	return testAccSubscriptionGroupConfig(referenceName) + fmt.Sprintf(`
resource "apple_subscription_group_localization" "test" {
  group_id = apple_subscription_group.test.id
  locale   = "en-US"
  name     = %[1]q
%[2]s}
`, name, custom)
}

func testAccSubscriptionGroupLocalizationLocaleConfig(referenceName, name, locale string) string {
	return testAccSubscriptionGroupConfig(referenceName) + fmt.Sprintf(`
resource "apple_subscription_group_localization" "test" {
  group_id = apple_subscription_group.test.id
  locale   = %[1]q
  name     = %[2]q
}
`, locale, name)
}

func testAccSubscriptionConfig(referenceName, productID, name, period string) string {
	return testAccSubscriptionGroupConfig(referenceName) + fmt.Sprintf(`
resource "apple_subscription" "test" {
  group_id            = apple_subscription_group.test.id
  product_id          = %[1]q
  name                = %[2]q
  subscription_period = %[3]q
}
`, productID, name, period)
}

// testAccSubscriptionCompleteConfig builds a subscription with everything Apple
// needs to consider it complete. The price point is looked up rather than
// hardcoded: a price point ID encodes the subscription it belongs to, so it
// cannot be known before the subscription exists.
func testAccSubscriptionCompleteConfig(referenceName, productID, name string) string {
	return testAccSubscriptionConfig(referenceName, productID, name, "ONE_MONTH") + fmt.Sprintf(`
resource "apple_subscription_localization" "test" {
  subscription_id = apple_subscription.test.id
  locale          = "en-US"
  name            = %[1]q
  description     = "Everything in Pro, billed monthly."
}

data "apple_subscription_price_points" "test" {
  subscription_id = apple_subscription.test.id
  territories     = ["USA"]
  customer_price  = "0.99"
  limit           = 1
}

resource "apple_subscription_availability" "test" {
  subscription_id       = apple_subscription.test.id
  available_territories = ["USA"]
}

resource "apple_subscription_price" "test" {
  subscription_id = apple_subscription.test.id
  price_point_id  = data.apple_subscription_price_points.test.price_points[0].id

  # A subscription cannot be priced in a territory it is not available in:
  # Apple answers POST /v1/subscriptionPrices with a 409 that names neither
  # availability nor the territory. Nothing in the price references the
  # availability, so the ordering has to be declared.
  depends_on = [apple_subscription_availability.test]
}
`, name)
}

// --- import ID functions ---

func testAccSubscriptionGroupImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%s", rs.Attributes["app_id"], rs.ID), nil
	}
}

func testAccSubscriptionGroupLocalizationImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%s", rs.Attributes["group_id"], rs.ID), nil
	}
}

func testAccSubscriptionPriceImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%s", rs.Attributes["subscription_id"], rs.ID), nil
	}
}

// testAccSubscriptionParentID imports by the parent subscription rather than
// the resource's own ID, which is what the availability needs.
func testAccSubscriptionParentID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return "", err
		}
		return rs.Attributes["subscription_id"], nil
	}
}

// --- existence and destroy checks ---

// testAccCheckSubscriptionGroupExists asks Apple rather than inspecting state,
// so a group that vanished from the portal is caught.
func testAccCheckSubscriptionGroupExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("could not build API client: %w", err)
		}

		group, err := client.GetSubscriptionGroup(rs.ID)
		if err != nil {
			return fmt.Errorf("subscription group %s not found at Apple: %w", rs.ID, err)
		}
		if group.ID != rs.ID {
			return fmt.Errorf("Apple returned group %s, expected %s", group.ID, rs.ID)
		}

		return nil
	}
}

func testAccCheckSubscriptionGroupLocalizationExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("could not build API client: %w", err)
		}

		localization, err := client.GetSubscriptionGroupLocalization(rs.ID)
		if err != nil {
			return fmt.Errorf("subscription group localization %s not found at Apple: %w", rs.ID, err)
		}
		if localization.ID != rs.ID {
			return fmt.Errorf("Apple returned localization %s, expected %s", localization.ID, rs.ID)
		}

		return nil
	}
}

func testAccCheckSubscriptionExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("could not build API client: %w", err)
		}

		subscription, err := client.GetSubscription(rs.ID)
		if err != nil {
			return fmt.Errorf("subscription %s not found at Apple: %w", rs.ID, err)
		}
		if subscription.ID != rs.ID {
			return fmt.Errorf("Apple returned subscription %s, expected %s", subscription.ID, rs.ID)
		}

		return nil
	}
}

func testAccCheckSubscriptionGroupDestroy(s *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("could not build API client: %w", err)
	}

	for name, rs := range s.RootModule().Resources {
		if rs.Type != "apple_subscription_group" {
			continue
		}

		if _, err := client.GetSubscriptionGroup(rs.Primary.ID); err == nil {
			return fmt.Errorf("subscription group %s (%s) still exists at Apple", rs.Primary.ID, name)
		} else if !testAccIsNotFound(err) {
			return fmt.Errorf("unexpected error checking subscription group %s: %w", rs.Primary.ID, err)
		}
	}

	return nil
}

func testAccCheckSubscriptionGroupLocalizationDestroy(s *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("could not build API client: %w", err)
	}

	for name, rs := range s.RootModule().Resources {
		if rs.Type != "apple_subscription_group_localization" {
			continue
		}

		if _, err := client.GetSubscriptionGroupLocalization(rs.Primary.ID); err == nil {
			return fmt.Errorf("subscription group localization %s (%s) still exists at Apple", rs.Primary.ID, name)
		} else if !testAccIsNotFound(err) {
			return fmt.Errorf("unexpected error checking subscription group localization %s: %w", rs.Primary.ID, err)
		}
	}

	return nil
}

func testAccCheckSubscriptionDestroy(s *terraform.State) error {
	client, err := testAccAPIClient()
	if err != nil {
		return fmt.Errorf("could not build API client: %w", err)
	}

	for name, rs := range s.RootModule().Resources {
		if rs.Type != "apple_subscription" {
			continue
		}

		if _, err := client.GetSubscription(rs.Primary.ID); err == nil {
			return fmt.Errorf("subscription %s (%s) still exists at Apple", rs.Primary.ID, name)
		} else if !testAccIsNotFound(err) {
			return fmt.Errorf("unexpected error checking subscription %s: %w", rs.Primary.ID, err)
		}
	}

	return nil
}

// testAccIsNotFound reports whether an API error means the resource is gone,
// as opposed to the request having failed for some other reason.
func testAccIsNotFound(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not found") || strings.Contains(msg, "404")
}

// TestAccSubscriptionPriceScheduleResource_basic covers the bulk price write.
//
// The point of the resource is that pricing N territories is one request rather
// than N, so the configuration goes through the equalization data source the way
// a real one does: one price point looked up by customer price, the rest derived
// from it. Prices are not deletable in their own right -- Apple removes only
// future price changes -- so the destroy check is the subscription's, which
// takes its prices with it.
func TestAccSubscriptionPriceScheduleResource_basic(t *testing.T) {
	referenceName := "Terraform Schedule " + acctest.RandString(6)
	productID := testAccSubscriptionProductID()
	name := testAccProductName("Pro Schedule")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionPriceScheduleConfig(referenceName, productID, name, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSubscriptionExists("apple_subscription.test"),
					// id mirrors subscription_id: a subscription has no price
					// schedule record of its own at Apple.
					resource.TestCheckResourceAttrPair(
						"apple_subscription_price_schedule.test", "id",
						"apple_subscription.test", "id"),
					resource.TestCheckResourceAttr("apple_subscription_price_schedule.test", "prices.#", "2"),
					// The equalization lookup must have yielded a GBR point, or
					// the second price could not have been written at all.
					resource.TestCheckResourceAttrSet(
						"data.apple_subscription_price_point_equalizations.test", "price_point_ids.GBR"),
					// Both prices reached Apple in a single PATCH.
					testAccCheckSubscriptionPriceCount("apple_subscription_price_schedule.test", 2),
				),
			},
			// Import verifies against Apple rather than against the
			// configuration: Apple reports a territory and a plan type on every
			// price whether or not one was configured, so an imported set is
			// richer than the one that was written and ImportStateVerify would
			// read that as a mismatch.
			{
				ResourceName:      "apple_subscription_price_schedule.test",
				ImportState:       true,
				ImportStateIdFunc: testAccSubscriptionParentID("apple_subscription_price_schedule.test"),
				ImportStateCheck:  testAccCheckImportedPriceCount(2),
			},
			// Adding a price rewrites the whole set in place: the schedule is
			// replaced wholesale on every write, so a scheduled increase is an
			// entry in the same resource rather than a resource of its own.
			{
				Config: testAccSubscriptionPriceScheduleConfig(referenceName, productID, name, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_subscription_price_schedule.test", "prices.#", "3"),
					testAccCheckSubscriptionPriceCount("apple_subscription_price_schedule.test", 3),
				),
			},
		},
	})
}

// testAccSubscriptionPriceScheduleConfig prices a subscription in two
// territories from one looked-up price point, optionally adding a future
// increase in the base territory.
func testAccSubscriptionPriceScheduleConfig(referenceName, productID, name string, withIncrease bool) string {
	increase := ""
	if withIncrease {
		increase = `,
    {
      territory_id           = "USA"
      price_point_id         = data.apple_subscription_price_points.increase.price_points[0].id
      start_date             = "2030-01-01"
      preserve_current_price = true
    }`
	}

	return testAccSubscriptionConfig(referenceName, productID, name, "ONE_MONTH") + fmt.Sprintf(`
data "apple_subscription_price_points" "test" {
  subscription_id = apple_subscription.test.id
  territories     = ["USA"]
  customer_price  = "0.99"
  limit           = 1
}

data "apple_subscription_price_points" "increase" {
  subscription_id = apple_subscription.test.id
  territories     = ["USA"]
  customer_price  = "1.99"
  limit           = 1
}

# What 0.99 in the United States is worth elsewhere. This is the lookup that
# makes pricing every storefront from one number possible, and the reason the
# schedule resource exists: the fan-out is a set, not a resource per territory.
data "apple_subscription_price_point_equalizations" "test" {
  price_point_id = data.apple_subscription_price_points.test.price_points[0].id
  territories    = ["GBR"]
}

resource "apple_subscription_availability" "test" {
  subscription_id       = apple_subscription.test.id
  available_territories = ["USA", "GBR"]
}

resource "apple_subscription_price_schedule" "test" {
  subscription_id = apple_subscription.test.id

  prices = [
    {
      territory_id   = "USA"
      price_point_id = data.apple_subscription_price_points.test.price_points[0].id
    },
    {
      territory_id   = "GBR"
      price_point_id = data.apple_subscription_price_point_equalizations.test.price_point_ids["GBR"]
    }%[1]s
  ]

  # A subscription cannot be priced in a territory it is not available in, and
  # Apple's rejection names neither availability nor the territory. Nothing in
  # a price references the availability, so the ordering has to be declared.
  depends_on = [apple_subscription_availability.test]
}
`, increase)
}

// testAccCheckSubscriptionPriceCount asks Apple how many prices the
// subscription actually carries.
//
// A check that only inspects Terraform state would pass on a PATCH that Apple
// accepted but committed partially, which is the failure mode a bulk write has
// and a per-territory one does not.
func testAccCheckSubscriptionPriceCount(resourceName string, want int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("could not build API client: %w", err)
		}

		subscriptionID := rs.Attributes["subscription_id"]
		prices, err := client.GetSubscriptionPrices(subscriptionID)
		if err != nil {
			return fmt.Errorf("reading prices of subscription %s: %w", subscriptionID, err)
		}

		if len(prices) != want {
			return fmt.Errorf("subscription %s has %d prices at Apple, want %d",
				subscriptionID, len(prices), want)
		}

		return nil
	}
}

// testAccCheckImportedPriceCount asserts the shape of an imported schedule.
func testAccCheckImportedPriceCount(want int) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("imported %d instances, want 1", len(states))
		}

		got := states[0].Attributes["prices.#"]
		if got != fmt.Sprint(want) {
			return fmt.Errorf("imported schedule has %q prices, want %d", got, want)
		}

		if states[0].Attributes["subscription_id"] != states[0].ID {
			return fmt.Errorf("imported subscription_id %q does not match the ID %q",
				states[0].Attributes["subscription_id"], states[0].ID)
		}

		return nil
	}
}

// --- introductory offers ---

func TestAccSubscriptionIntroductoryOfferResource_basic(t *testing.T) {
	referenceName := "Terraform Intro " + acctest.RandString(6)
	productID := testAccSubscriptionProductID()
	name := testAccProductName("Pro Intro")
	var offerID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionIntroductoryOfferConfig(referenceName, productID, name, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIntroductoryOfferCount("apple_subscription_introductory_offer.test", 1),
					resource.TestCheckResourceAttr("apple_subscription_introductory_offer.test", "offer_mode", "FREE_TRIAL"),
					resource.TestCheckNoResourceAttr("apple_subscription_introductory_offer.test", "price_point_id"),
					resource.TestCheckResourceAttrWith("apple_subscription_introductory_offer.test", "id", func(value string) error {
						offerID = value
						return nil
					}),
				),
			},
			{
				ResourceName:      "apple_subscription_introductory_offer.test",
				ImportState:       true,
				ImportStateIdFunc: testAccSubscriptionPriceImportID("apple_subscription_introductory_offer.test"),
				ImportStateVerify: true,
			},
			// end_date is the one attribute Apple's update request carries, so
			// setting it must keep the offer rather than replace it.
			{
				Config: testAccSubscriptionIntroductoryOfferConfig(referenceName, productID, name, "2030-12-31"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_subscription_introductory_offer.test", "end_date", "2030-12-31"),
					resource.TestCheckResourceAttrWith("apple_subscription_introductory_offer.test", "id", func(value string) error {
						if value != offerID {
							return fmt.Errorf("offer replaced: id %s, was %s", value, offerID)
						}
						return nil
					}),
				),
			},
		},
	})
}

// TestAccSubscriptionIntroductoryOfferResource_freeTrialRejectsPricePoint is a
// plan-time refusal, so it writes nothing.
func TestAccSubscriptionIntroductoryOfferResource_freeTrialRejectsPricePoint(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + `
resource "apple_subscription_introductory_offer" "test" {
  subscription_id   = "6740021496"
  territory_id      = "USA"
  offer_mode        = "FREE_TRIAL"
  duration          = "ONE_WEEK"
  number_of_periods = 1
  price_point_id    = "eyJzIjoiNjc0MDAyMTQ5NiJ9"
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("Price Point Does Not Fit The Offer Mode"),
			},
		},
	})
}

func TestAccSubscriptionIntroductoryOfferScheduleResource_basic(t *testing.T) {
	referenceName := "Terraform Intro Schedule " + acctest.RandString(6)
	productID := testAccSubscriptionProductID()
	name := testAccProductName("Pro Intro Schedule")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSubscription(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testAccCheckDestroyAll(
			testAccCheckSubscriptionDestroy,
			testAccCheckSubscriptionGroupDestroy,
		),
		Steps: []resource.TestStep{
			{
				Config: testAccSubscriptionIntroductoryOfferScheduleConfig(referenceName, productID, name, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"apple_subscription_introductory_offer_schedule.test", "id",
						"apple_subscription.test", "id"),
					resource.TestCheckResourceAttr("apple_subscription_introductory_offer_schedule.test", "offers.#", "2"),
					// Both offers reached Apple in a single PATCH.
					testAccCheckIntroductoryOfferCount("apple_subscription_introductory_offer_schedule.test", 2),
				),
			},
			{
				ResourceName:      "apple_subscription_introductory_offer_schedule.test",
				ImportState:       true,
				ImportStateIdFunc: testAccSubscriptionParentID("apple_subscription_introductory_offer_schedule.test"),
				ImportStateCheck:  testAccCheckImportedOfferCount(2),
			},
			// The GBR offer changes mode, which Apple cannot do in place: the
			// old one has to be deleted before the new one is created, since a
			// territory holds one introductory offer at a time.
			{
				Config: testAccSubscriptionIntroductoryOfferScheduleConfig(referenceName, productID, name, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apple_subscription_introductory_offer_schedule.test", "offers.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("apple_subscription_introductory_offer_schedule.test", "offers.*", map[string]string{
						"territory_id": "GBR",
						"offer_mode":   "PAY_UP_FRONT",
					}),
					testAccCheckIntroductoryOfferCount("apple_subscription_introductory_offer_schedule.test", 2),
				),
			},
		},
	})
}

// testAccSubscriptionIntroductoryOfferConfig adds a USA free trial to a
// complete subscription, optionally with an end date.
func testAccSubscriptionIntroductoryOfferConfig(referenceName, productID, name, endDate string) string {
	end := ""
	if endDate != "" {
		end = fmt.Sprintf("\n  end_date = %q", endDate)
	}

	return testAccSubscriptionCompleteConfig(referenceName, productID, name) + fmt.Sprintf(`
resource "apple_subscription_introductory_offer" "test" {
  subscription_id   = apple_subscription.test.id
  territory_id      = "USA"
  offer_mode        = "FREE_TRIAL"
  duration          = "ONE_WEEK"
  number_of_periods = 1%[1]s

  depends_on = [apple_subscription_availability.test, apple_subscription_price.test]
}
`, end)
}

// testAccSubscriptionIntroductoryOfferScheduleConfig prices a subscription in
// USA and GBR and gives it a free trial in one and a paid offer in the other.
func testAccSubscriptionIntroductoryOfferScheduleConfig(referenceName, productID, name string, payUpFront bool) string {
	gbrOffer := `{
      territory_id      = "GBR"
      offer_mode        = "PAY_AS_YOU_GO"
      duration          = "ONE_MONTH"
      number_of_periods = 2
      price_point_id    = data.apple_subscription_price_point_equalizations.test.price_point_ids["GBR"]
    }`
	if payUpFront {
		gbrOffer = `{
      territory_id      = "GBR"
      offer_mode        = "PAY_UP_FRONT"
      duration          = "THREE_MONTHS"
      number_of_periods = 1
      price_point_id    = data.apple_subscription_price_point_equalizations.test.price_point_ids["GBR"]
    }`
	}

	return testAccSubscriptionConfig(referenceName, productID, name, "ONE_MONTH") + fmt.Sprintf(`
data "apple_subscription_price_points" "test" {
  subscription_id = apple_subscription.test.id
  territories     = ["USA"]
  customer_price  = "0.99"
  limit           = 1
}

data "apple_subscription_price_point_equalizations" "test" {
  price_point_id = data.apple_subscription_price_points.test.price_points[0].id
  territories    = ["GBR"]
}

resource "apple_subscription_availability" "test" {
  subscription_id       = apple_subscription.test.id
  available_territories = ["USA", "GBR"]
}

resource "apple_subscription_price_schedule" "test" {
  subscription_id = apple_subscription.test.id

  prices = [
    {
      territory_id   = "USA"
      price_point_id = data.apple_subscription_price_points.test.price_points[0].id
    },
    {
      territory_id   = "GBR"
      price_point_id = data.apple_subscription_price_point_equalizations.test.price_point_ids["GBR"]
    },
  ]

  depends_on = [apple_subscription_availability.test]
}

resource "apple_subscription_introductory_offer_schedule" "test" {
  subscription_id = apple_subscription.test.id

  offers = [
    {
      territory_id      = "USA"
      offer_mode        = "FREE_TRIAL"
      duration          = "ONE_WEEK"
      number_of_periods = 1
    },
    %[1]s,
  ]

  depends_on = [apple_subscription_price_schedule.test]
}
`, gbrOffer)
}

// testAccCheckIntroductoryOfferCount asks Apple how many introductory offers
// the subscription actually carries, which is what catches a bulk write Apple
// committed partially, or an append that left an old offer behind.
func testAccCheckIntroductoryOfferCount(resourceName string, want int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, err := testAccStateResource(s, resourceName)
		if err != nil {
			return err
		}

		client, err := testAccAPIClient()
		if err != nil {
			return fmt.Errorf("could not build API client: %w", err)
		}

		subscriptionID := rs.Attributes["subscription_id"]
		offers, err := client.GetSubscriptionIntroductoryOffers(subscriptionID, nil)
		if err != nil {
			return fmt.Errorf("reading introductory offers of subscription %s: %w", subscriptionID, err)
		}

		if len(offers) != want {
			return fmt.Errorf("subscription %s has %d introductory offers at Apple, want %d",
				subscriptionID, len(offers), want)
		}

		return nil
	}
}

// testAccCheckImportedOfferCount asserts the shape of an imported schedule.
func testAccCheckImportedOfferCount(want int) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("imported %d instances, want 1", len(states))
		}

		if got := states[0].Attributes["offers.#"]; got != fmt.Sprint(want) {
			return fmt.Errorf("imported schedule has %q offers, want %d", got, want)
		}

		return nil
	}
}
