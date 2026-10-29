// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// GetSubscriptionIntroductoryOffers retrieves the introductory offers of a
// subscription, optionally narrowed to some territories.
//
// include is required for the result to be usable, as it is for prices: without
// it Apple reports the territory and price point as links alone, and an offer
// whose territory is unknown cannot be matched against configuration.
func (c *Client) GetSubscriptionIntroductoryOffers(subscriptionID string, territories []string) ([]models.SubscriptionIntroductoryOffer, error) {
	params := url.Values{}
	params.Set("include", "territory,subscriptionPricePoint")
	if len(territories) > 0 {
		params.Set("filter[territory]", strings.Join(territories, ","))
	}

	return getAllPagesQuery[models.SubscriptionIntroductoryOffer](
		c,
		fmt.Sprintf("/v1/subscriptions/%s/introductoryOffers", subscriptionID),
		defaultPageSize,
		params,
	)
}

// GetSubscriptionIntroductoryOffer finds one introductory offer by its ID.
//
// Apple publishes no GET /v1/subscriptionIntroductoryOffers/{id}, so this lists
// the subscription's offers and scans them. territoryID narrows the listing
// server-side when it is known, which keeps a per-territory resource's refresh
// to one small page rather than the whole collection.
func (c *Client) GetSubscriptionIntroductoryOffer(subscriptionID, territoryID, offerID string) (*models.SubscriptionIntroductoryOffer, error) {
	var territories []string
	if territoryID != "" {
		territories = []string{territoryID}
	}

	offers, err := c.GetSubscriptionIntroductoryOffers(subscriptionID, territories)
	if err != nil {
		return nil, err
	}

	for _, offer := range offers {
		if offer.ID == offerID {
			return &offer, nil
		}
	}

	return nil, fmt.Errorf("introductory offer '%s' not found on subscription '%s'", offerID, subscriptionID)
}

// SubscriptionIntroductoryOfferInput is one introductory offer to create.
//
// It is an input helper rather than a wire type, the way SubscriptionManualPrice
// is: the single create and the bulk inline create carry the same members in
// different envelopes, and both are assembled from this.
type SubscriptionIntroductoryOfferInput struct {
	TerritoryID     string
	OfferMode       models.SubscriptionOfferMode
	Duration        models.SubscriptionOfferDuration
	NumberOfPeriods int
	PricePointID    string
	StartDate       *string
	EndDate         *string
	TargetPlanType  *models.SubscriptionPlanType
}

func (o SubscriptionIntroductoryOfferInput) attributes() models.SubscriptionIntroductoryOfferCreateAttributes {
	return models.SubscriptionIntroductoryOfferCreateAttributes{
		Duration:                   o.Duration,
		OfferMode:                  o.OfferMode,
		NumberOfPeriods:            o.NumberOfPeriods,
		StartDate:                  o.StartDate,
		EndDate:                    o.EndDate,
		TargetSubscriptionPlanType: o.TargetPlanType,
	}
}

func (o SubscriptionIntroductoryOfferInput) relationships(subscriptionID string) models.SubscriptionIntroductoryOfferCreateRelationships {
	relationships := models.SubscriptionIntroductoryOfferCreateRelationships{
		Subscription: models.ResourceIdentifier{
			Data: models.ResourceData{Type: "subscriptions", ID: subscriptionID},
		},
	}
	if o.TerritoryID != "" {
		relationships.Territory = &models.ResourceIdentifier{
			Data: models.ResourceData{Type: "territories", ID: o.TerritoryID},
		}
	}
	if o.PricePointID != "" {
		relationships.SubscriptionPricePoint = &models.ResourceIdentifier{
			Data: models.ResourceData{Type: "subscriptionPricePoints", ID: o.PricePointID},
		}
	}
	return relationships
}

// CreateSubscriptionIntroductoryOffer creates one introductory offer.
func (c *Client) CreateSubscriptionIntroductoryOffer(
	subscriptionID string,
	offer SubscriptionIntroductoryOfferInput,
	authToken *string,
) (*models.SubscriptionIntroductoryOffer, error) {
	requestData := models.Request[models.SubscriptionIntroductoryOfferCreateRequest]{
		Data: models.SubscriptionIntroductoryOfferCreateRequest{
			Type:          "subscriptionIntroductoryOffers",
			Attributes:    offer.attributes(),
			Relationships: offer.relationships(subscriptionID),
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/subscriptionIntroductoryOffers", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionIntroductoryOffer]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateSubscriptionIntroductoryOfferEndDate sets or clears an offer's end date,
// the only thing Apple allows to change on an existing offer. A nil endDate is
// sent as an explicit null.
func (c *Client) UpdateSubscriptionIntroductoryOfferEndDate(offerID string, endDate *string, authToken *string) (*models.SubscriptionIntroductoryOffer, error) {
	requestData := models.Request[models.SubscriptionIntroductoryOfferUpdateRequest]{
		Data: models.SubscriptionIntroductoryOfferUpdateRequest{
			Type:       "subscriptionIntroductoryOffers",
			ID:         offerID,
			Attributes: models.SubscriptionIntroductoryOfferUpdateAttributes{EndDate: endDate},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/subscriptionIntroductoryOffers/%s", c.HostURL, offerID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionIntroductoryOffer]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteSubscriptionIntroductoryOffer removes an introductory offer.
func (c *Client) DeleteSubscriptionIntroductoryOffer(offerID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/subscriptionIntroductoryOffers/%s", c.HostURL, offerID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// AddSubscriptionIntroductoryOffers creates several introductory offers in one
// request.
//
// POST /v1/subscriptionIntroductoryOffers takes one offer in one territory, so a
// free trial in every storefront through it is one request per territory. The
// bulk form hangs off the subscription, as SetSubscriptionPrices does:
// PATCH /v1/subscriptions/{id} with an "introductoryOffers" relationship whose
// new members travel inline in "included".
//
// The relationship names keepIDs -- offers that already exist and should stay --
// as well as the placeholders. Whether Apple treats the relationship as the
// complete set (as it does for prices) or as additions is unverified against the
// live API, and naming the kept offers gives the same result either way.
func (c *Client) AddSubscriptionIntroductoryOffers(
	subscriptionID string,
	keepIDs []string,
	offers []SubscriptionIntroductoryOfferInput,
	authToken *string,
) (*models.Subscription, error) {
	linkage := make([]models.ResourceData, 0, len(keepIDs)+len(offers))
	for _, id := range keepIDs {
		linkage = append(linkage, models.ResourceData{Type: "subscriptionIntroductoryOffers", ID: id})
	}

	included := make([]models.SubscriptionIntroductoryOfferInlineCreate, 0, len(offers))
	for i, offer := range offers {
		placeholderID := fmt.Sprintf("${offer%d}", i)

		linkage = append(linkage, models.ResourceData{Type: "subscriptionIntroductoryOffers", ID: placeholderID})
		included = append(included, models.SubscriptionIntroductoryOfferInlineCreate{
			Type:          "subscriptionIntroductoryOffers",
			ID:            placeholderID,
			Attributes:    offer.attributes(),
			Relationships: offer.relationships(subscriptionID),
		})
	}

	requestData := models.SubscriptionIntroductoryOffersUpdateRequest{
		Data: models.SubscriptionIntroductoryOffersUpdateData{
			Type: "subscriptions",
			ID:   subscriptionID,
			Relationships: models.SubscriptionIntroductoryOffersUpdateRelationships{
				IntroductoryOffers: models.ResourceIdentifiers{Data: linkage},
			},
		},
		Included: included,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/subscriptions/%s", c.HostURL, subscriptionID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Subscription]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}
