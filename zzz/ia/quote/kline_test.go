package quote

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKline(t *testing.T) {
	body := []byte(`{"data":{"code":"1.000300","name":"沪深300","klines":["2026-08-24,4123.45","2026-08-25,4200.00"]}}`)
	points, err := ParseKline(body, "1.000300")
	require.NoError(t, err)
	require.Len(t, points, 2)
	assert.Equal(t, "2026-08-24", points[0].Date)
	assert.InDelta(t, 4123.45, points[0].Close, 1e-9)

	if _, err := ParseKline([]byte(`{"data":null}`), "1.000300"); err == nil {
		t.Fatal("expected error for null data")
	}
}

func TestNormalizeSeries(t *testing.T) {
	points := []KLinePoint{
		{Date: "2026-01-05", Close: 100},
		{Date: "2026-01-06", Close: 110},
		{Date: "2026-01-07", Close: 99},
	}
	norm := NormalizeSeries(points, "2026-01-06")
	require.NotNil(t, norm)
	assert.NotContains(t, norm, "2026-01-05", "基期之前的数据应被排除")
	assert.InDelta(t, 100.0, norm["2026-01-06"], 1e-9, "基期归一为 100")
	assert.InDelta(t, 90.0, norm["2026-01-07"], 1e-9)
}

func TestNormalizeSeries_EmptyBase(t *testing.T) {
	points := []KLinePoint{{Date: "2025-12-31", Close: 50}}
	assert.Nil(t, NormalizeSeries(points, "2026-01-05"), "基期晚于全部数据时返回 nil")
}

func TestFilterKlineByRange(t *testing.T) {
	points := []KLinePoint{
		{Date: "2026-01-01", Close: 1},
		{Date: "2026-01-15", Close: 2},
		{Date: "2026-02-01", Close: 3},
	}
	out := FilterKlineByRange(points, "2026-01-10", "")
	assert.Len(t, out, 2)
	out = FilterKlineByRange(points, "", "2026-01-20")
	assert.Len(t, out, 2)
}
