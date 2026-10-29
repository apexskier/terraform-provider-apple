// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"fmt"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// fakeAccount is a Source over canned records: one team with one app that
// has a little of everything, including the cases the export has to treat
// specially.
type fakeAccount struct {
	calls []string
}

func ptr[T any](v T) *T { return &v }

func id(kind, value string) *models.ResourceIdentifier {
	return &models.ResourceIdentifier{Data: models.ResourceData{Type: kind, ID: value}}
}

var errNotFound = fmt.Errorf("API error (status 404): not found")

const (
	appID     = "6478123456"
	bundleRef = "B2C3D4E5F6"
)

func (f *fakeAccount) GetBundleIDs() ([]models.BundleID, error) {
	return []models.BundleID{{ID: bundleRef, Attributes: models.BundleIDAttributes{
		Identifier: "com.example.app", Name: "Example App", Platform: models.UNIVERSAL, SeedID: "TEAM123456",
	}}}, nil
}

func (f *fakeAccount) GetBundleIDCapabilities(bundleID string) ([]models.BundleIDCapability, error) {
	return []models.BundleIDCapability{
		{ID: bundleID + "_PUSH_NOTIFICATIONS", Attributes: models.BundleIDCapabilityAttributes{CapabilityType: models.PushNotifications}},
		{ID: bundleID + "_ICLOUD", Attributes: models.BundleIDCapabilityAttributes{
			CapabilityType: models.ICloud,
			Settings: []models.CapabilitySetting{{
				Key:     "ICLOUD_VERSION",
				Options: []models.CapabilitySettingOption{{Key: "XCODE_6"}},
			}},
		}},
	}, nil
}

func (f *fakeAccount) GetDevices() ([]models.Device, error) {
	return []models.Device{
		{ID: "DEV1", Attributes: models.DeviceAttributes{Name: "Ada's iPhone", UDID: "00008030-000A4D8E0AB8802E", Platform: models.DeviceIOS, Status: models.DeviceStatusENABLED}},
		{ID: "DEV2", Attributes: models.DeviceAttributes{Name: "Old iPad", UDID: "00008030-000B000000000000", Platform: models.DeviceIOS, Status: models.DeviceStatusDISABLED}},
	}, nil
}

func (f *fakeAccount) GetMerchantIDs() ([]models.MerchantID, error) {
	return []models.MerchantID{{ID: "4B62XU6945", Attributes: models.MerchantIDAttributes{Identifier: "merchant.com.example.app", DisplayName: "Example Pay"}}}, nil
}

func (f *fakeAccount) GetPassTypeIDs() ([]models.PassTypeIDResource, error) { return nil, nil }

func (f *fakeAccount) GetCertificates() ([]models.Certificate, error) {
	return []models.Certificate{{ID: "CERT1", Attributes: models.CertificateAttributes{
		SerialNumber: "6A1B2C3D4E5F", DisplayName: "Example Distribution", CertificateType: models.Distribution,
	}}}, nil
}

func (f *fakeAccount) GetProfiles() ([]models.Profile, error) {
	return []models.Profile{
		{ID: "PROF1", Attributes: models.ProfileAttributes{Name: "Example App Store", ProfileType: ptr(models.ProfileTypeAppStore), ProfileState: ptr(models.ProfileStateActive)}},
		{ID: "PROF2", Attributes: models.ProfileAttributes{Name: "Example Ad Hoc", ProfileType: ptr(models.ProfileTypeAdhoc), ProfileState: ptr(models.ProfileStateActive)}},
		{ID: "PROF3", Attributes: models.ProfileAttributes{Name: "Example Expired", ProfileType: ptr(models.ProfileTypeAdhoc), ProfileState: ptr(models.ProfileStateExpired)}},
	}, nil
}

