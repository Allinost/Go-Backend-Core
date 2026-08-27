package ia

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// dateAt 构造本地时间
func dateAt(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.Local)
}

func TestInQuoteWindow(t *testing.T) {
	s := NewJobScheduler(nil)

	// 周内交易时段
	mon10am := dateAt(2026, 8, 24, 10, 0) // 周一
	assert.True(t, s.inQuoteWindow(mon10am))

	// 窗口边界
	mon9am := dateAt(2026, 8, 24, 9, 0)
	assert.True(t, s.inQuoteWindow(mon9am))
	mon8am := dateAt(2026, 8, 24, 8, 59)
	assert.False(t, s.inQuoteWindow(mon8am))
	mon16pm := dateAt(2026, 8, 24, 16, 0)
	assert.False(t, s.inQuoteWindow(mon16pm))

	// 周末
	sat10am := dateAt(2026, 8, 29, 10, 0)
	assert.False(t, s.inQuoteWindow(sat10am))
}

func TestCNYConverter(t *testing.T) {
	conv := cnyConverter(map[string]float64{
		"CNY->USD": 7.16,
	})

	assert.Equal(t, 100.0, conv("CNY", 100))
	assert.Equal(t, 716.0, conv("USD", 100))
	assert.Equal(t, 0.0, conv("JPY", 100), "缺汇率时不折算")
	assert.Equal(t, 0.0, conv("USD", 0))
}
