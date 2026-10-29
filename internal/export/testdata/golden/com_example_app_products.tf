# Subscriptions and in-app purchases of Example (app ID 6478123456).
# product_id forces replacement, and Apple never releases a product identifier: do not edit one.

import {
  to = apple_subscription_group.premium
  id = "6478123456/GRP1"
}
resource "apple_subscription_group" "premium" {
  app_id         = "6478123456"
  reference_name = "Premium"
}

import {
  to = apple_subscription_group_localization.premium_en_us
  id = "GRP1/GRPLOC1"
}
resource "apple_subscription_group_localization" "premium_en_us" {
  group_id = apple_subscription_group.premium.id
  locale   = "en-US"
  name     = "Example Premium"
}

import {
  to = apple_subscription.com_example_app_monthly
  id = "SUB1"
}
resource "apple_subscription" "com_example_app_monthly" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.monthly"
  name                = "Monthly"
  subscription_period = "ONE_MONTH"
  family_sharable     = false
  group_level         = 1
  review_note         = "Use the demo account."
}

import {
  to = apple_subscription_localization.com_example_app_monthly_en_us
  id = "SUB1/SUBLOC1"
}
resource "apple_subscription_localization" "com_example_app_monthly_en_us" {
  subscription_id = apple_subscription.com_example_app_monthly.id
  locale          = "en-US"
  name            = "Monthly"
  description     = "Everything, monthly."
}

import {
  to = apple_subscription_availability.com_example_app_monthly
  id = "SUB1"
}
resource "apple_subscription_availability" "com_example_app_monthly" {
  subscription_id              = apple_subscription.com_example_app_monthly.id
  available_in_new_territories = false
  available_territories        = ["GBR", "USA"]
}

import {
  to = apple_subscription_introductory_offer_schedule.com_example_app_monthly
  id = "SUB1"
}
resource "apple_subscription_introductory_offer_schedule" "com_example_app_monthly" {
  subscription_id = apple_subscription.com_example_app_monthly.id
  offers = [
    {
      duration          = "ONE_MONTH"
      end_date          = "2027-01-31"
      number_of_periods = 3
      offer_mode        = "PAY_AS_YOU_GO"
      price_point_id    = "spp-gbr-intro"
      start_date        = "2026-11-01"
      territory_id      = "GBR"
    },
    {
      duration          = "ONE_WEEK"
      number_of_periods = 1
      offer_mode        = "FREE_TRIAL"
      territory_id      = "USA"
    },
  ]
  depends_on = [apple_subscription_availability.com_example_app_monthly]
}

import {
  to = apple_subscription_price_schedule.com_example_app_monthly
  id = "SUB1"
}
resource "apple_subscription_price_schedule" "com_example_app_monthly" {
  subscription_id = apple_subscription.com_example_app_monthly.id
  prices = [
    {
      price_point_id = "spp-gbr"
      start_date     = "2026-12-01"
      territory_id   = "GBR"
    },
    {
      price_point_id = "spp-usa"
      territory_id   = "USA"
    },
  ]
  depends_on = [apple_subscription_availability.com_example_app_monthly]
}

import {
  to = apple_in_app_purchase.com_example_app_pro
  id = "6478123456/IAP1"
}
resource "apple_in_app_purchase" "com_example_app_pro" {
  app_id               = "6478123456"
  product_id           = "com.example.app.pro"
  name                 = "Pro Unlock"
  in_app_purchase_type = "NON_CONSUMABLE"
  family_sharable      = true
}

import {
  to = apple_in_app_purchase_localization.com_example_app_pro_en_us
  id = "IAP1/IAPLOC1"
}
resource "apple_in_app_purchase_localization" "com_example_app_pro_en_us" {
  in_app_purchase_id = apple_in_app_purchase.com_example_app_pro.id
  locale             = "en-US"
  name               = "Pro"
}

import {
  to = apple_in_app_purchase_price_schedule.com_example_app_pro
  id = "IAP1"
}
resource "apple_in_app_purchase_price_schedule" "com_example_app_pro" {
  in_app_purchase_id = apple_in_app_purchase.com_example_app_pro.id
  base_territory     = "USA"
  prices = [
    {
      price_point_id = "ipp-usa"
    },
  ]
}