func (f *fakeAccount) GetProfileLinkages(profileID string) (*apple.ProfileLinkages, error) {
	switch profileID {
	case "PROF1":
		return &apple.ProfileLinkages{BundleID: bundleRef, CertificateIDs: []string{"CERT1"}}, nil
	default:
		// The second certificate has been revoked, so no data source names it.
		return &apple.ProfileLinkages{BundleID: bundleRef, CertificateIDs: []string{"CERT1", "CERT_GONE"}, DeviceIDs: []string{"DEV1"}}, nil
	}
}

func (f *fakeAccount) app() models.App {
	return models.App{ID: appID, Attributes: models.AppAttributes{
		Name: "Example", BundleID: "com.example.app", SKU: "EXAMPLE", PrimaryLocale: "en-US",
		ContentRightsDeclaration: ptr("DOES_NOT_USE_THIRD_PARTY_CONTENT"),
		SubscriptionStatusURL:    ptr("https://example.com/asn"),
	}}
}

func (f *fakeAccount) GetApps() ([]models.App, error)     { return []models.App{f.app()}, nil }
func (f *fakeAccount) GetApp(string) (*models.App, error) { a := f.app(); return &a, nil }
func (f *fakeAccount) GetAppByBundleID(string) (*models.App, error) {
	a := f.app()
	return &a, nil
}

func (f *fakeAccount) GetEditableAppInfo(string) (*models.AppInfo, error) {
	return &models.AppInfo{ID: "INFO1", Relationships: &models.AppInfoRelationships{
		PrimaryCategory:   id("appCategories", "PRODUCTIVITY"),
		SecondaryCategory: id("appCategories", ""),
	}}, nil
}

func (f *fakeAccount) GetAppInfoLocalizations(string) ([]models.AppInfoLocalization, error) {
	return []models.AppInfoLocalization{{ID: "INFOLOC1", Attributes: models.AppInfoLocalizationAttributes{
		Locale: "en-US", Name: "Example", Subtitle: ptr("Gets things ${done}"), PrivacyPolicyURL: ptr("https://example.com/privacy"),
	}}}, nil
}

func (f *fakeAccount) GetAgeRatingDeclaration(string) (*models.AgeRatingDeclaration, error) {
	return &models.AgeRatingDeclaration{ID: "AGE1", Attributes: models.AgeRatingDeclarationAttributes{
		Contests: ptr("NONE"), ViolenceRealistic: ptr("NONE"), Gambling: ptr(false), UnrestrictedWebAccess: ptr(false),
	}}, nil
}

func (f *fakeAccount) GetAppStoreVersions(string, string) ([]models.AppStoreVersion, error) {
	return []models.AppStoreVersion{
		{ID: "VER_OLD", Attributes: models.AppStoreVersionAttributes{Platform: "IOS", VersionString: "1.9", AppVersionState: ptr("READY_FOR_DISTRIBUTION")}},
		{ID: "VER_NEW", Attributes: models.AppStoreVersionAttributes{
			Platform: "IOS", VersionString: "1.10", AppVersionState: ptr("PREPARE_FOR_SUBMISSION"),
			Copyright: ptr("2026 Example Ltd"), ReleaseType: ptr("MANUAL"), EarliestReleaseDate: ptr("2026-11-01T08:00:00Z"),
		}},
		{ID: "VER_MAC", Attributes: models.AppStoreVersionAttributes{Platform: "MAC_OS", VersionString: "2.0", AppVersionState: ptr("READY_FOR_DISTRIBUTION")}},
	}, nil
}

func (f *fakeAccount) GetAppStoreVersionBuild(versionID string) (*models.Build, error) {
	if versionID == "VER_NEW" {
		return &models.Build{ID: "BUILD42", Attributes: models.BuildAttributes{Version: ptr("42")}}, nil
	}
	return nil, nil
}

