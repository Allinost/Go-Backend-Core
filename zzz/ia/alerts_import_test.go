package ia

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CSV 对账单解析
// ============================================================================

const sampleStatement = `trade_date,trade_type,code,market,name,asset_type,shares,price,fee,tax,amount
2026/08/01,buy,600519,CN-SH,贵州茅台,,100,1500.00,5.00,0,
2026-08-15,卖出,600519,CN-SH,贵州茅台,,40,1680.00,5.60,0,
2026-08-20,分红,000858,,理财宝,wealth,200,1.20,0,48,`

func TestParseStatementCSV(t *testing.T) {
	rows, err := ParseStatementCSV(sampleStatement)
	require.NoError(t, err)
	require.Len(t, rows, 3)

	// 第一行：斜杠日期 + 英文类型
	assert.Equal(t, "2026-08-01", rows[0].Date.Format(dateLayout))
	assert.Equal(t, string(TradeBuy), rows[0].Type)
	assert.Equal(t, "600519", rows[0].Code)
	assert.Equal(t, 1500.0, rows[0].Price)

	// 第二行：中文别名"卖出"
	assert.Equal(t, string(TradeSell), rows[1].Type)
	assert.Equal(t, 40.0, rows[1].Shares)

	// 第三行：中文"分红"，缺 market 但 wealth+code 无法推断 → 需显式或空
	assert.Equal(t, string(TradeDividendCash), rows[2].Type)
	assert.Equal(t, "wealth", rows[2].AssetType)
}

func TestParseStatementCSV_Errors(t *testing.T) {
	// 缺表头
	if _, err := ParseStatementCSV("a,b,c\n1,2,3"); err == nil {
		t.Fatal("expected header error")
	}
	// 空数据
	if _, err := ParseStatementCSV("trade_date,trade_type,code\n"); err == nil {
		t.Fatal("expected empty error")
	}
	// 非法日期（含行号）
	if _, err := ParseStatementCSV("trade_date,trade_type,code\ndeadbeef,buy,600519"); err == nil {
		t.Fatal("expected date error")
	} else {
		assert.Contains(t, err.Error(), "第 2 行")
	}
	// 未知交易类型（含行号）
	if _, err := ParseStatementCSV("trade_date,trade_type,code\n2026-08-01,打新,600519"); err == nil {
		t.Fatal("expected type error")
	}
}

func TestParseCSVDate(t *testing.T) {
	for _, s := range []string{"2026-08-26", "2026/08/26", "20260826"} {
		got, err := parseCSVDate(s)
		require.NoError(t, err, s)
		assert.Equal(t, 2026, got.Year())
	}
	if _, err := parseCSVDate("not-a-date"); err == nil {
		t.Fatal("expected error")
	}
}

func TestInferMarket(t *testing.T) {
	assert.Equal(t, string(MarketCNSH), inferMarket("600519", string(AssetTypeStock)))
	assert.Equal(t, string(MarketCNSZ), inferMarket("000858", string(AssetTypeStock)))
	assert.Equal(t, string(MarketCNSZ), inferMarket("300750", string(AssetTypeStock)))
	assert.Equal(t, string(MarketCNSH), inferMarket("510300", string(AssetTypeETF)))
	assert.Equal(t, string(MarketCNSZ), inferMarket("159915", string(AssetTypeETF)))
	assert.Empty(t, inferMarket("270042", "")) // 无法推断需显式提供
}

func TestInferDefaultAssetType(t *testing.T) {
	assert.Equal(t, string(AssetTypeDeposit), inferDefaultAssetType(string(TradeInterest)))
	assert.Equal(t, string(AssetTypeStock), inferDefaultAssetType(string(TradeBuy)))
}

// ============================================================================
// 价格告警
// ============================================================================

func TestAlertHit(t *testing.T) {
	assert.True(t, alertHit(string(AlertTakeProfit), 10, 10.5))
	assert.False(t, alertHit(string(AlertTakeProfit), 10, 9.9))

	assert.True(t, alertHit(string(AlertStopLoss), 10, 9.5))
	assert.False(t, alertHit(string(AlertStopLoss), 10, 10.1))

	// 到价：±1% 区间
	assert.True(t, alertHit(string(AlertTargetHit), 100, 100.9))
	assert.False(t, alertHit(string(AlertTargetHit), 100, 101.5))
	assert.False(t, alertHit(string(AlertTargetHit), 100, 98.8))
}

func TestAlertHit_ZeroGuard(t *testing.T) {
	assert.False(t, alertHit(string(AlertTakeProfit), 0, 100))
	assert.False(t, alertHit(string(AlertTakeProfit), 10, 0))
}

func TestBuildAlertMessage(t *testing.T) {
	msg := buildAlertMessage([]AlertTriggeredView{
		{Code: "600519", Name: "贵州茅台", AlertType: "take_profit", TargetPrice: 1500, LastPrice: 1600},
	})
	assert.Contains(t, msg, "贵州茅台")
	assert.Contains(t, msg, "止盈")
	assert.Contains(t, msg, "1600")
}

func TestCategoryName(t *testing.T) {
	assert.Equal(t, "股票", categoryName(string(AllocStock)))
	assert.Equal(t, "现金", categoryName(string(AllocCash)))
	assert.Equal(t, "unknown", categoryName("unknown"))
}

func TestOrDefault(t *testing.T) {
	assert.Equal(t, "def", orDefault("", "def"))
	assert.Equal(t, "v", orDefault("v", "def"))
}

var _ = time.Now
