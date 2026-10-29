// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// Source is the part of the App Store Connect client the export reads
// through. *apple.Client is the real one; tests substitute canned records.
type Source interface {
	GetBundleIDs() ([]models.BundleID, error)
	GetBundleIDCapabilities(bundleID string) ([]models.BundleIDCapability, error)
	GetDevices() ([]models.Device, error)
	GetMerchantIDs() ([]models.MerchantID, error)
	GetPassTypeIDs() ([]models.PassTypeIDResource, error)
	GetCertificates() ([]models.Certificate, error)
	GetProfiles() ([]models.Profile, error)
	GetProfileLinkages(profileID string) (*apple.ProfileLinkages, error)

	GetApps() ([]models.App, error)
	GetApp(appID string) (*models.App, error)
	GetAppByBundleID(bundleID string) (*models.App, error)
	GetEditableAppInfo(appID string) (*models.AppInfo, error)
	GetAppInfoLocalizations(appInfoID string) ([]models.AppInfoLocalization, error)
	GetAgeRatingDeclaration(appInfoID string) (*models.AgeRatingDeclaration, error)
	GetAppStoreVersions(appID, platform string) ([]models.AppStoreVersion, error)
	GetAppStoreVersionBuild(versionID string) (*models.Build, error)
	GetAppStoreVersionLocalizations(versionID string) ([]models.AppStoreVersionLocalization, error)
	GetAppStoreReviewDetail(versionID string) (*models.AppStoreReviewDetail, error)
	GetAppPrices(appID string) (*models.AppPriceSchedule, []models.AppPrice, error)
	GetAppAvailabilityWithTerritories(appID string) (*models.AppAvailability, []models.TerritoryAvailability, error)

	GetSubscriptionGroups(appID string) ([]models.SubscriptionGroup, error)
	GetSubscriptionGroupLocalizations(groupID string) ([]models.SubscriptionGroupLocalization, error)
	GetSubscriptions(groupID string) ([]models.Subscription, error)
	GetSubscriptionLocalizations(subscriptionID string) ([]models.SubscriptionLocalization, error)
	GetSubscriptionPrices(subscriptionID string) ([]models.SubscriptionPrice, error)
	GetSubscriptionAvailability(subscriptionID string) (*models.SubscriptionAvailability, error)
	GetSubscriptionAvailableTerritories(availabilityID string) ([]models.Territory, error)
	GetSubscriptionIntroductoryOffers(subscriptionID string, territories []string) ([]models.SubscriptionIntroductoryOffer, error)
	GetInAppPurchases(appID string) ([]models.InAppPurchase, error)
	GetInAppPurchaseVersions(purchaseID string) ([]models.InAppPurchaseVersion, error)
	GetInAppPurchaseLocalizations(versionID string) ([]models.InAppPurchaseLocalization, error)
	GetInAppPurchasePrices(purchaseID string) (*models.InAppPurchasePriceSchedule, []models.InAppPurchasePrice, error)
	GetInAppPurchaseAvailability(purchaseID string) (*models.InAppPurchaseAvailability, error)
	GetInAppPurchaseAvailableTerritories(availabilityID string) ([]models.Territory, error)

	GetBetaGroups(appID string) ([]models.BetaGroup, error)
	GetBetaGroupTesters(groupID string) ([]models.BetaTester, error)
	GetBetaAppLocalizations(appID string) ([]models.BetaAppLocalization, error)
	GetBetaAppReviewDetail(appID string) (*models.BetaAppReviewDetail, error)

	GetUsers() ([]models.User, error)
	GetUserVisibleApps(userID string) ([]models.App, error)
	GetUserInvitations() ([]models.UserInvitation, error)
	GetUserInvitationVisibleApps(invitationID string) ([]models.App, error)
}

var _ Source = (*apple.Client)(nil)