func (f *fakeAccount) GetAppStoreVersionLocalizations(versionID string) ([]models.AppStoreVersionLocalization, error) {
	if versionID != "VER_NEW" {
		return nil, nil
	}
	return []models.AppStoreVersionLocalization{{ID: "VERLOC1", Attributes: models.AppStoreVersionLocalizationAttributes{
		Locale: "en-US", Description: ptr("Line one.\nLine two."), Keywords: ptr("tasks, todo,lists"), SupportURL: ptr("https://example.com/help"),
	}}}, nil
}

func (f *fakeAccount) GetAppStoreReviewDetail(versionID string) (*models.AppStoreReviewDetail, error) {
	if versionID != "VER_NEW" {
		return nil, errNotFound
	}
	return &models.AppStoreReviewDetail{ID: "REV1", Attributes: models.AppStoreReviewDetailAttributes{
		ContactEmail: ptr("review@example.com"), DemoAccountName: ptr("demo"), DemoAccountPassword: ptr("••••"), DemoAccountRequired: ptr(true),
	}}, nil
}

func (f *fakeAccount) GetAppPrices(string) (*models.AppPriceSchedule, []models.AppPrice, error) {
	return &models.AppPriceSchedule{ID: appID, Relationships: &models.AppPriceScheduleRelationships{BaseTerritory: id("territories", "USA")}},
		[]models.AppPrice{{ID: "P1", Relationships: &models.AppPriceRelationships{AppPricePoint: id("appPricePoints", "eyJzIjoiMTAwMDAifQ")}}}, nil
}

func (f *fakeAccount) GetAppAvailabilityWithTerritories(string) (*models.AppAvailability, []models.TerritoryAvailability, error) {
	return &models.AppAvailability{ID: "AVAIL1", Attributes: models.AppAvailabilityAttributes{AvailableInNewTerritories: ptr(true)}},
		[]models.TerritoryAvailability{
			{ID: "TA2", Attributes: models.TerritoryAvailabilityAttributes{Available: ptr(true)}, Relationships: &models.TerritoryAvailabilityRelationships{Territory: id("territories", "USA")}},
			{ID: "TA1", Attributes: models.TerritoryAvailabilityAttributes{Available: ptr(true), ReleaseDate: ptr("2025-01-01")}, Relationships: &models.TerritoryAvailabilityRelationships{Territory: id("territories", "GBR")}},
			{ID: "TA3", Attributes: models.TerritoryAvailabilityAttributes{Available: ptr(false)}, Relationships: &models.TerritoryAvailabilityRelationships{Territory: id("territories", "CHN")}},
		}, nil
}

func (f *fakeAccount) GetSubscriptionGroups(string) ([]models.SubscriptionGroup, error) {
	return []models.SubscriptionGroup{{ID: "GRP1", Attributes: models.SubscriptionGroupAttributes{ReferenceName: "Premium"}}}, nil
}

func (f *fakeAccount) GetSubscriptionGroupLocalizations(string) ([]models.SubscriptionGroupLocalization, error) {
	return []models.SubscriptionGroupLocalization{{ID: "GRPLOC1", Attributes: models.SubscriptionGroupLocalizationAttributes{Locale: "en-US", Name: "Example Premium"}}}, nil
}

func (f *fakeAccount) GetSubscriptions(string) ([]models.Subscription, error) {
	return []models.Subscription{{ID: "SUB1", Attributes: models.SubscriptionAttributes{
		Name: "Monthly", ProductID: "com.example.app.monthly", SubscriptionPeriod: ptr(models.SubscriptionPeriod("ONE_MONTH")),
		FamilySharable: ptr(false), GroupLevel: ptr(1), ReviewNote: ptr("Use the demo account."),
	}}}, nil
}

func (f *fakeAccount) GetSubscriptionLocalizations(string) ([]models.SubscriptionLocalization, error) {
	return []models.SubscriptionLocalization{{ID: "SUBLOC1", Attributes: models.SubscriptionLocalizationAttributes{Locale: "en-US", Name: "Monthly", Description: ptr("Everything, monthly.")}}}, nil
}

