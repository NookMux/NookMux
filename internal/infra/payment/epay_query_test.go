package payment

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/infra/httpclient"
)

// setupEpayQueryGateway 启动模拟易支付网关并注入查单配置，返回清理函数。
func setupEpayQueryGateway(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldPayAddress, oldEpayId, oldEpayKey := operation.PayAddress, operation.EpayId, operation.EpayKey
	operation.PayAddress = server.URL
	operation.EpayId = "1001"
	operation.EpayKey = "test-merchant-key"
	t.Cleanup(func() {
		operation.PayAddress, operation.EpayId, operation.EpayKey = oldPayAddress, oldEpayId, oldEpayKey
	})

	httpclient.InitHttpClient()
	return server
}

func TestQueryEpayOrderPaid(t *testing.T) {
	setupEpayQueryGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api.php" {
			t.Errorf("request path = %q, want /api.php", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("act") != "order" {
			t.Errorf("act = %q, want order", query.Get("act"))
		}
		if query.Get("pid") != "1001" {
			t.Errorf("pid = %q, want 1001", query.Get("pid"))
		}
		if query.Get("key") != "test-merchant-key" {
			t.Errorf("key = %q, want test-merchant-key", query.Get("key"))
		}
		if query.Get("out_trade_no") != "USR1NO123" {
			t.Errorf("out_trade_no = %q, want USR1NO123", query.Get("out_trade_no"))
		}
		_, _ = w.Write([]byte(`{"code":1,"msg":"查询订单号成功！","trade_no":"2016080622555342651","out_trade_no":"USR1NO123","type":"alipay","money":"1.00","status":1,"endtime":"2016-08-06 22:55:52"}`))
	})

	result, err := QueryEpayOrder("USR1NO123")
	if err != nil {
		t.Fatalf("QueryEpayOrder() error = %v", err)
	}
	if result.Code != 1 {
		t.Errorf("Code = %d, want 1", result.Code)
	}
	if result.Status != 1 {
		t.Errorf("Status = %d, want 1 (paid)", result.Status)
	}
	if result.Type != "alipay" {
		t.Errorf("Type = %q, want alipay", result.Type)
	}
	if result.Money != "1.00" {
		t.Errorf("Money = %q, want \"1.00\"", result.Money)
	}
	if result.OutTradeNo != "USR1NO123" {
		t.Errorf("OutTradeNo = %q, want USR1NO123", result.OutTradeNo)
	}
}

func TestQueryEpayOrderUnpaid(t *testing.T) {
	setupEpayQueryGateway(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":1,"msg":"查询订单号成功！","trade_no":"2016080622555342651","out_trade_no":"USR1NO123","type":"wxpay","money":"5.00","status":0}`))
	})

	result, err := QueryEpayOrder("USR1NO123")
	if err != nil {
		t.Fatalf("QueryEpayOrder() error = %v", err)
	}
	if result.Code != 1 || result.Status != 0 {
		t.Errorf("Code/Status = %d/%d, want 1/0 (unpaid)", result.Code, result.Status)
	}
}

func TestQueryEpayOrderGatewayBusinessError(t *testing.T) {
	setupEpayQueryGateway(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"msg":"订单不存在"}`))
	})

	result, err := QueryEpayOrder("USR1NO123")
	if err != nil {
		t.Fatalf("QueryEpayOrder() error = %v, business error must be returned as result for caller to judge", err)
	}
	if result.Code != 0 || result.Msg != "订单不存在" {
		t.Errorf("Code/Msg = %d/%q, want 0/订单不存在", result.Code, result.Msg)
	}
}

func TestQueryEpayOrderInvalidJSON(t *testing.T) {
	setupEpayQueryGateway(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not a json body`))
	})

	if _, err := QueryEpayOrder("USR1NO123"); err == nil {
		t.Fatal("QueryEpayOrder() expected error for invalid JSON, got nil")
	}
}

func TestQueryEpayOrderHTTPStatusError(t *testing.T) {
	setupEpayQueryGateway(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := QueryEpayOrder("USR1NO123"); err == nil {
		t.Fatal("QueryEpayOrder() expected error for HTTP 500, got nil")
	}
}

func TestQueryEpayOrderMissingConfig(t *testing.T) {
	oldPayAddress, oldEpayId, oldEpayKey := operation.PayAddress, operation.EpayId, operation.EpayKey
	operation.PayAddress, operation.EpayId, operation.EpayKey = "", "", ""
	t.Cleanup(func() {
		operation.PayAddress, operation.EpayId, operation.EpayKey = oldPayAddress, oldEpayId, oldEpayKey
	})

	if _, err := QueryEpayOrder("USR1NO123"); err == nil {
		t.Fatal("QueryEpayOrder() expected error for missing config, got nil")
	}
}

func TestQueryEpayOrderEmptyTradeNo(t *testing.T) {
	if _, err := QueryEpayOrder(""); err == nil {
		t.Fatal("QueryEpayOrder() expected error for empty trade_no, got nil")
	}
}
