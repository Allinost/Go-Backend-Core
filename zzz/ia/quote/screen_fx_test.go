package quote

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleSnapshot = `{"data":{"total":5000,"diff":[
 {"f12":"600519","f14":"贵州茅台","f2":1525.6,"f3":1.25,"f8":0.51,"f9":21.3,"f20":191600000000,"f21":191600000000,"f23":7.8,"f37":28.5,"f45":75000000000,"f46":15.2,"f62":1200000000,"f115":22.1},
 {"f12":"000001","f14":"平安银行","f2":11.5,"f3":-0.5,"f8":0.32,"f9":4.8,"f20":223000000000,"f21":210000000000,"f23":0.6,"f37":9.9,"f46":-3.5,"f62":-500000000,"f115":5.0},
 {"f12":"300001","f14":"ST特力A","f2":8.0,"f3":5.0,"f8":2.0,"f9":50,"f20":4000000000,"f21":3500000000,"f23":3.2,"f37":5.0,"f46":10.0,"f62":100000000,"f115":48.0}
]}}`

func TestParseSnapshot(t *testing.T) {
	rows, err := ParseSnapshot([]byte(sampleSnapshot))
	require.NoError(t, err)
	require.Len(t, rows, 2, "ST 股应被过滤")

	mt := rows[0]
	assert.Equal(t, "600519", mt.Code)
	assert.Equal(t, 1525.6, mt.Price)
	assert.InDelta(t, 1916.0, mt.TotalMVYi, 1e-6) // 元→亿元
	assert.Equal(t, 28.5, mt.ROE)
	assert.InDelta(t, 1200000000.0/191600000000.0, mt.MainInflowRatio, 1e-9)
}

func TestParseSnapshot_Empty(t *testing.T) {
	if _, err := ParseSnapshot([]byte(`{"data":null}`)); err == nil {
		t.Fatal("expected error for null data")
	}
}

func TestParseFxResponse(t *testing.T) {
	body := []byte(`{"result":"success","rates":{"USD":0.13966,"HKD":1.08932}}`)
	rates, err := parseFxResponse(body, SupportedQuoteCcys)
	require.NoError(t, err)
	assert.InDelta(t, 1.0/0.13966, rates["USD"], 1e-9, "存储口径为 1 USD 兑 CNY")
	assert.InDelta(t, 1.0/1.08932, rates["HKD"], 1e-9)
}

func TestParseFxResponse_Errors(t *testing.T) {
	if _, err := parseFxResponse([]byte(`{"result":"error","rates":{}}`), nil); err == nil {
		t.Fatal("expected error for failure result")
	}
	if _, err := parseFxResponse([]byte(`not-json`), nil); err == nil {
		t.Fatal("expected error for bad json")
	}
	if _, err := parseFxResponse([]byte(`{"result":"success","rates":{"JPY":20}}`), SupportedQuoteCcys); err == nil {
		t.Fatal("expected error when targets missing")
	}
}