func (f *fakeAccount) GetSubscriptionPrices(string) ([]models.SubscriptionPrice, error) {
	return []models.SubscriptionPrice{
		{ID: "SP2", Attributes: models.SubscriptionPriceAttributes{StartDate: ptr("2026-12-01")}, Relationships: &models.SubscriptionPriceRelationships{SubscriptionPricePoint: id("subscriptionPricePoints", "spp-gbr"), Territory: id("territories", "GBR")}},
		{ID: "SP1", Relationships: &models.SubscriptionPriceRelationships{SubscriptionPricePoint: id("subscriptionPricePoints", "spp-usa"), Territory: id("territories", "USA")}},
	}, nil
}

func (f *fakeAccount) GetSubscriptionAvailability(string) (*models.SubscriptionAvailability, error) {
	return &models.SubscriptionAvailability{ID: "SUBAVAIL1"}, nil
}

func (f *fakeAccount) GetSubscriptionAvailableTerritories(string) ([]models.Territory, error) {
	return []models.Territory{{ID: "USA"}, {ID: "GBR"}}, nil
}

func (f *fakeAccount) GetSubscriptionIntroductoryOffers(string, []string) ([]models.SubscriptionIntroductoryOffer, error) {
	return []models.SubscriptionIntroductoryOffer{
		{ID: "SIO2", Attributes: models.SubscriptionIntroductoryOfferAttributes{
			OfferMode: ptr(models.SubscriptionOfferModePayAsYouGo), Duration: ptr(models.SubscriptionOfferDurationOneMonth), NumberOfPeriods: ptr(3),
			StartDate: ptr("2026-11-01"), EndDate: ptr("2027-01-31"),
		}, Relationships: &models.SubscriptionIntroductoryOfferRelationships{Territory: id("territories", "GBR"), SubscriptionPricePoint: id("subscriptionPricePoints", "spp-gbr-intro")}},
		{ID: "SIO1", Attributes: models.SubscriptionIntroductoryOfferAttributes{
			OfferMode: ptr(models.SubscriptionOfferModeFreeTrial), Duration: ptr(models.SubscriptionOfferDurationOneWeek), NumberOfPeriods: ptr(1),
		}, Relationships: &models.SubscriptionIntroductoryOfferRelationships{Territory: id("territories", "USA")}},
	}, nil
}

func (f *fakeAccount) GetInAppPurchases(string) ([]models.InAppPurchase, error) {
	return []models.InAppPurchase{{ID: "IAP1", Attributes: models.InAppPurchaseAttributes{
		Name: "Pro Unlock", ProductID: "com.example.app.pro", InAppPurchaseType: ptr(models.InAppPurchaseType("NON_CONSUMABLE")), FamilySharable: ptr(true),
	}}}, nil
}

func (f *fakeAccount) GetInAppPurchaseVersions(string) ([]models.InAppPurchaseVersion, error) {
	return []models.InAppPurchaseVersion{
		{ID: "IAPV1", Attributes: models.InAppPurchaseVersionAttributes{Version: ptr(1)}},
		{ID: "IAPV2", Attributes: models.InAppPurchaseVersionAttributes{Version: ptr(2)}},
	}, nil
}

func (f *fakeAccount) GetInAppPurchaseLocalizations(versionID string) ([]models.InAppPurchaseLocalization, error) {
	f.calls = append(f.calls, "GetInAppPurchaseLocalizations "+versionID)
	return []models.InAppPurchaseLocalization{{ID: "IAPLOC1", Attributes: models.InAppPurchaseLocalizationAttributes{Locale: "en-US", Name: "Pro"}}}, nil
}

func (f *fakeAccount) GetInAppPurchasePrices(string) (*models.InAppPurchasePriceSchedule, []models.InAppPurchasePrice, error) {
	return &models.InAppPurchasePriceSchedule{ID: "IAPSCHED1", Relationships: &models.InAppPurchasePriceScheduleRelationships{BaseTerritory: id("territories", "USA")}},
		[]models.InAppPurchasePrice{{ID: "IP1", Relationships: &models.InAppPurchasePriceRelationships{InAppPurchasePricePoint: id("inAppPurchasePricePoints", "ipp-usa")}}}, nil
}

