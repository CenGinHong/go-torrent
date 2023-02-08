package bencode

import (
	"errors"
	"io"
	"reflect"
	"strings"
)

func Unmarshal(r io.Reader, s interface{}) error {
	// 非指针报错
	p := reflect.ValueOf(s)
	if p.Kind() != reflect.Ptr {
		return errors.New("dest must be a pointer")
	}
	// 解析成BObject
	o, err := Parse(r)
	if err != nil {
		return err
	}
	switch o.type_ {
	case BLIST:
		list, err := o.List()
		if err != nil {
			return err
		}
		// p实际只提供一个类型的模板，并不需要对p直接操作，而是根据p的类型去构造新的值对象
		l := reflect.MakeSlice(p.Elem().Type(), len(list), len(list))
		p.Elem().Set(l)
		if err = unmarshalList(p, list); err != nil {
			return err
		}
	case BDICT:
		// 传进来一般都是struct,不需要去构建slice
		dict, _ := o.Dict()
		err = unmarshalDict(p, dict)
		if err != nil {
			return err
		}
	default:
		return errors.New("src code must be struct or slice")
	}
	return nil
}

// unmarshalList
func unmarshalList(p reflect.Value, list []*BObject) error {
	// p需要是指针
	// p指向的值的类型必须是slice
	if p.Kind() != reflect.Ptr || p.Elem().Type().Kind() != reflect.Slice {
		return errors.New("dest must be pointer to slice")
	}
	v := p.Elem()
	if len(list) == 0 {
		return nil
	}
	switch list[0].type_ {
	case BSTR:
		for i, o := range list {
			val, err := o.Str()
			if err != nil {
				return err
			}
			v.Index(i).SetString(val)
		}
	case BINT:
		for i, o := range list {
			val, err := o.Int()
			if err != nil {
				return err
			}
			v.Index(i).SetInt(int64(val))
		}
	case BLIST:
		for i, o := range list {
			val, err := o.List()
			if err != nil {
				return err
			}
			// 保证列表的元素也是列表
			if v.Type().Elem().Kind() != reflect.Slice {
				return ErrTyp
			}
			// 做一个指针指向数组地址
			lp := reflect.New(v.Type().Elem())
			// 开拓带有长度等信息的数组并指向
			ls := reflect.MakeSlice(v.Type().Elem(), len(val), len(val))
			lp.Elem().Set(ls)
			if err = unmarshalList(lp, val); err != nil {
				return err
			}
			v.Index(i).Set(lp.Elem())
		}
	case BDICT:
		for i, o := range list {
			val, err := o.Dict()
			if v.Type().Elem().Kind() != reflect.Struct {
				return ErrTyp
			}
			dp := reflect.New(v.Type().Elem())
			err = unmarshalDict(dp, val)
			if err != nil {
				return err
			}
			v.Index(i).Set(dp.Elem())
		}
	}
	return nil
}

func unmarshalDict(p reflect.Value, dict map[string]*BObject) error {
	if p.Kind() != reflect.Ptr || p.Elem().Type().Kind() != reflect.Struct {
		return errors.New("dest must be a pointer")
	}
	v := p.Elem()
	for i, n := 0, v.NumField(); i < n; i++ {
		// 遍历每一个字段，找可以bencode里的值
		fv := v.Field(i)
		if !fv.CanSet() {
			continue
		}
		ft := v.Type().Field(i)
		key := ft.Tag.Get("bencode")
		// 如果没有tag,使用结构体名字
		if key == "" {
			key = strings.ToLower(ft.Name)
		}
		fo := dict[key]
		if fo == nil {
			continue
		}
		switch fo.type_ {
		case BSTR:
			if ft.Type.Kind() != reflect.String {
				break
			}
			val, err := fo.Str()
			if err != nil {
				return err
			}
			fv.SetString(val)
		case BINT:
			if ft.Type.Kind() != reflect.Int {
				break
			}
			val, err := fo.Int()
			if err != nil {
				return err
			}
			fv.SetInt(int64(val))
		case BLIST:
			if ft.Type.Kind() != reflect.Slice {
				break
			}
			list, err := fo.List()
			if err != nil {
				return err
			}
			lp := reflect.New(ft.Type)
			ls := reflect.MakeSlice(ft.Type, len(list), len(list))
			lp.Elem().Set(ls)
			if err = unmarshalList(lp, list); err != nil {
				break
			}
			fv.Set(lp.Elem())
		case BDICT:
			if ft.Type.Kind() != reflect.Struct {
				break
			}
			dp := reflect.New(ft.Type)
			d, _ := fo.Dict()
			if err := unmarshalDict(dp, d); err != nil {
				break
			}
			fv.Set(dp.Elem())
		}
	}
	return nil
}

func Marshal(w io.Writer, s interface{}) int {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return marshalValue(w, v)
}

func marshalValue(w io.Writer, v reflect.Value) int {
	wLen := 0
	switch v.Kind() {
	case reflect.String:
		wLen += EncodeString(w, v.String())
	case reflect.Int:
		wLen += EncodeInt(w, int(v.Int()))
	case reflect.Slice:
		wLen += marshalList(w, v)
	case reflect.Struct:
		wLen += marshalDict(w, v)
	}
	return wLen
}

func marshalList(w io.Writer, v reflect.Value) int {
	wLen := 2
	_, _ = w.Write([]byte{'l'})
	for i := 0; i < v.Len(); i++ {
		e := v.Index(i)
		wLen += marshalValue(w, e)
	}
	_, _ = w.Write([]byte{'e'})
	return wLen
}

func marshalDict(w io.Writer, v reflect.Value) int {
	wLen := 2
	_, _ = w.Write([]byte{'d'})
	for i := 0; i < v.NumField(); i++ {
		fv := v.Field(i)
		ft := v.Type().Field(i)
		key := ft.Tag.Get("bencode")
		if key == "" {
			key = strings.ToLower(ft.Name)
		}
		wLen += EncodeString(w, key)
		wLen += marshalValue(w, fv)
	}
	_, _ = w.Write([]byte{'e'})
	return wLen
}
