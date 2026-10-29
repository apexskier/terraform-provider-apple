// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// TestGetSubscriptionPricePointsSendsFilters covers the query parameters the
// price point catalogue depends on. Without include=territory a returned price
// point cannot be identified, and without filter[territory] the request pulls
// every territory Apple sells in.
func TestGetSubscriptionPricePointsSendsFilters(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"type":"subscriptionPricePoints","id":"p1",
			"attributes":{"customerPrice":"9.99","proceeds":"6.99"},
			"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}]}`)
	}))
	defer srv.Close()

	points, err := newTestClient(srv).GetSubscriptionPricePoints("sub1", []string{"USA", "GBR"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotQuery.Get("include"); got != "territory" {
		t.Errorf("include = %q, want %q", got, "territory")
	}
	if got := gotQuery.Get("filter[territory]"); got != "USA,GBR" {
		t.Errorf("filter[territory] = %q, want %q", got, "USA,GBR")
	}
	if got := gotQuery.Get("limit"); got != "200" {
		t.Errorf("limit = %q, want %q", got, "200")
	}

	if len(points) != 1 {
		t.Fatalf("got %d price points, want 1", len(points))
	}
	if points[0].Relationships == nil || points[0].Relationships.Territory == nil {
		t.Fatal("territory relationship was not decoded")
	}
	if got := points[0].Relationships.Territory.Data.ID; got != "USA" {
		t.Errorf("territory = %q, want %q", got, "USA")
	}
}

// TestGetSubscriptionPricePointsOmitsEmptyTerritoryFilter checks that an
// unfiltered call sends no filter at all rather than an empty one, which Apple
// would reject.
func TestGetSubscriptionPricePointsOmitsEmptyTerritoryFilter(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).GetSubscriptionPricePoints("sub1", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, present := gotQuery["filter[territory]"]; present {
		t.Error("filter[territory] was sent for an unfiltered request")
	}
}

// TestGetSubscriptionPricePointEqualizationsRequestsEqualizationsPath covers the
// endpoint and the include. The equalization read is what prices a subscription
// in every territory from one base point, and a returned point whose territory
// was not populated cannot be matched to the storefront it prices.
func TestGetSubscriptionPricePointEqualizationsRequestsEqualizationsPath(t *testing.T) {
	var gotPath string
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		fmt.Fprint(w, `{"data":[
			{"type":"subscriptionPricePoints","id":"eq-gbr",
				"attributes":{"customerPrice":"8.99","proceeds":"6.29"},
				"relationships":{"territory":{"data":{"type":"territories","id":"GBR"}}}},
			{"type":"subscriptionPricePoints","id":"eq-egy",
				"attributes":{"customerPrice":"199.99","proceeds":"139.99"},
				"relationships":{"territory":{"data":{"type":"territories","id":"EGY"}}}}]}`)
	}))
	defer srv.Close()

	points, err := newTestClient(srv).GetSubscriptionPricePointEqualizations("base-usa", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := "/v1/subscriptionPricePoints/base-usa/equalizations"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if got := gotQuery.Get("include"); got != "territory" {
		t.Errorf("include = %q, want %q", got, "territory")
	}
	if _, present := gotQuery["filter[territory]"]; present {
		t.Error("filter[territory] was sent for an unfiltered request")
	}

	if len(points) != 2 {
		t.Fatalf("got %d equalized price points, want 2", len(points))
	}
	if points[0].Relationships == nil || points[0].Relationships.Territory == nil {
		t.Fatal("territory relationship was not decoded")
	}
	if got := points[0].Relationships.Territory.Data.ID; got != "GBR" {
		t.Errorf("territory = %q, want %q", got, "GBR")
	}
	if got := points[1].Attributes.CustomerPrice; got != "199.99" {
		t.Errorf("customer price = %q, want %q", got, "199.99")
	}
}