func (f *fakeAccount) GetInAppPurchaseAvailability(string) (*models.InAppPurchaseAvailability, error) {
	return nil, fmt.Errorf("availability not found for in-app purchase 'IAP1'")
}

func (f *fakeAccount) GetInAppPurchaseAvailableTerritories(string) ([]models.Territory, error) {
	return nil, nil
}

func (f *fakeAccount) GetBetaGroups(string) ([]models.BetaGroup, error) {
	return []models.BetaGroup{
		{ID: "BG1", Attributes: models.BetaGroupAttributes{Name: ptr("Team"), IsInternalGroup: ptr(true), HasAccessToAllBuilds: ptr(true), PublicLinkEnabled: ptr(false), FeedbackEnabled: ptr(true)}},
		{ID: "BG2", Attributes: models.BetaGroupAttributes{Name: ptr("Public Beta"), IsInternalGroup: ptr(false), PublicLinkEnabled: ptr(true), PublicLinkLimitEnabled: ptr(true), PublicLinkLimit: ptr(int64(500))}},
	}, nil
}

func (f *fakeAccount) GetBetaGroupTesters(groupID string) ([]models.BetaTester, error) {
	if groupID != "BG2" {
		return nil, nil
	}
	return []models.BetaTester{
		{ID: "T1", Attributes: models.BetaTesterAttributes{Email: ptr("ada@example.com"), FirstName: ptr("Ada"), InviteType: ptr("EMAIL")}},
		{ID: "T2", Attributes: models.BetaTesterAttributes{Email: ptr("someone@example.org"), InviteType: ptr("PUBLIC_LINK")}},
	}, nil
}

func (f *fakeAccount) GetBetaAppLocalizations(string) ([]models.BetaAppLocalization, error) {
	return []models.BetaAppLocalization{{ID: "BAL1", Attributes: models.BetaAppLocalizationAttributes{Locale: "en-US", FeedbackEmail: ptr("beta@example.com")}}}, nil
}

func (f *fakeAccount) GetBetaAppReviewDetail(string) (*models.BetaAppReviewDetail, error) {
	return &models.BetaAppReviewDetail{ID: appID, Attributes: models.BetaAppReviewDetailAttributes{ContactEmail: ptr("beta@example.com"), DemoAccountRequired: ptr(false)}}, nil
}

func (f *fakeAccount) GetUsers() ([]models.User, error) {
	return []models.User{
		{ID: "U0", Attributes: models.UserAttributes{Username: ptr("owner@example.com"), Roles: []string{"ACCOUNT_HOLDER", "ADMIN"}}},
		{ID: "U1", Attributes: models.UserAttributes{Username: ptr("dev@example.com"), FirstName: ptr("Dev"), Roles: []string{"DEVELOPER", "APP_MANAGER"}, AllAppsVisible: ptr(false), ProvisioningAllowed: ptr(true)}},
	}, nil
}

func (f *fakeAccount) GetUserVisibleApps(string) ([]models.App, error) {
	return []models.App{{ID: appID}}, nil
}

func (f *fakeAccount) GetUserInvitations() ([]models.UserInvitation, error) {
	return []models.UserInvitation{{ID: "INV1", Attributes: models.UserInvitationAttributes{
		Email: ptr("new@example.com"), FirstName: ptr("New"), LastName: ptr("Hire"), Roles: []string{"MARKETING"}, AllAppsVisible: ptr(true), ExpirationDate: ptr("2026-10-23T00:00:00Z"),
	}}}, nil
}

func (f *fakeAccount) GetUserInvitationVisibleApps(string) ([]models.App, error) { return nil, nil }
