# Introductory offers import as "<subscription_id>/<offer_id>".
#
# The subscription ID is required because Apple publishes no GET for a single
# introductory offer: an offer is only reachable through the collection hanging
# off its subscription.
terraform import apple_subscription_introductory_offer.pro_monthly_usa_trial 6739472901/9923456702
