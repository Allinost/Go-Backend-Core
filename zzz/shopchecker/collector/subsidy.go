package collector

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// flexFloat 兼容接口返回字符串/数字两种形式的数值。
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	s = strings.Trim(s, `"`)
	s = strings.TrimSpace(strings.Trim(s, "¥￥ "))
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		*f = 0
		return nil
	}
	*f = flexFloat(v)
	return nil
}

func (f flexFloat) Float64() float64 { return float64(f) }

// unmarshalFlexible 解析可能包含字符串数字的 JSON；解析失败时不返回错误（尽量降级）。
func unmarshalFlexible(data string, out any) bool {
	if data == "" {
		return false
	}
	return json.Unmarshal([]byte(data), out) == nil
}

// SubsidyHint 页面文案中的国补提示。
// Amount/Rate 至少有一个有效；Cap 为“最高减/封顶”等文案给出的上限。
type SubsidyHint struct {
	Amount float64
	Rate   float64
	Cap    float64
}

var (
	// “减￥500”“立减 500 元”“国补优惠500元”等明确金额
	subsidyAmountRes = []*regexp.Regexp{
		regexp.MustCompile(`(?:立减|直减|减|优惠|补贴|返现|返|省)\s*(?:最高|至多)?\s*[¥￥]?\s*([0-9]+(?:\.[0-9]+)?)\s*(?:元|块)?`),
		regexp.MustCompile(`[¥￥]\s*([0-9]+(?:\.[0-9]+)?)\s*(?:元)?\s*(?:国补|国家补贴|政府补贴|补贴|优惠)`),
		regexp.MustCompile(`(?:国补|国家补贴|政府补贴|补贴)\s*(?:金额|额度)?\s*[:：]?\s*([0-9]+(?:\.[0-9]+)?)\s*(?:元|块)`),
	}
	// “国补15%”“国家补贴 20 %”“15%国补”等比例
	subsidyRateRes = []*regexp.Regexp{
		regexp.MustCompile(`(?:国补|国家补贴|政府补贴|补贴|优惠)\s*(?:力度|比例|折扣)?\s*[:：]?\s*([0-9]+(?:\.[0-9]+)?)\s*%`),
		regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*%\s*(?:的)?\s*(?:国补|国家补贴|政府补贴|补贴)`),
	}
	// “最高减500”“封顶￥2000”
	subsidyCapRe = regexp.MustCompile(`(?:最高|封顶|上限)\s*(?:减|补贴|优惠|立减)?\s*[¥￥]?\s*([0-9]+(?:\.[0-9]+)?)`)

	// 只把明确的国补文案作为补贴线索；泛化的“补贴/企业补贴/超级补贴”容易误伤到手价。
	subsidyKeywordRe = regexp.MustCompile(`国补|国家补贴|政府补贴`)
)

// ParseSubsidyHint 从页面文案中解析国补金额/比例/上限。
// 只识别带“减/优惠/补贴/元/%”等语义的数字，避免把“国补后 3999”误当补贴金额。
func ParseSubsidyHint(text string) SubsidyHint {
	text = strings.TrimSpace(text)
	if text == "" {
		return SubsidyHint{}
	}
	var hint SubsidyHint

	if m := subsidyCapRe.FindStringSubmatch(text); len(m) >= 2 {
		hint.Cap = parseFloatStr(m[1])
	}
	// 先解析比例，避免把“补贴15%”里的 15 误当成 15 元。
	for _, re := range subsidyRateRes {
		if m := re.FindStringSubmatch(text); len(m) >= 2 {
			if v := parseFloatStr(m[1]); v > 0 && v < 100 {
				hint.Rate = v
				break
			}
		}
	}
	if v := findSubsidyAmount(text); v > 0 {
		hint.Amount = v
	}
	return hint
}

// findSubsidyAmount 查找金额，跳过后面紧跟 % 的数字（比例不是金额）。
func findSubsidyAmount(text string) float64 {
	for _, re := range subsidyAmountRes {
		for _, idx := range re.FindAllStringSubmatchIndex(text, -1) {
			if len(idx) < 4 || idx[2] < 0 || idx[3] < 0 {
				continue
			}
			end := idx[1]
			for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
				end++
			}
			if end < len(text) && (strings.HasPrefix(text[end:], "%") || strings.HasPrefix(text[end:], "％")) {
				continue
			}
			if v := parseFloatStr(text[idx[2]:idx[3]]); v > 0 {
				return v
			}
		}
	}
	return 0
}

// ExtractSubsidyContext 从长文本（如页面 innerText）中截取国补关键字附近的片段。
// 返回空字符串表示文本中没有国补/补贴提示。
func ExtractSubsidyContext(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	matches := subsidyKeywordRe.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return ""
	}
	var b strings.Builder
	seen := map[[2]int]bool{}
	for _, loc := range matches {
		start := loc[0] - 100
		if start < 0 {
			start = 0
		}
		end := loc[1] + 160
		if end > len(text) {
			end = len(text)
		}
		key := [2]int{start, end}
		if seen[key] {
			continue
		}
		seen[key] = true
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(text[start:end])
		if b.Len() > 1200 {
			break
		}
	}
	return b.String()
}

// HasSubsidyHint 判断文本中是否出现国补/补贴相关提示。
func HasSubsidyHint(text string) bool {
	return subsidyKeywordRe.MatchString(text)
}

// ComputeSubsidizedPrice 根据基础价 + 国补金额/比例计算到手价与补贴金额。
// 金额优先于比例；比例场景支持文案中的“最高减/封顶”上限。
// 结果会做基本合理性校验，避免把解析噪音当成价格。
func ComputeSubsidizedPrice(basePrice, amount, rate, cap float64) (finalPrice, appliedAmount float64) {
	if basePrice <= 0 {
		return 0, 0
	}
	if amount <= 0 && rate > 0 && rate < 100 {
		amount = round2(basePrice * rate / 100)
	}
	if amount <= 0 {
		return 0, 0
	}
	if cap > 0 && amount > cap {
		amount = cap
	}
	// 补贴不能吃掉全部商品价格；超过 90% 视为解析异常。
	if amount >= basePrice*0.9 {
		return 0, 0
	}
	finalPrice = round2(basePrice - amount)
	if finalPrice <= 0 {
		return 0, 0
	}
	return finalPrice, round2(amount)
}

// FillSubsidy 把补贴计算结果写回 PriceResult，并保留基础价/来源信息。
func FillSubsidy(result *PriceResult, basePrice float64, hint SubsidyHint, source string) {
	if result == nil || basePrice <= 0 {
		return
	}
	finalPrice, amount := ComputeSubsidizedPrice(basePrice, hint.Amount, hint.Rate, hint.Cap)
	if finalPrice <= 0 {
		return
	}
	result.BasePrice = round2(basePrice)
	result.SubsidyAmount = amount
	result.SubsidyRate = hint.Rate
	result.CurrentPrice = finalPrice
	result.PriceSource = source
	if result.OriginalPrice <= 0 || result.OriginalPrice < result.BasePrice {
		result.OriginalPrice = result.BasePrice
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