// TestGetSubscriptionPricePointEqualizationsSendsTerritoryFilter checks that a
// narrowed read passes the filter to Apple rather than fetching every territory
// and discarding most of them.
func TestGetSubscriptionPricePointEqualizationsSendsTerritoryFilter(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetSubscriptionPricePointEqualizations("base-usa", []string{"GBR", "EGY"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotQuery.Get("filter[territory]"); got != "GBR,EGY" {
		t.Errorf("filter[territory] = %q, want %q", got, "GBR,EGY")
	}
}

// TestPaginationCarriesQueryParametersAcrossPages covers the interaction
// between getAllPagesQuery and the "next" link: Apple's link already carries
// the parameters forward, so they must not be re-appended and duplicated.
func TestPaginationCarriesQueryParametersAcrossPages(t *testing.T) {
	var requested []string

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.String())

		if r.URL.Query().Get("cursor") == "" {
			fmt.Fprintf(w, `{"data":[{"type":"subscriptionPrices","id":"a"}],
				"links":{"next":"%s/v1/subscriptions/sub1/prices?include=territory%%2CsubscriptionPricePoint&limit=200&cursor=next"}}`, srv.URL)
			return
		}
		fmt.Fprint(w, `{"data":[{"type":"subscriptionPrices","id":"b"}]}`)
	}))
	defer srv.Close()

	prices, err := newTestClient(srv).GetSubscriptionPrices("sub1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prices) != 2 {
		t.Fatalf("got %d prices, want 2 -- pagination did not follow the next link", len(prices))
	}
	if len(requested) != 2 {
		t.Fatalf("made %d requests, want 2", len(requested))
	}

	// The first request must carry both include values, and the second must be
	// exactly the link Apple supplied.
	first := requested[0]
	for _, want := range []string{"include=territory%2CsubscriptionPricePoint", "limit=200"} {
		if !strings.Contains(first, want) {
			t.Errorf("first request %q does not contain %q", first, want)
		}
	}
	if !strings.Contains(requested[1], "cursor=next") {
		t.Errorf("second request %q did not follow the next link", requested[1])
	}
}

// TestGetSubscriptionRequestsGroupInclude covers the include that makes import
// possible: without it Apple reports the group relationship as links alone.
func TestGetSubscriptionRequestsGroupInclude(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, `{"data":{"type":"subscriptions","id":"sub1",
			"attributes":{"name":"Pro","productId":"com.example.pro","state":"MISSING_METADATA"},
			"relationships":{"group":{"data":{"type":"subscriptionGroups","id":"grp1"}}}}}`)
	}))
	defer srv.Close()

	subscription, err := newTestClient(srv).GetSubscription("sub1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotQuery.Get("include"); got != "group" {
		t.Errorf("include = %q, want %q", got, "group")
	}
	if subscription.Relationships == nil || subscription.Relationships.Group == nil {
		t.Fatal("group relationship was not decoded")
	}
	if got := subscription.Relationships.Group.Data.ID; got != "grp1" {
		t.Errorf("group = %q, want %q", got, "grp1")
	}
}

// TestCreateSubscriptionOmitsUnsetAttributes checks that optional attributes
// left unset are absent from the request body rather than sent as zero values.
// Apple treats an explicit null differently from an omitted member.
func TestCreateSubscriptionOmitsUnsetAttributes(t *testing.T) {
	var body map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("could not decode request body: %v", err)
		}
		fmt.Fprint(w, `{"data":{"type":"subscriptions","id":"sub1",
			"attributes":{"name":"Pro","productId":"com.example.pro"}}}`)
	}))
	defer srv.Close()

	attributes := models.SubscriptionCreateAttributes{
		Name:      "Pro",
		ProductID: "com.example.pro",
	}

	_, err := newTestClient(srv).CreateSubscription("grp1", attributes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := body["data"].(map[string]interface{})
	sentAttributes, _ := data["attributes"].(map[string]interface{})

	for _, absent := range []string{"familySharable", "subscriptionPeriod", "reviewNote", "groupLevel"} {
		if _, present := sentAttributes[absent]; present {
			t.Errorf("attribute %q was sent despite being unset", absent)
		}
	}
	if got := sentAttributes["productId"]; got != "com.example.pro" {
		t.Errorf("productId = %v, want com.example.pro", got)
	}

	relationships, _ := data["relationships"].(map[string]interface{})
	group, _ := relationships["group"].(map[string]interface{})
	groupData, _ := group["data"].(map[string]interface{})
	if got := groupData["id"]; got != "grp1" {
		t.Errorf("group id = %v, want grp1", got)
	}
	if got := groupData["type"]; got != "subscriptionGroups" {
		t.Errorf("group type = %v, want subscriptionGroups", got)
	}
}

