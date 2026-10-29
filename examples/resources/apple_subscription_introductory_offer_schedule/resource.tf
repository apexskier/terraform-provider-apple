data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_subscription_group" "premium" {
  app_id         = data.apple_apps.example.apps[0].id
  reference_name = "Premium"
}

resource "apple_subscription" "pro_annual" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.pro.annual"
  name                = "Pro Annual"
  subscription_period = "ONE_YEAR"
}

data "apple_territories" "all" {}

# Every territory an offer names has to be one the subscription sells in.
resource "apple_subscription_availability" "pro_annual" {
  subscription_id       = apple_subscription.pro_annual.id
  available_territories = data.apple_territories.all.ids
}

# The first three months at a reduced price in the United States, paid once.
data "apple_subscription_price_points" "usa_intro" {
  subscription_id = apple_subscription.pro_annual.id
  territories     = ["USA"]
  customer_price  = "19.99"
}

# A two-week free trial in every storefront except the United States, which
# gets the paid offer instead -- a territory holds one introductory offer at a
# time. Created in one API call, where the same fan-out through
# apple_subscription_introductory_offer is one resource and one POST per
# storefront.
resource "apple_subscription_introductory_offer_schedule" "pro_annual" {
  subscription_id = apple_subscription.pro_annual.id

  offers = concat(
    [
      for territory in data.apple_territories.all.ids : {
        territory_id      = territory
        offer_mode        = "FREE_TRIAL"
        duration          = "TWO_WEEKS"
        number_of_periods = 1
        # Spelled out so both halves of the concat have the same shape.
        price_point_id = null
      } if territory != "USA"
    ],
    [
      {
        territory_id      = "USA"
        offer_mode        = "PAY_UP_FRONT"
        duration          = "THREE_MONTHS"
        number_of_periods = 1
        price_point_id    = data.apple_subscription_price_points.usa_intro.price_points[0].id
      },
    ],
  )

  depends_on = [apple_subscription_availability.pro_annual]
}
