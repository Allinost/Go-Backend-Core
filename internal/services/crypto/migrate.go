package crypto

import (
	"fmt"
	"reflect"

	"gorm.io/gorm"
)

type FieldMeta struct {
	Table     string
	FieldName string
	Column    string
}

type EncryptedModel interface {
	EncryptedFields() []FieldMeta
}

func ReEncryptTable(db *gorm.DB, fe *FieldEncryptor, tableName string, model interface{}, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 100
	}

	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var total int64
	offset := 0

	for {
		slice := reflect.New(reflect.SliceOf(reflect.PtrTo(t))).Interface()
		if err := db.Table(tableName).Where("deleted_at IS NULL").Offset(offset).Limit(batchSize).Find(slice).Error; err != nil {
			return total, fmt.Errorf("crypto: 查询 %s 失败: %w", tableName, err)
		}

		val := reflect.ValueOf(slice)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Len() == 0 {
			break
		}

		for i := 0; i < val.Len(); i++ {
			item := val.Index(i)
			itemVal := item.Elem()

			if err := fe.DecryptFields(itemVal.Addr().Interface()); err != nil {
				return total, fmt.Errorf("crypto: 解密 %s[%d] 失败: %w", tableName, offset+i, err)
			}
			if err := fe.EncryptFields(itemVal.Addr().Interface()); err != nil {
				return total, fmt.Errorf("crypto: 重加密 %s[%d] 失败: %w", tableName, offset+i, err)
			}

			if err := db.Table(tableName).Where("id = ?", itemVal.FieldByName("ID").Interface()).Save(itemVal.Addr().Interface()).Error; err != nil {
				return total, fmt.Errorf("crypto: 保存 %s[%d] 失败: %w", tableName, offset+i, err)
			}
		}

		total += int64(val.Len())
		offset += batchSize
	}

	return total, nil
}

func ReEncryptRows(db *gorm.DB, fe *FieldEncryptor, tableName string, idField string, fieldNames []string, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 100
	}
	if idField == "" {
		idField = "id"
	}

	columns := append([]string{idField}, fieldNames...)

	var total int64
	offset := 0

	for {
		type row struct {
			ID     uint
			Values []string
		}

		rows := []map[string]any{}
		if err := db.Table(tableName).Select(columns).Where("deleted_at IS NULL").Offset(offset).Limit(batchSize).Find(&rows).Error; err != nil {
			return total, fmt.Errorf("crypto: 查询 %s 失败: %w", tableName, err)
		}

		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			id := row[idField]
			updates := map[string]any{}

			for _, field := range fieldNames {
				val, ok := row[field]
				if !ok {
					continue
				}
				str, ok := val.(string)
				if !ok || str == "" || !isEncrypted(str) {
					continue
				}

				if fe == nil {
					continue
				}

				decrypted, err := fe.km.DecryptString(str)
				if err != nil {
					continue
				}

				reEncrypted, err := fe.km.EncryptString(decrypted)
				if err != nil {
					continue
				}

				updates[field] = reEncrypted
			}

			if len(updates) > 0 {
				if err := db.Table(tableName).Where(idField+" = ?", id).Updates(updates).Error; err != nil {
					return total, fmt.Errorf("crypto: 更新 %s id=%v 失败: %w", tableName, id, err)
				}
			}
		}

		total += int64(len(rows))
		offset += batchSize
	}

	return total, nil
}
