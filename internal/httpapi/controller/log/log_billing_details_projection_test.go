package logcontroller

import (
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/config/console"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

func TestFilterHiddenUsageLogFieldsAppliesBillingDetailsVisibility(t *testing.T) {
	billingDetails := `{"schema_version":1,"tokens":{"input":{"text_input":12,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":7,"audio_output":0,"image_output":0,"reasoning_output":3,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":4,"write_cache":5,"write_cache_5m":5,"write_cache_1h":0}}}`
	oldFields := console.GetConsoleSetting().UsageLogFields
	oldUserDetailsEnabled := console.GetConsoleSetting().UsageLogFieldsUserEnabled
	t.Cleanup(func() {
		console.GetConsoleSetting().UsageLogFields = oldFields
		console.GetConsoleSetting().UsageLogFieldsUserEnabled = oldUserDetailsEnabled
	})

	console.GetConsoleSetting().UsageLogFields = `{"billing_details":{"admin":true,"user":true}}`

	visible := []*logstore.Log{{BillingDetails: &billingDetails}}
	filterHiddenUsageLogFields(visible)
	if visible[0].BillingDetails == nil || *visible[0].BillingDetails != billingDetails {
		t.Fatalf("visible billing_details = %v, want unchanged schema v1 payload", visible[0].BillingDetails)
	}

	console.GetConsoleSetting().UsageLogFields = `{"billing_details":{"admin":true,"user":false}}`
	hidden := []*logstore.Log{{BillingDetails: &billingDetails}}
	filterHiddenUsageLogFields(hidden)
	if hidden[0].BillingDetails != nil {
		t.Fatalf("hidden billing_details = %v, want nil", hidden[0].BillingDetails)
	}

	console.GetConsoleSetting().UsageLogFieldsUserEnabled = false
	detailsDisabled := []*logstore.Log{{BillingDetails: &billingDetails}}
	filterHiddenUsageLogFields(detailsDisabled)
	if detailsDisabled[0].BillingDetails != nil {
		t.Fatalf("billing_details with user details disabled = %v, want nil", detailsDisabled[0].BillingDetails)
	}
}

func TestFilterHiddenUsageLogFieldsAppliesPriceSnapshotVisibility(t *testing.T) {
	other := `{"billing_price_snapshot":{"source":"legacy","service_tier":"default"},"text_input":5}`
	oldFields := console.GetConsoleSetting().UsageLogFields
	oldUserDetailsEnabled := console.GetConsoleSetting().UsageLogFieldsUserEnabled
	t.Cleanup(func() {
		console.GetConsoleSetting().UsageLogFields = oldFields
		console.GetConsoleSetting().UsageLogFieldsUserEnabled = oldUserDetailsEnabled
	})

	console.GetConsoleSetting().UsageLogFields = `{"price_table":{"admin":true,"user":true}}`
	visible := []*logstore.Log{{Other: other}}
	filterHiddenUsageLogFields(visible)
	if !strings.Contains(visible[0].Other, "billing_price_snapshot") {
		t.Fatalf("visible price snapshot Other = %s, want snapshot retained", visible[0].Other)
	}

	console.GetConsoleSetting().UsageLogFields = `{"price_table":{"admin":true,"user":false}}`
	hidden := []*logstore.Log{{Other: other}}
	filterHiddenUsageLogFields(hidden)
	if strings.Contains(hidden[0].Other, "billing_price_snapshot") {
		t.Fatalf("hidden price snapshot Other = %s, want snapshot removed", hidden[0].Other)
	}
	if !strings.Contains(hidden[0].Other, `"text_input":5`) {
		t.Fatalf("hidden price snapshot Other = %s, want unrelated token fields retained", hidden[0].Other)
	}

	console.GetConsoleSetting().UsageLogFields = ""
	console.GetConsoleSetting().UsageLogFieldsUserEnabled = false
	detailsDisabled := []*logstore.Log{{Other: other}}
	filterHiddenUsageLogFields(detailsDisabled)
	if strings.Contains(detailsDisabled[0].Other, "billing_price_snapshot") {
		t.Fatalf("details disabled Other = %s, want price snapshot removed", detailsDisabled[0].Other)
	}
}

func TestFilterHiddenUsageLogFieldsReusesStoreOtherProjection(t *testing.T) {
	other := `{"billing_price_snapshot":{"source":"legacy"},"admin_info":{"secret":"value"},"keep":"visible"}`
	oldFields := console.GetConsoleSetting().UsageLogFields
	oldUserDetailsEnabled := console.GetConsoleSetting().UsageLogFieldsUserEnabled
	t.Cleanup(func() {
		console.GetConsoleSetting().UsageLogFields = oldFields
		console.GetConsoleSetting().UsageLogFieldsUserEnabled = oldUserDetailsEnabled
	})

	console.GetConsoleSetting().UsageLogFields = `{"price_table":{"admin":true,"user":false}}`
	logs := []*logstore.Log{{Other: other}}
	logs[0].OtherProjection = map[string]interface{}{
		"billing_price_snapshot": map[string]interface{}{"source": "legacy"},
		"keep":                   "visible",
	}
	logs[0].OtherProjectionParsed = true

	filterHiddenUsageLogFields(logs)
	if strings.Contains(logs[0].Other, "billing_price_snapshot") {
		t.Fatalf("projected Other = %s, want hidden snapshot removed without reparsing raw Other", logs[0].Other)
	}
	if strings.Contains(logs[0].Other, "admin_info") || strings.Contains(logs[0].Other, "secret") {
		t.Fatalf("projected Other = %s, want store-level admin field to stay removed", logs[0].Other)
	}
	if !strings.Contains(logs[0].Other, `"keep":"visible"`) {
		t.Fatalf("projected Other = %s, want unrelated field retained", logs[0].Other)
	}
	if logs[0].OtherProjection != nil || logs[0].OtherProjectionParsed {
		t.Fatalf("projection cache = %v/%v, want consumed after HTTP-boundary filtering", logs[0].OtherProjection, logs[0].OtherProjectionParsed)
	}
}

func TestFilterUsageLogFieldsForRoleAppliesAdminVisibility(t *testing.T) {
	billingDetails := `{"schema_version":1,"tokens":{"input":{"text_input":12,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":0,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`
	other := `{"billing_price_snapshot":{"source":"legacy"}}`
	oldFields := console.GetConsoleSetting().UsageLogFields
	oldAdminDetailsEnabled := console.GetConsoleSetting().UsageLogFieldsAdminEnabled
	t.Cleanup(func() {
		console.GetConsoleSetting().UsageLogFields = oldFields
		console.GetConsoleSetting().UsageLogFieldsAdminEnabled = oldAdminDetailsEnabled
	})

	console.GetConsoleSetting().UsageLogFields = `{
		"billing_details":{"admin":true,"user":true},
		"price_table":{"admin":false,"user":true}
	}`

	adminLogs := []*logstore.Log{
		{BillingDetails: &billingDetails, Other: other},
	}
	filterUsageLogFieldsForRole(adminLogs, true)
	if adminLogs[0].BillingDetails == nil || *adminLogs[0].BillingDetails != billingDetails {
		t.Fatalf("visible admin billing_details = %v, want retained", adminLogs[0].BillingDetails)
	}
	if strings.Contains(adminLogs[0].Other, "billing_price_snapshot") {
		t.Fatalf("hidden admin Other = %s, want snapshot removed", adminLogs[0].Other)
	}

	userLogs := []*logstore.Log{
		{BillingDetails: &billingDetails, Other: other},
	}
	filterUsageLogFieldsForRole(userLogs, false)
	if userLogs[0].BillingDetails == nil || userLogs[0].Other != other {
		t.Fatalf("user projection = %v / %s, want both fields retained", userLogs[0].BillingDetails, userLogs[0].Other)
	}

	console.GetConsoleSetting().UsageLogFieldsAdminEnabled = false
	adminDisabled := []*logstore.Log{
		{BillingDetails: &billingDetails, Other: other},
	}
	filterUsageLogFieldsForRole(adminDisabled, true)
	if adminDisabled[0].BillingDetails != nil || strings.Contains(adminDisabled[0].Other, "billing_price_snapshot") {
		t.Fatalf("admin-disabled projection = %v / %s, want details removed", adminDisabled[0].BillingDetails, adminDisabled[0].Other)
	}
}

func TestBillingDetailsWireProjection(t *testing.T) {
	billingDetails := `{"schema_version":1,"tokens":{"input":{"text_input":12,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":0,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`

	encoded, err := jsonx.Marshal(logstore.Log{BillingDetails: &billingDetails})
	if err != nil {
		t.Fatalf("marshal visible billing_details: %v", err)
	}
	if !strings.Contains(string(encoded), `"billing_details":`) {
		t.Fatalf("visible wire payload %s, want billing_details projection", encoded)
	}

	encoded, err = jsonx.Marshal(logstore.Log{})
	if err != nil {
		t.Fatalf("marshal hidden billing_details: %v", err)
	}
	if strings.Contains(string(encoded), `"billing_details":`) {
		t.Fatalf("hidden wire payload %s, want omitted billing_details", encoded)
	}
}