// TestCreateSubscriptionPriceOmitsEmptyAttributes pins the wire format.
//
// Every attribute is optional, so a price with no start date, no
// preserveCurrentPrice and no plan type marshalled to "attributes":{} — and
// App Store Connect answered 409 "An error occurred while processing the
// pricing information", which reads like a complaint about the price point.
// The member has to be absent, not empty.
func TestCreateSubscriptionPriceOmitsEmptyAttributes(t *testing.T) {
	var body map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %s", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"subscriptionPrices","id":"price-1","attributes":{}}}`)
	}))
	defer srv.Close()

	client := &Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}

	if _, err := client.CreateSubscriptionPrice(
		"sub-1", "point-1", nil, &models.SubscriptionPriceCreateAttributes{}, nil,
	); err != nil {
		t.Fatalf("CreateSubscriptionPrice: %s", err)
	}

	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("request has no data member: %v", body)
	}
	if _, present := data["attributes"]; present {
		t.Errorf("attributes member sent for a price with nothing set: %v", data["attributes"])
	}

	// A price that does set something must still carry it.
	startDate := "2027-01-01"
	if _, err := client.CreateSubscriptionPrice(
		"sub-1", "point-1", nil,
		&models.SubscriptionPriceCreateAttributes{StartDate: &startDate}, nil,
	); err != nil {
		t.Fatalf("CreateSubscriptionPrice with a start date: %s", err)
	}

	data, _ = body["data"].(map[string]any)
	attrs, ok := data["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes dropped for a price that sets a start date: %v", data)
	}
	if attrs["startDate"] != startDate {
		t.Errorf("startDate = %v, want %q", attrs["startDate"], startDate)
	}
}

