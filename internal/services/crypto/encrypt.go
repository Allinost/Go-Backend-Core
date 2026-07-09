package crypto

import (
	"fmt"
	"reflect"
)

type FieldEncryptor struct {
	km *KeyManager
}

func NewFieldEncryptor(km *KeyManager) *FieldEncryptor {
	return &FieldEncryptor{km: km}
}

func (fe *FieldEncryptor) EncryptFields(model interface{}) error {
	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("crypto: 需要结构体或结构体指针, 得到 %T", model)
	}

	return fe.encryptStruct(val)
}

func (fe *FieldEncryptor) DecryptFields(model interface{}) error {
	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("crypto: 需要结构体或结构体指针, 得到 %T", model)
	}

	return fe.decryptStruct(val)
}

func (fe *FieldEncryptor) EncryptSlice(models interface{}) error {
	val := reflect.ValueOf(models)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Slice {
		return fmt.Errorf("crypto: 需要切片, 得到 %T", models)
	}

	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		if !item.CanAddr() {
			item = item.Addr().Elem()
		}
		if err := fe.encryptStruct(item); err != nil {
			return fmt.Errorf("crypto: [%d] %w", i, err)
		}
	}
	return nil
}

func (fe *FieldEncryptor) DecryptSlice(models interface{}) error {
	val := reflect.ValueOf(models)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Slice {
		return fmt.Errorf("crypto: 需要切片, 得到 %T", models)
	}

	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		if !item.CanAddr() {
			item = item.Addr().Elem()
		}
		if err := fe.decryptStruct(item); err != nil {
			return fmt.Errorf("crypto: [%d] %w", i, err)
		}
	}
	return nil
}

func (fe *FieldEncryptor) encryptStruct(val reflect.Value) error {
	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("encrypt")
		if tag == "" || tag == "false" {
			continue
		}

		fv := val.Field(i)
		if !fv.CanSet() || !fv.CanInterface() {
			continue
		}

		switch fv.Kind() {
		case reflect.String:
			if fv.String() != "" {
				enc, err := fe.km.EncryptString(fv.String())
				if err != nil {
					return fmt.Errorf("crypto: 加密 %s 失败: %w", field.Name, err)
				}
				fv.SetString(enc)
			}
		case reflect.Slice:
			if fv.Type().Elem().Kind() == reflect.Uint8 && fv.Len() > 0 {
				enc, err := fe.km.Encrypt(fv.Bytes())
				if err != nil {
					return fmt.Errorf("crypto: 加密 %s 失败: %w", field.Name, err)
				}
				fv.SetBytes(enc)
			}
		case reflect.Ptr:
			if !fv.IsNil() {
				elem := fv.Elem()
				if elem.Kind() == reflect.String {
					if elem.String() != "" {
						enc, err := fe.km.EncryptString(elem.String())
						if err != nil {
							return fmt.Errorf("crypto: 加密 %s 失败: %w", field.Name, err)
						}
						fv.Set(reflect.ValueOf(&enc))
					}
				} else if elem.Kind() == reflect.Slice && elem.Type().Elem().Kind() == reflect.Uint8 {
					enc, err := fe.km.Encrypt(elem.Bytes())
					if err != nil {
						return fmt.Errorf("crypto: 加密 %s 失败: %w", field.Name, err)
					}
					fv.Set(reflect.ValueOf(&enc))
				}
			}
		}
	}
	return nil
}

func (fe *FieldEncryptor) decryptStruct(val reflect.Value) error {
	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("encrypt")
		if tag == "" || tag == "false" {
			continue
		}

		fv := val.Field(i)
		if !fv.CanSet() || !fv.CanInterface() {
			continue
		}

		switch fv.Kind() {
		case reflect.String:
			if fv.String() != "" && isEncrypted(fv.String()) {
				dec, err := fe.km.DecryptString(fv.String())
				if err != nil {
					return fmt.Errorf("crypto: 解密 %s 失败: %w", field.Name, err)
				}
				fv.SetString(dec)
			}
		case reflect.Slice:
			if fv.Type().Elem().Kind() == reflect.Uint8 && fv.Len() > 0 && isEncryptedBytes(fv.Bytes()) {
				dec, err := fe.km.Decrypt(fv.Bytes())
				if err != nil {
					return fmt.Errorf("crypto: 解密 %s 失败: %w", field.Name, err)
				}
				fv.SetBytes(dec)
			}
		case reflect.Ptr:
			if !fv.IsNil() {
				elem := fv.Elem()
				if elem.Kind() == reflect.String {
					if elem.String() != "" && isEncrypted(elem.String()) {
						dec, err := fe.km.DecryptString(elem.String())
						if err != nil {
							return fmt.Errorf("crypto: 解密 %s 失败: %w", field.Name, err)
						}
						fv.Set(reflect.ValueOf(&dec))
					}
				}
			}
		}
	}
	return nil
}

func isEncrypted(s string) bool {
	if len(s) < 3 {
		return false
	}
	return (s[0] == 'v' && (s[1] >= '0' && s[1] <= '9') && s[2] == ':') || len(s) > 32
}

func isEncryptedBytes(data []byte) bool {
	return len(data) > 12
}
