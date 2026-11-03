package goodser

import (
	"strings"
)

// storageImageMarker 是对象存储中商品图片的路径标记。
const storageImageMarker = "/goodser/images/"

// normalizeImageURL 把对象存储的预签名 URL 归一为公开访问 URL。
//
// 历史商品保存的是 7 天有效的预签名 URL，过期后浏览器会拿到 XML 错误
// （表现为 ERR_BLOCKED_BY_ORB）。对象存储桶为公开读时，去掉签名查询串即可
// 长期访问；读取时归一化也顺带修复了存量数据。
func normalizeImageURL(raw *string) *string {
	if raw == nil {
		return nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return raw
	}
	idx := strings.Index(value, storageImageMarker)
	if idx < 0 {
		return raw
	}
	if cut := strings.IndexAny(value, "?#"); cut >= 0 {
		value = value[:cut]
	}
	return &value
}
