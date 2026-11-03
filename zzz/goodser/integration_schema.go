package goodser

import (
	"encoding/json"

	"github.com/Allinost/go-backend-core/internal/services/action"
)

// actionParamsSchemas 描述各动作 params 的 JSON Schema。
// MCP tools/list 会把它放进工具的 inputSchema.params，模型可以据此生成参数。
// 未列出的动作仍使用通用 params 对象。
var actionParamsSchemas = map[string]json.RawMessage{
	"goodser.stock.query":          stockQuerySchema(),
	"goodser.customer.stock_query": stockQuerySchema(),

	"goodser.order.list": actionSchema(`{
		"type":"object",
		"properties":{
			"inventory_id":{"type":"string","description":"库存目录 ID"},
			"page":{"type":"integer","minimum":1},
			"page_size":{"type":"integer","minimum":1,"maximum":100}
		},
		"required":["inventory_id"],
		"additionalProperties":false
	}`),

	"goodser.order.create": actionSchema(`{
		"type":"object",
		"properties":{
			"inventory_id":{"type":"string","description":"库存目录 ID"},
			"order_no":{"type":"string","description":"可选，指定单号"},
			"type":{"type":"string","enum":["outbound","reserve"],"description":"出库或预留，默认出库"},
			"order_info":{"type":"string","description":"订单备注信息"},
			"remark":{"type":"string"},
			"items":` + productItemsSchema + `,
			"combo_id":{"type":"string","description":"组合包 ID，与 items/bundle 互斥"},
			"combo_quantity":{"type":"integer","minimum":1},
			"bundle":` + bundleSelectionSchema + `
		},
		"required":["inventory_id"],
		"additionalProperties":false
	}`),

	"goodser.order.confirm": actionSchema(`{
		"type":"object",
		"properties":{"id":{"type":"string","description":"出库单 ID"}},
		"required":["id"],
		"additionalProperties":false
	}`),

	"goodser.order.cancel": actionSchema(`{
		"type":"object",
		"properties":{"id":{"type":"string","description":"出库/预留单 ID"}},
		"required":["id"],
		"additionalProperties":false
	}`),

	"goodser.return.create": actionSchema(`{
		"type":"object",
		"properties":{
			"outbound_order_id":{"type":"string","description":"退货对应的出库单 ID"},
			"reason":{"type":"string"},
			"remark":{"type":"string"},
			"items":{"type":"array","minItems":1,"items":{
				"type":"object",
				"properties":{
					"product_id":{"type":"string"},
					"quantity":{"type":"integer","minimum":1},
					"restock":{"type":"boolean","description":"是否回库，默认 true"}
				},
				"required":["product_id","quantity"]
			}}
		},
		"required":["outbound_order_id","items"],
		"additionalProperties":false
	}`),

	"goodser.return.confirm": actionSchema(`{
		"type":"object",
		"properties":{"id":{"type":"string","description":"退货单 ID"}},
		"required":["id"],
		"additionalProperties":false
	}`),

	"goodser.combo.list": actionSchema(`{
		"type":"object",
		"properties":{
			"inventory_id":{"type":"string"},
			"status":{"type":"string"}
		},
		"additionalProperties":false
	}`),

	"goodser.combo.create": actionSchema(`{
		"type":"object",
		"properties":{
			"inventory_id":{"type":"string"},
			"code":{"type":"string"},
			"name":{"type":"string"},
			"description":{"type":"string"},
			"image_url":{"type":"string"},
			"price":{"type":"number","minimum":0},
			"status":{"type":"string"},
			"items":{"type":"array","minItems":1,"items":{
				"type":"object",
				"properties":{
					"product_id":{"type":"string"},
					"quantity":{"type":"integer","minimum":1}
				},
				"required":["product_id","quantity"]
			}}
		},
		"required":["inventory_id","code","name","items"],
		"additionalProperties":false
	}`),

	"goodser.bundle.list": actionSchema(`{
		"type":"object",
		"properties":{
			"inventory_id":{"type":"string"},
			"status":{"type":"string"}
		},
		"additionalProperties":false
	}`),

	"goodser.bundle.get": actionSchema(`{
		"type":"object",
		"properties":{"bundle_id":{"type":"string"}},
		"required":["bundle_id"],
		"additionalProperties":false
	}`),

	"goodser.bundle.create": actionSchema(`{
		"type":"object",
		"properties":{
			"inventory_id":{"type":"string"},
			"code":{"type":"string"},
			"name":{"type":"string"},
			"description":{"type":"string"},
			"status":{"type":"string"},
			"main_filter":{"type":"object","description":"主商品筛选条件"},
			"main_product_ids":{"type":"array","items":{"type":"string"}},
			"sub_products":{"type":"array","items":{
				"type":"object",
				"properties":{
					"product_id":{"type":"string"},
					"default_quantity":{"type":"integer","minimum":1}
				},
				"required":["product_id"]
			}},
			"min_main_quantity":{"type":"integer","minimum":0},
			"min_main_amount":{"type":"number","minimum":0},
			"min_sub_quantity":{"type":"integer","minimum":0},
			"max_sub_quantity":{"type":"integer","minimum":0},
			"min_sub_amount":{"type":"number","minimum":0},
			"price_mode":{"type":"string"},
			"fixed_price":{"type":"number","minimum":0}
		},
		"required":["inventory_id","code","name"],
		"additionalProperties":false
	}`),

	"goodser.bundle.update": actionSchema(`{
		"type":"object",
		"properties":{
			"id":{"type":"string"},
			"code":{"type":"string"},
			"name":{"type":"string"},
			"description":{"type":"string"},
			"status":{"type":"string"},
			"main_filter":{"type":"object"},
			"main_product_ids":{"type":"array","items":{"type":"string"}},
			"sub_products":{"type":"array","items":{
				"type":"object",
				"properties":{
					"product_id":{"type":"string"},
					"default_quantity":{"type":"integer","minimum":1}
				},
				"required":["product_id"]
			}},
			"min_main_quantity":{"type":"integer","minimum":0},
			"min_main_amount":{"type":"number","minimum":0},
			"min_sub_quantity":{"type":"integer","minimum":0},
			"max_sub_quantity":{"type":"integer","minimum":0},
			"min_sub_amount":{"type":"number","minimum":0},
			"price_mode":{"type":"string"},
			"fixed_price":{"type":"number","minimum":0}
		},
		"required":["id"],
		"additionalProperties":false
	}`),

	"goodser.bundle.delete": actionSchema(`{
		"type":"object",
		"properties":{"bundle_id":{"type":"string"}},
		"required":["bundle_id"],
		"additionalProperties":false
	}`),

	"goodser.bundle.preview": actionSchema(`{
		"type":"object",
		"properties":{
			"bundle_id":{"type":"string"},
			"main_product_id":{"type":"string"},
			"main_quantity":{"type":"integer","minimum":1},
			"quantity":{"type":"integer","minimum":1},
			"sub_items":{"type":"array","items":{
				"type":"object",
				"properties":{
					"product_id":{"type":"string"},
					"quantity":{"type":"integer","minimum":1}
				},
				"required":["product_id","quantity"]
			}}
		},
		"required":["bundle_id","main_product_id"],
		"additionalProperties":false
	}`),

	"goodser.dictionary.list": actionSchema(`{
		"type":"object",
		"properties":{
			"kind":{"type":"string"},
			"enabled":{"type":"boolean"}
		},
		"additionalProperties":false
	}`),

	"goodser.stats.summary": actionSchema(`{
		"type":"object",
		"properties":{
			"inventory_id":{"type":"string"},
			"days":{"type":"integer","minimum":1,"maximum":365}
		},
		"required":["inventory_id"],
		"additionalProperties":false
	}`),

	"goodser.customer.order_request": actionSchema(`{
		"type":"object",
		"properties":{
			"inventory_id":{"type":"string"},
			"customer_name":{"type":"string"},
			"contact":{"type":"string","description":"客户联系方式，如 wechat:xxx"},
			"items":` + productItemsSchema + `,
			"combo_id":{"type":"string","description":"组合包 ID，与 items/bundle 互斥"},
			"combo_quantity":{"type":"integer","minimum":1},
			"bundle":` + bundleSelectionSchema + `,
			"order_info":{"type":"string"},
			"remark":{"type":"string"}
		},
		"required":["inventory_id"],
		"additionalProperties":false
	}`),
}