// TestCreateSubscriptionAvailabilityRequest pins the request shape.
//
// A subscription availability is a prerequisite for pricing, and Apple reports
// a malformed one the same opaque way it reports a missing one -- so the
// relationship names and territory type are worth asserting rather than
// discovering through a 409 that says nothing.
func TestCreateSubscriptionAvailabilityRequest(t *testing.T) {
	var (
		body   map[string]any
		method string
		path   string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %s", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1",
			"attributes":{"availableInNewTerritories":true}}}`)
	}))
	defer srv.Close()

	client := &Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}

	availability, err := client.CreateSubscriptionAvailability("sub-1", true, []string{"USA", "GBR"}, nil)
	if err != nil {
		t.Fatalf("CreateSubscriptionAvailability: %s", err)
	}
	if availability.ID != "avail-1" {
		t.Errorf("ID = %q, want %q", availability.ID, "avail-1")
	}

	if method != "POST" || path != "/v1/subscriptionAvailabilities" {
		t.Errorf("request = %s %s, want POST /v1/subscriptionAvailabilities", method, path)
	}

	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("request has no data member: %v", body)
	}
	if data["type"] != "subscriptionAvailabilities" {
		t.Errorf("type = %v, want subscriptionAvailabilities", data["type"])
	}

	attrs, ok := data["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("request has no attributes member: %v", data)
	}
	if attrs["availableInNewTerritories"] != true {
		t.Errorf("availableInNewTerritories = %v, want true", attrs["availableInNewTerritories"])
	}

	rels, ok := data["relationships"].(map[string]any)
	if !ok {
		t.Fatalf("request has no relationships member: %v", data)
	}

	sub, ok := rels["subscription"].(map[string]any)
	if !ok {
		t.Fatalf("no subscription relationship: %v", rels)
	}
	subData, _ := sub["data"].(map[string]any)
	if subData["type"] != "subscriptions" || subData["id"] != "sub-1" {
		t.Errorf("subscription relationship = %v, want type subscriptions id sub-1", subData)
	}

	territories, ok := rels["availableTerritories"].(map[string]any)
	if !ok {
		t.Fatalf("no availableTerritories relationship: %v", rels)
	}
	list, _ := territories["data"].([]any)
	if len(list) != 2 {
		t.Fatalf("availableTerritories has %d entries, want 2: %v", len(list), list)
	}
	for i, want := range []string{"USA", "GBR"} {
		entry, _ := list[i].(map[string]any)
		if entry["type"] != "territories" || entry["id"] != want {
			t.Errorf("territory %d = %v, want type territories id %q", i, entry, want)
		}
	}
}

// TestGetSubscriptionGroupLocalizationRequestsGroupInclude pins the one part of
// the group hierarchy whose parent can be read back. A subscription group never
// reports its owning app, so import has to be composite there; a group
// localization does report its group, which is what lets a bare ID import.
func TestGetSubscriptionGroupLocalizationRequestsGroupInclude(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, `{"data":{"type":"subscriptionGroupLocalizations","id":"loc1",
			"attributes":{"name":"Premium","locale":"en-US","state":"PREPARE_FOR_SUBMISSION"},
			"relationships":{"subscriptionGroup":{"data":{"type":"subscriptionGroups","id":"grp1"}}}}}`)
	}))
	defer srv.Close()

	localization, err := newTestClient(srv).GetSubscriptionGroupLocalization("loc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotQuery.Get("include"); got != "subscriptionGroup" {
		t.Errorf("include = %q, want %q", got, "subscriptionGroup")
	}
	if localization.Relationships == nil || localization.Relationships.SubscriptionGroup == nil {
		t.Fatal("subscriptionGroup relationship was not decoded")
	}
	if got := localization.Relationships.SubscriptionGroup.Data.ID; got != "grp1" {
		t.Errorf("subscriptionGroup = %q, want %q", got, "grp1")
	}
}

// TestCreateSubscriptionGroupLocalizationOmitsUnsetCustomAppName checks that an
// unset custom app name is absent from the request rather than sent as an empty
// string, which Apple would take as an instruction to show no app name at all.
func TestCreateSubscriptionGroupLocalizationOmitsUnsetCustomAppName(t *testing.T) {
	var body map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("could not decode request body: %v", err)
		}
		fmt.Fprint(w, `{"data":{"type":"subscriptionGroupLocalizations","id":"loc1",
			"attributes":{"name":"Premium","locale":"en-US"}}}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).CreateSubscriptionGroupLocalization("grp1", "en-US", "Premium", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := body["data"].(map[string]any)
	if got := data["type"]; got != "subscriptionGroupLocalizations" {
		t.Errorf("type = %v, want subscriptionGroupLocalizations", got)
	}

	attributes, _ := data["attributes"].(map[string]any)
	if _, present := attributes["customAppName"]; present {
		t.Error("customAppName was sent despite being unset")
	}
	if got := attributes["name"]; got != "Premium" {
		t.Errorf("name = %v, want Premium", got)
	}
	if got := attributes["locale"]; got != "en-US" {
		t.Errorf("locale = %v, want en-US", got)
	}

	relationships, _ := data["relationships"].(map[string]any)
	group, _ := relationships["subscriptionGroup"].(map[string]any)
	groupData, _ := group["data"].(map[string]any)
	if groupData["type"] != "subscriptionGroups" || groupData["id"] != "grp1" {
		t.Errorf("subscriptionGroup relationship = %v, want type subscriptionGroups id grp1", groupData)
	}
}

