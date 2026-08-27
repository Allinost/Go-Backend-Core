package quote

import (
	"testing"
	"time"

	pkgNet "github.com/Allinost/go-backend-core/pkg/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmSecid(t *testing.T) {
	assert.Equal(t, []string{"1.600519"}, emSecid("CN-SH", "600519"))
	assert.Equal(t, []string{"0.159915"}, emSecid("CN-SZ", "159915"))
	assert.Equal(t, []string{"116.00700"}, emSecid("HK", "00700"))
	us := emSecid("US", "AAPL")
	assert.Equal(t, []string{"105.AAPL", "106.AAPL", "107.AAPL"}, us)
	assert.Nil(t, emSecid("OTC", "270042"))
}

func TestEastmoneySupport(t *testing.T) {
	p := NewEastmoneyProvider(pkgNet.NewHTTPClient(pkgNet.DefaultHTTPConfig()))
	assert.True(t, p.Support(AssetInfo{Code: "600519", Market: "CN-SH", AssetType: "stock"}))
	assert.True(t, p.Support(AssetInfo{Code: "510300", Market: "CN-SH", AssetType: "etf"}))
	assert.False(t, p.Support(AssetInfo{Code: "270042", Market: "OTC", AssetType: "fund"}))
	assert.False(t, p.Support(AssetInfo{Code: "000001", Market: "BANK", AssetType: "deposit"}))
}

func TestParseEastmoney(t *testing.T) {
	body := `{"rc":0,"rt":4,"svr":1,"lt":1,"full":1,"data":{"f57":"600519","f58":"贵州茅台","f43":1525.60,"f60":1500.00,"f86":1756189200}}`
	q, err := parseEastmoney([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, "贵州茅台", q.Name)
	assert.Equal(t, 1525.60, q.Price)
	assert.Equal(t, 1500.00, q.PrevClose)
	assert.Equal(t, "official", q.Source)
	assert.NotEmpty(t, q.NavDate)
}

func TestParseEastmoney_NullFields(t *testing.T) {
	body := `{"data":{"f57":"AAPL","f58":"苹果","f43":228.36,"f60":"-","f86":null}}`
	q, err := parseEastmoney([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, 228.36, q.Price)
	assert.Equal(t, 0.0, q.PrevClose)
}

func TestParseEastmoney_Invalid(t *testing.T) {
	if _, err := parseEastmoney([]byte(`{"data":null}`)); err == nil {
		t.Fatal("expected error for null data")
	}
	if _, err := parseEastmoney([]byte(`not-json`)); err == nil {
		t.Fatal("expected error for invalid json")
	}
}

const sampleFundGz = `jsonpgz({"fundcode":"270042","name":"广发纳斯达克100ETF联接(QDII)A","jzrq":"2026-08-24","dwjz":"5.1230","gsz":"5.1800","gszzl":"1.11","gztime":"2026-08-25 15:00"});`

func TestParseFundGz_FreshNav(t *testing.T) {
	// now 与净值日同日：估值有效
	now, _ := time.ParseInLocation("2006-01-02 15:04", "2026-08-25 20:00", time.Local)
	q, err := ParseFundGzAt(sampleFundGz, now)
	require.NoError(t, err)
	assert.Equal(t, 5.1230, q.Nav)
	assert.Equal(t, "2026-08-24", q.NavDate)
	assert.Equal(t, 5.18, q.Estimate)
	assert.Equal(t, 5.18, q.Price, "估值有效时应采用估算净值")
	assert.Equal(t, "estimated", q.Source)
}

func TestParseFundGz_StaleNav(t *testing.T) {
	// 净值日超过宽限期：退化为官方净值
	now, _ := time.ParseInLocation("2006-01-02 15:04", "2026-09-10 20:00", time.Local)
	q, err := ParseFundGzAt(sampleFundGz, now)
	require.NoError(t, err)
	assert.Equal(t, 0.0, q.Estimate)
	assert.Equal(t, 5.1230, q.Price, "估值过期时应采用官方净值")
	assert.Equal(t, "official", q.Source)
}

func TestParseFundGz_NoEstimate(t *testing.T) {
	body := `jsonpgz({"fundcode":"270042","name":"测试基金","jzrq":"2026-08-24","dwjz":"2.0000","gsz":"","gszzl":""});`
	q, err := ParseFundGz(body)
	require.NoError(t, err)
	assert.Equal(t, 2.0, q.Price)
	assert.Equal(t, 0.0, q.Estimate)
	assert.Equal(t, "official", q.Source)
}

func TestParseFundGz_Invalid(t *testing.T) {
	if _, err := ParseFundGz(`(not json)`); err == nil {
		t.Fatal("expected error for malformed payload")
	}
	if _, err := ParseFundGz(`jsonpgz({"fundcode":"","dwjz":""});`); err == nil {
		t.Fatal("expected error for empty nav")
	}
}

func TestFundProvider_Support(t *testing.T) {
	p := NewFundProvider(pkgNet.NewHTTPClient(pkgNet.DefaultHTTPConfig()))
	assert.True(t, p.Support(AssetInfo{Code: "270042", Market: "OTC", AssetType: "fund"}))
	assert.False(t, p.Support(AssetInfo{Code: "270042", Market: "CN-SH", AssetType: "fund"}))
	assert.False(t, p.Support(AssetInfo{Code: "600519", Market: "CN-SH", AssetType: "stock"}))
}
