data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_subscription_group" "premium" {
  app_id         = data.apple_apps.example.apps[0].id
  reference_name = "Premium"
}

resource "apple_subscription" "pro_monthly" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.pro.monthly"
  name                = "Pro Monthly"
  subscription_period = "ONE_MONTH"
}

# An offer is for one territory, and the subscription has to be available
# there. Nothing in an offer references the availability, so the dependency is
# declared by hand below.
resource "apple_subscription_availability" "pro_monthly" {
  subscription_id       = apple_subscription.pro_monthly.id
  available_territories = ["USA", "GBR"]
}

# A one-week free trial for new subscribers in the United States. A free trial
# references no price point.
resource "apple_subscription_introductory_offer" "pro_monthly_usa_trial" {
  subscription_id   = apple_subscription.pro_monthly.id
  territory_id      = "USA"
  offer_mode        = "FREE_TRIAL"
  duration          = "ONE_WEEK"
  number_of_periods = 1

  # The one attribute that changes in place: moving or removing the end date
  # updates this offer rather than replacing it.
  end_date = "2027-03-31"

  depends_on = [apple_subscription_availability.pro_monthly]
}

# Three months at a reduced price in the United Kingdom. A paid offer is a
# price point like any other price, read from the subscription's catalogue.
data "apple_subscription_price_points" "gbr_intro" {
  subscription_id = apple_subscription.pro_monthly.id
  territories     = ["GBR"]
  customer_price  = "0.99"
}

resource "apple_subscription_introductory_offer" "pro_monthly_gbr_intro" {
  subscription_id   = apple_subscription.pro_monthly.id
  territory_id      = "GBR"
  offer_mode        = "PAY_AS_YOU_GO"
  duration          = "ONE_MONTH"
  number_of_periods = 3
  price_point_id    = data.apple_subscription_price_points.gbr_intro.price_points[0].id

  depends_on = [apple_subscription_availability.pro_monthly]
}