const (
	productItemsSchema = `{
		"type":"array",
		"minItems":1,
		"items":{
			"type":"object",
			"properties":{
				"product_id":{"type":"string"},
				"quantity":{"type":"integer","minimum":1}
			},
			"required":["product_id","quantity"]
		}
	}`

	bundleSelectionSchema = `{
		"type":"object",
		"properties":{
			"bundle_id":{"type":"string"},
			"main_product_id":{"type":"string"},
			"main_quantity":{"type":"integer","minimum":1},
			"quantity":{"type":"integer","minimum":1},
			"sub_items":{"type":"array","items":{
				"type":"object",
				"properties":{
					"product_id":{"type":"string"},
					"quantity":{"type":"integer","minimum":1}
				},
				"required":["product_id","quantity"]
			}}
		},
		"required":["bundle_id","main_product_id"]
	}`
)

func actionSchema(raw string) json.RawMessage {
	return json.RawMessage(raw)
}

func stockQuerySchema() json.RawMessage {
	return actionSchema(`{
		"type":"object",
		"properties":{
			"inventory_id":{"type":"string","description":"库存目录 ID"},
			"keyword":{"type":"string","description":"商品名/编号关键词"},
			"series_id":{"type":"string"},
			"series_name":{"type":"string"},
			"character_id":{"type":"string"},
			"character_name":{"type":"string"},
			"set_type_id":{"type":"string"},
			"product_type_id":{"type":"string"},
			"product_type":{"type":"string"},
			"status_code":{"type":"string"},
			"include_out_of_stock":{"type":"boolean"},
			"page":{"type":"integer","minimum":1},
			"page_size":{"type":"integer","minimum":1,"maximum":50}
		},
		"required":["inventory_id"],
		"additionalProperties":false
	}`)
}

// applyActionParamsSchemas 在注册前把 params schema 挂到动作定义上。
func applyActionParamsSchemas(definitions []action.Definition) {
	for i := range definitions {
		if schema, ok := actionParamsSchemas[definitions[i].Name]; ok {
			definitions[i].ParamsSchema = schema
		}
	}
}