// TestSetSubscriptionPricesRequestShape pins the bulk price write.
//
// This is the request that replaces one POST /v1/subscriptionPrices per
// territory with a single call, and almost every part of it is load-bearing in
// a way Apple's errors do not explain: the method and path are the
// subscription's rather than the price collection's, the placeholder IDs in
// "included" have to be the same strings the "prices" relationship references,
// and an empty attributes member is answered with a 409 about "processing the
// pricing information" that names neither the member nor the price.
func TestSetSubscriptionPricesRequestShape(t *testing.T) {
	var (
		body   map[string]any
		method string
		path   string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %s", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{}}}`)
	}))
	defer srv.Close()

	startDate := "2027-01-01"
	preserve := true
	planType := models.SubscriptionPlanType("MONTHLY")

	if _, err := newTestClient(srv).SetSubscriptionPrices("sub-1", []SubscriptionManualPrice{
		{PricePointID: "point-usa", TerritoryID: "USA"},
		{
			PricePointID:         "point-gbr",
			TerritoryID:          "GBR",
			StartDate:            &startDate,
			PreserveCurrentPrice: &preserve,
			PlanType:             &planType,
		},
	}, nil); err != nil {
		t.Fatalf("SetSubscriptionPrices: %s", err)
	}

	if method != "PATCH" {
		t.Errorf("method = %q, want PATCH", method)
	}
	if path != "/v1/subscriptions/sub-1" {
		t.Errorf("path = %q, want /v1/subscriptions/sub-1", path)
	}

	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("request has no data member: %v", body)
	}
	if data["type"] != "subscriptions" || data["id"] != "sub-1" {
		t.Errorf("data identifies %v/%v, want subscriptions/sub-1", data["type"], data["id"])
	}

	relationships, _ := data["relationships"].(map[string]any)
	pricesRel, _ := relationships["prices"].(map[string]any)
	linkage, ok := pricesRel["data"].([]any)
	if !ok {
		t.Fatalf("prices relationship is not a to-many linkage: %v", relationships["prices"])
	}
	if len(linkage) != 2 {
		t.Fatalf("got %d linkages, want 2", len(linkage))
	}

	included, ok := body["included"].([]any)
	if !ok || len(included) != 2 {
		t.Fatalf("got %v included members, want 2", body["included"])
	}

	// Every linkage has to resolve to an included member, or Apple commits a
	// relationship to a price that was never sent.
	byID := make(map[string]map[string]any, len(included))
	for _, member := range included {
		entry, _ := member.(map[string]any)
		if entry["type"] != "subscriptionPrices" {
			t.Errorf("included member type = %v, want subscriptionPrices", entry["type"])
		}
		byID[fmt.Sprint(entry["id"])] = entry
	}
	for i, member := range linkage {
		entry, _ := member.(map[string]any)
		if entry["type"] != "subscriptionPrices" {
			t.Errorf("linkage %d type = %v, want subscriptionPrices", i, entry["type"])
		}
		want := fmt.Sprintf("${price%d}", i)
		if entry["id"] != want {
			t.Errorf("linkage %d id = %v, want %q", i, entry["id"], want)
		}
		if _, ok := byID[want]; !ok {
			t.Errorf("linkage %d references %q, which is not in included", i, want)
		}
	}

	// A price with nothing set sends no attributes member at all.
	first := byID["${price0}"]
	if _, present := first["attributes"]; present {
		t.Errorf("attributes member sent for a price with nothing set: %v", first["attributes"])
	}
	firstRels, _ := first["relationships"].(map[string]any)
	for name, wantType := range map[string]string{
		"subscription":           "subscriptions",
		"subscriptionPricePoint": "subscriptionPricePoints",
		"territory":              "territories",
	} {
		rel, ok := firstRels[name].(map[string]any)
		if !ok {
			t.Errorf("inline price is missing the %s relationship", name)
			continue
		}
		relData, _ := rel["data"].(map[string]any)
		if relData["type"] != wantType {
			t.Errorf("%s type = %v, want %q", name, relData["type"], wantType)
		}
	}

	// A price that sets something carries all of it.
	second := byID["${price1}"]
	attrs, ok := second["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes dropped for a price that sets them: %v", second)
	}
	if attrs["startDate"] != startDate {
		t.Errorf("startDate = %v, want %q", attrs["startDate"], startDate)
	}
	if attrs["preserveCurrentPrice"] != true {
		t.Errorf("preserveCurrentPrice = %v, want true", attrs["preserveCurrentPrice"])
	}
	if attrs["planType"] != "MONTHLY" {
		t.Errorf("planType = %v, want MONTHLY", attrs["planType"])
	}
}

// TestSetSubscriptionPricesOmitsUnsetTerritory checks that a price relying on
// the territory its price point encodes sends no territory relationship, rather
// than an empty one. ResourceIdentifier marshals an unset value as
// {"type":"","id":""}, which Apple rejects.
func TestSetSubscriptionPricesOmitsUnsetTerritory(t *testing.T) {
	var body map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %s", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{}}}`)
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).SetSubscriptionPrices("sub-1", []SubscriptionManualPrice{
		{PricePointID: "point-usa"},
	}, nil); err != nil {
		t.Fatalf("SetSubscriptionPrices: %s", err)
	}

	included, _ := body["included"].([]any)
	if len(included) != 1 {
		t.Fatalf("got %d included members, want 1", len(included))
	}
	entry, _ := included[0].(map[string]any)
	relationships, _ := entry["relationships"].(map[string]any)
	if _, present := relationships["territory"]; present {
		t.Errorf("territory relationship sent for a price that named none: %v", relationships["territory"])
	}
}

