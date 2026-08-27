package ia

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDateStr(t *testing.T) {
	assert.True(t, isDateStr("2026-01-02"))
	assert.False(t, isDateStr("2026-1-2"))
	assert.False(t, isDateStr("2026/01/02"))
	assert.False(t, isDateStr(""))
}

func TestParseDateTime(t *testing.T) {
	cases := []string{
		"2026-08-26T10:30:00+08:00",
		"2026-08-26 10:30:00",
		"2026-08-26",
	}
	for _, c := range cases {
		got, err := parseDateTime(c)
		require.NoError(t, err, c)
		assert.Equal(t, 2026, got.Year(), c)
	}
	if _, err := parseDateTime("not-a-date"); err == nil {
		t.Fatal("expected error for invalid datetime")
	}
}

func TestSafeDiv(t *testing.T) {
	assert.Equal(t, 0.0, safeDiv(1, 0))
	assert.Equal(t, 2.5, safeDiv(5, 2))
}

func TestRounding(t *testing.T) {
	assert.Equal(t, 1234.5679, round4(1234.56789))
	assert.Equal(t, 1.234568, round6(1.2345678))
	assert.Equal(t, 0.0, round4(-0.00001))
}

func TestWeekdayNum(t *testing.T) {
	// 2026-08-24 是周一
	d := time.Date(2026, 8, 24, 0, 0, 0, 0, time.Local)
	for i := 0; i < 7; i++ {
		assert.Equal(t, i+1, weekdayNum(d.AddDate(0, 0, i)))
	}
}

func TestNextExecDate(t *testing.T) {
	today := truncateDate(time.Now())

	// 日定投：明天
	next := nextExecDate(today.Format(dateLayout), string(DCADaily), 1)
	assert.Equal(t, today.AddDate(0, 0, 1).Format(dateLayout), next)

	// 月定投：本月未过执行日则取本月，否则下月
	next = nextExecDate(today.Format(dateLayout), string(DCAMonthly), 28)
	parsed, err := time.Parse(dateLayout, next)
	require.NoError(t, err)
	assert.True(t, parsed.After(today), "下次执行日应晚于今天")
	assert.Equal(t, 28, parsed.Day())
}

func TestLockedShares(t *testing.T) {
	now := time.Now()
	asset := &Asset{LockDays: 90}

	// 封闭期内：全额锁定
	inLock := &Asset{LockDays: 36500, StartDate: now.AddDate(0, 0, -1).Format(dateLayout)}
	assert.Equal(t, 100.0, lockedShares(now, inLock, 100))

	// 封闭期结束：不锁定
	outLock := &Asset{LockDays: 30, StartDate: now.AddDate(0, 0, -60).Format(dateLayout)}
	assert.Equal(t, 0.0, lockedShares(now, outLock, 100))

	// 无封闭期配置
	assert.Equal(t, 0.0, lockedShares(now, asset, 100))
	assert.Equal(t, 0.0, lockedShares(now, &Asset{}, 100))
}

func TestCheckTradeCompatibility(t *testing.T) {
	// 分红/送转仅限证券类
	assert.Error(t, checkTradeCompatibility(string(TradeDividendCash), string(AssetTypeDeposit)))
	assert.NoError(t, checkTradeCompatibility(string(TradeDividendCash), string(AssetTypeStock)))
	// 利息仅限存款理财
	assert.Error(t, checkTradeCompatibility(string(TradeInterest), string(AssetTypeStock)))
	assert.NoError(t, checkTradeCompatibility(string(TradeInterest), string(AssetTypeWealth)))
	// 买卖通用
	assert.NoError(t, checkTradeCompatibility(string(TradeBuy), string(AssetTypeFund)))
}

func TestAllocCategoryOf(t *testing.T) {
	assert.Equal(t, AllocStock, allocCategoryOf(string(AssetTypeStock)))
	assert.Equal(t, AllocETF, allocCategoryOf(string(AssetTypeETF)))
	assert.Equal(t, AllocFund, allocCategoryOf(string(AssetTypeFund)))
	assert.Equal(t, AllocDeposit, allocCategoryOf(string(AssetTypeDeposit)))
	assert.Equal(t, AllocWealth, allocCategoryOf(string(AssetTypeWealth)))
	assert.Equal(t, AllocCategory(""), allocCategoryOf("unknown"))
}
