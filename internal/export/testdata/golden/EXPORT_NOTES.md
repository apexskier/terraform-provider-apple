# Export notes

This configuration was written by `apple terraform export` from what App Store Connect holds.
Every resource has an `import` block beside it, so nothing is created:

```sh
terraform init
terraform plan
```

The plan should report **34 to import, 0 to add, 0 to change, 0 to destroy**. Anything else is something the export got wrong. Do not apply a plan that destroys or replaces anything; fix the configuration first. Once applied, the `import` blocks can be deleted.

## Imported

| Resource | Count |
|---|---|
| `apple_app_age_rating_declaration` | 1 |
| `apple_app_availability` | 1 |
| `apple_app_info` | 1 |
| `apple_app_info_localization` | 1 |
| `apple_app_price_schedule` | 1 |
| `apple_app_settings` | 1 |
| `apple_app_store_review_detail` | 1 |
| `apple_app_store_version` | 2 |
| `apple_app_store_version_localization` | 1 |
| `apple_beta_app_localization` | 1 |
| `apple_beta_app_review_detail` | 1 |
| `apple_beta_group` | 2 |
| `apple_beta_tester` | 1 |
| `apple_bundle_id` | 1 |
| `apple_bundle_id_capability` | 2 |
| `apple_device` | 1 |
| `apple_in_app_purchase` | 1 |
| `apple_in_app_purchase_localization` | 1 |
| `apple_in_app_purchase_price_schedule` | 1 |
| `apple_merchant_id` | 1 |
| `apple_profile` | 2 |
| `apple_subscription` | 1 |
| `apple_subscription_availability` | 1 |
| `apple_subscription_group` | 1 |
| `apple_subscription_group_localization` | 1 |
| `apple_subscription_introductory_offer_schedule` | 1 |
| `apple_subscription_localization` | 1 |
| `apple_subscription_price_schedule` | 1 |
| `apple_user` | 1 |
| `apple_user_invitation` | 1 |

## Needs attention

- `apple_bundle_id.com_example_app`: platform is UNIVERSAL because that is what Apple reports for every Bundle ID. If this is an iOS or macOS app, you can change it to IOS or MAC_OS; that change is recorded in place and does not replace the Bundle ID.
- `data.apple_certificates.example_distribution_distribution`: Certificate "Example Distribution" (serial 6A1B2C3D4E5F) is read through a data source rather than imported: importing an apple_certificate reissues it, which revokes the one you are signing with. To have Terraform issue its successor, see the Code Signing guide.
- `apple_app_age_rating_declaration.com_example_app`: 9 age rating questions are unanswered at Apple and are left out here. Leaving one out keeps Apple's answer rather than answering NONE, and an unanswered questionnaire blocks submission.
- `apple_app_store_version.com_example_app_ios`: Apple holds an earliest release date of 2026-11-01T08:00:00Z, which an import does not record. Add earliest_release_date to keep it; leaving it out leaves Apple's value alone.
- `apple_app_store_review_detail.com_example_app_ios`: A demo account is required, and Apple does not return its password. Set var.app_review_com_example_app_ios_demo_password (it is sensitive, and still stored in state in plain text); until it is set, Terraform leaves the password Apple holds alone.
- `apple_app_store_version.com_example_app_mac_os`: com.example.app 2.0 is READY_FOR_DISTRIBUTION, so Apple accepts no changes to it. Changing version_string replaces it with a new version, which is how the next release is cut.
- `apple_app_price_schedule.com_example_app`: This changes what com.example.app costs. Apple publishes no DELETE for a price schedule, so a change applied here cannot be undone by removing it.
- `apple_subscription_price_schedule.com_example_app_monthly`: Every price Apple holds for com.example.app.monthly is listed, past ones included. Apple deletes only future price changes, so to change a price add a new one with a later start_date rather than editing or removing a live one.
- `apple_user_invitation.new_example_com`: The invitation to new@example.com is pending and lapses at 2026-10-23T00:00:00Z. Once it is accepted Apple deletes it; replace this block with an apple_user for them then.

## Not in the configuration

- Device "Old iPad" (00008030-000B000000000000) is disabled, so it was left out.
- Profile "Example Expired" is EXPIRED, so it was left out.
- com.example.app 1.10 (IOS): screenshot sets were left out. Apple keeps a screenshot's bytes, not the file it came from, so an apple_app_screenshot_set needs the image files on disk; add one per display type pointing at them.
- com.example.app: App Privacy, app preview videos, the build upload, export compliance and submission have no resource; see remaining_manual_steps in examples/app-listing.
- com.example.app: App Review screenshots of subscriptions and in-app purchases were left out. Apple keeps the bytes, not the file, so they need the image on disk; add an apple_subscription_review_screenshot or apple_in_app_purchase_review_screenshot pointing at it.
- `apple_beta_group.com_example_app_public_beta`: 1 testers who joined "Public Beta" through its public link were left out.
- com.example.app: builds given to beta groups (apple_beta_group_build) and per-build "What to Test" notes (apple_beta_build_localization) were left out. Both belong to one build and change with every upload, so they are better written next to the pipeline that produces the build.
- owner@example.com is the Account Holder, which the API cannot manage, so they were left out.