// TestCreateSubscriptionIntroductoryOfferRequestShape checks that a free trial
// sends no price point relationship and a paid offer sends one. ResourceIdentifier
// marshals an unset value as {"type":"","id":""}, which Apple rejects.
func TestCreateSubscriptionIntroductoryOfferRequestShape(t *testing.T) {
	tests := []struct {
		name           string
		offer          SubscriptionIntroductoryOfferInput
		wantPricePoint bool
	}{
		{
			name: "free trial",
			offer: SubscriptionIntroductoryOfferInput{
				TerritoryID:     "USA",
				OfferMode:       models.SubscriptionOfferModeFreeTrial,
				Duration:        models.SubscriptionOfferDurationOneWeek,
				NumberOfPeriods: 1,
			},
		},
		{
			name: "pay as you go",
			offer: SubscriptionIntroductoryOfferInput{
				TerritoryID:     "GBR",
				OfferMode:       models.SubscriptionOfferModePayAsYouGo,
				Duration:        models.SubscriptionOfferDurationOneMonth,
				NumberOfPeriods: 3,
				PricePointID:    "point-gbr",
			},
			wantPricePoint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				body   map[string]any
				method string
				path   string
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method, path = r.Method, r.URL.Path
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decoding request body: %s", err)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1","attributes":{}}}`)
			}))
			defer srv.Close()

			offer, err := newTestClient(srv).CreateSubscriptionIntroductoryOffer("sub-1", tt.offer, nil)
			if err != nil {
				t.Fatalf("CreateSubscriptionIntroductoryOffer: %s", err)
			}
			if offer.ID != "offer-1" {
				t.Errorf("id = %q, want offer-1", offer.ID)
			}
			if method != "POST" || path != "/v1/subscriptionIntroductoryOffers" {
				t.Errorf("request = %s %s, want POST /v1/subscriptionIntroductoryOffers", method, path)
			}

			data, _ := body["data"].(map[string]any)
			attrs, _ := data["attributes"].(map[string]any)
			if attrs["offerMode"] != string(tt.offer.OfferMode) {
				t.Errorf("offerMode = %v, want %q", attrs["offerMode"], tt.offer.OfferMode)
			}
			if attrs["duration"] != string(tt.offer.Duration) {
				t.Errorf("duration = %v, want %q", attrs["duration"], tt.offer.Duration)
			}
			if attrs["numberOfPeriods"] != float64(tt.offer.NumberOfPeriods) {
				t.Errorf("numberOfPeriods = %v, want %d", attrs["numberOfPeriods"], tt.offer.NumberOfPeriods)
			}
			for _, unset := range []string{"startDate", "endDate", "targetSubscriptionPlanType"} {
				if _, present := attrs[unset]; present {
					t.Errorf("%s sent although unset: %v", unset, attrs[unset])
				}
			}

			relationships, _ := data["relationships"].(map[string]any)
			territory, _ := relationships["territory"].(map[string]any)
			territoryData, _ := territory["data"].(map[string]any)
			if territoryData["id"] != tt.offer.TerritoryID {
				t.Errorf("territory = %v, want %q", territoryData["id"], tt.offer.TerritoryID)
			}

			_, present := relationships["subscriptionPricePoint"]
			if present != tt.wantPricePoint {
				t.Errorf("subscriptionPricePoint present = %v, want %v", present, tt.wantPricePoint)
			}
		})
	}
}

// TestGetSubscriptionIntroductoryOfferSendsFilters checks the listing that
// stands in for the single-offer GET Apple does not publish.
func TestGetSubscriptionIntroductoryOfferSendsFilters(t *testing.T) {
	var (
		gotQuery url.Values
		gotPath  string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery, gotPath = r.URL.Query(), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-1",
			"attributes":{"offerMode":"FREE_TRIAL","duration":"ONE_WEEK","numberOfPeriods":1},
			"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}]}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)

	offer, err := client.GetSubscriptionIntroductoryOffer("sub-1", "USA", "offer-1")
	if err != nil {
		t.Fatalf("GetSubscriptionIntroductoryOffer: %s", err)
	}
	if gotPath != "/v1/subscriptions/sub-1/introductoryOffers" {
		t.Errorf("path = %q", gotPath)
	}
	if got := gotQuery.Get("include"); got != "territory,subscriptionPricePoint" {
		t.Errorf("include = %q", got)
	}
	if got := gotQuery.Get("filter[territory]"); got != "USA" {
		t.Errorf("filter[territory] = %q, want USA", got)
	}
	if offer.Attributes.OfferMode == nil || *offer.Attributes.OfferMode != models.SubscriptionOfferModeFreeTrial {
		t.Errorf("offerMode = %v, want FREE_TRIAL", offer.Attributes.OfferMode)
	}

	if _, err := client.GetSubscriptionIntroductoryOffer("sub-1", "", "offer-2"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing offer error = %v, want one containing \"not found\"", err)
	}
	if gotQuery.Has("filter[territory]") {
		t.Errorf("filter[territory] sent with no territory: %q", gotQuery.Get("filter[territory]"))
	}
}

// TestUpdateSubscriptionIntroductoryOfferEndDateSendsNull checks that clearing
// an end date sends an explicit null rather than omitting the member, which
// would leave the old date in place.
func TestUpdateSubscriptionIntroductoryOfferEndDateSendsNull(t *testing.T) {
	var (
		raw    string
		method string
		path   string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		raw = string(b)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1","attributes":{}}}`)
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).UpdateSubscriptionIntroductoryOfferEndDate("offer-1", nil, nil); err != nil {
		t.Fatalf("UpdateSubscriptionIntroductoryOfferEndDate: %s", err)
	}
	if method != "PATCH" || path != "/v1/subscriptionIntroductoryOffers/offer-1" {
		t.Errorf("request = %s %s", method, path)
	}
	if !strings.Contains(raw, `"attributes":{"endDate":null}`) {
		t.Errorf("body = %s, want an explicit null endDate", raw)
	}
}

