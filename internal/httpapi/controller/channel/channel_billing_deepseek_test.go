package channelcontroller

import "testing"

func TestPickDeepSeekBalanceInfoPrefersCNY(t *testing.T) {
	infos := []deepSeekBalanceInfo{
		{Currency: "USD", TotalBalance: "20.00"},
		{Currency: "CNY", TotalBalance: "110.00", GrantedBalance: "10.00", ToppedUpBalance: "100.00"},
	}

	balanceInfo, err := pickDeepSeekBalanceInfo(infos)
	if err != nil {
		t.Fatalf("pickDeepSeekBalanceInfo returned error: %v", err)
	}
	if balanceInfo.Currency != "CNY" {
		t.Fatalf("currency = %q, want CNY", balanceInfo.Currency)
	}
	if balanceInfo.TotalBalance != "110.00" {
		t.Fatalf("total_balance = %q, want 110.00", balanceInfo.TotalBalance)
	}
}

func TestPickDeepSeekBalanceInfoFallsBackToUSD(t *testing.T) {
	infos := []deepSeekBalanceInfo{
		{Currency: "USD", TotalBalance: "15.00", GrantedBalance: "5.00", ToppedUpBalance: "10.00"},
	}

	balanceInfo, err := pickDeepSeekBalanceInfo(infos)
	if err != nil {
		t.Fatalf("pickDeepSeekBalanceInfo returned error: %v", err)
	}
	if balanceInfo.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", balanceInfo.Currency)
	}
}

func TestPickDeepSeekBalanceInfoNormalizesCurrency(t *testing.T) {
	infos := []deepSeekBalanceInfo{
		{Currency: " cny ", TotalBalance: "110.00"},
	}

	balanceInfo, err := pickDeepSeekBalanceInfo(infos)
	if err != nil {
		t.Fatalf("pickDeepSeekBalanceInfo returned error: %v", err)
	}
	if balanceInfo.Currency != " cny " {
		t.Fatalf("original currency value should stay untouched, got %q", balanceInfo.Currency)
	}
}

func TestPickDeepSeekBalanceInfoRejectsUnknownCurrencies(t *testing.T) {
	infos := []deepSeekBalanceInfo{
		{Currency: "EUR", TotalBalance: "9.99"},
	}

	if _, err := pickDeepSeekBalanceInfo(infos); err == nil {
		t.Fatal("pickDeepSeekBalanceInfo should fail when no CNY/USD entry exists")
	}
}

func TestParseDeepSeekAmount(t *testing.T) {
	testCases := []struct {
		name    string
		raw     string
		want    float64
		wantErr bool
	}{
		{name: "plain amount", raw: "100.00", want: 100},
		{name: "missing field", raw: "", want: 0},
		{name: "blank field", raw: "  ", want: 0},
		{name: "invalid amount", raw: "abc", wantErr: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := parseDeepSeekAmount(testCase.raw)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("parseDeepSeekAmount(%q) error = %v, wantErr %v", testCase.raw, err, testCase.wantErr)
			}
			if !testCase.wantErr && got != testCase.want {
				t.Fatalf("parseDeepSeekAmount(%q) = %v, want %v", testCase.raw, got, testCase.want)
			}
		})
	}
}