// TestAddSubscriptionIntroductoryOffersRequestShape pins the bulk offer write:
// kept offers are named by their real IDs, new ones by placeholders that each
// resolve to an included member.
func TestAddSubscriptionIntroductoryOffersRequestShape(t *testing.T) {
	var (
		body   map[string]any
		method string
		path   string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %s", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{}}}`)
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).AddSubscriptionIntroductoryOffers("sub-1", []string{"existing-1"}, []SubscriptionIntroductoryOfferInput{
		{TerritoryID: "USA", OfferMode: models.SubscriptionOfferModeFreeTrial, Duration: models.SubscriptionOfferDurationOneWeek, NumberOfPeriods: 1},
		{TerritoryID: "GBR", OfferMode: models.SubscriptionOfferModePayUpFront, Duration: models.SubscriptionOfferDurationThreeMonths, NumberOfPeriods: 1, PricePointID: "point-gbr"},
	}, nil); err != nil {
		t.Fatalf("AddSubscriptionIntroductoryOffers: %s", err)
	}

	if method != "PATCH" || path != "/v1/subscriptions/sub-1" {
		t.Errorf("request = %s %s, want PATCH /v1/subscriptions/sub-1", method, path)
	}

	data, _ := body["data"].(map[string]any)
	if _, present := data["attributes"]; present {
		t.Errorf("attributes member sent on the subscription: %v", data["attributes"])
	}
	relationships, _ := data["relationships"].(map[string]any)
	offersRel, _ := relationships["introductoryOffers"].(map[string]any)
	linkage, _ := offersRel["data"].([]any)

	var ids []string
	for _, member := range linkage {
		entry, _ := member.(map[string]any)
		if entry["type"] != "subscriptionIntroductoryOffers" {
			t.Errorf("linkage type = %v", entry["type"])
		}
		ids = append(ids, fmt.Sprint(entry["id"]))
	}
	if want := "existing-1,${offer0},${offer1}"; strings.Join(ids, ",") != want {
		t.Errorf("linkage = %v, want %s", ids, want)
	}

	included, _ := body["included"].([]any)
	if len(included) != 2 {
		t.Fatalf("got %d included members, want 2", len(included))
	}
	for i, member := range included {
		entry, _ := member.(map[string]any)
		if want := fmt.Sprintf("${offer%d}", i); entry["id"] != want {
			t.Errorf("included %d id = %v, want %s", i, entry["id"], want)
		}
		rels, _ := entry["relationships"].(map[string]any)
		if _, ok := rels["subscription"]; !ok {
			t.Errorf("included %d is missing the subscription relationship", i)
		}
		_, hasPoint := rels["subscriptionPricePoint"]
		if hasPoint != (i == 1) {
			t.Errorf("included %d subscriptionPricePoint present = %v", i, hasPoint)
		}
	}
}
