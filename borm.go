/*
   borm is a better orm library for Go.

  Copyright (c) 2019 <http://ez8.co> <orca.zhang@yahoo.com>

  This library is released under the MIT License.
  Please see LICENSE file or visit https://github.com/orca-zhang/borm for details.
*/

package borm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/modern-go/reflect2"
)

const (
	_fields = 1 >> iota
	_join
	_indexedBy
	_where
	_groupBy
	_having
	_orderBy
	_limit
	_onConflictDoUpdateSet

	_cond = iota
	_andCondEx
	_orCondEx
)

const (
	_timeLayout           = "2006-01-02 15:04:05"
	_timeLayoutWithZ      = "2006-01-02 15:04:05Z"
	_timeLayoutWithTZ     = "2006-01-02 15:04:05 -07:00"
	_timeLayoutWithNanoTZ = "2006-01-02 15:04:05.999999999 -07:00"
)

var config struct {
	Mock bool
}

// V - an alias object value type
type V map[string]interface{}

// U - an alias string type for update to support `x=x+1`
type U string

// Config .
type Config struct {
	Debug               bool
	Reuse               bool // 默认开启，提供2-14倍性能提升
	UseNameWhenTagEmpty bool
	ToTimestamp         bool
}

// Table .
func Table(db BormDBIFace, name string, ctx ...context.Context) *BormTable {
	if len(ctx) > 0 {
		return &BormTable{
			DB:   db,
			Name: name,
			ctx:  ctx[0],
			Cfg:  Config{Reuse: true}, // 默认开启Reuse（内建形状感知）
		}
	}
	return &BormTable{
		DB:   db,
		Name: name,
		ctx:  context.Background(),
		Cfg:  Config{Reuse: true}, // 默认开启Reuse（内建形状感知）
	}
}

// Reuse .
func (t *BormTable) Reuse() *BormTable {
	t.Cfg.Reuse = true
	return t
}

// NoReuse 关闭Reuse功能（如果不需要缓存优化）
func (t *BormTable) NoReuse() *BormTable {
	t.Cfg.Reuse = false
	return t
}

// Debug .
func (t *BormTable) Debug() *BormTable {
	t.Cfg.Debug = true
	return t
}

// UseNameWhenTagEmpty .
func (t *BormTable) UseNameWhenTagEmpty() *BormTable {
	t.Cfg.UseNameWhenTagEmpty = true
	return t
}

// ToTimestamp .
func (t *BormTable) ToTimestamp() *BormTable {
	t.Cfg.ToTimestamp = true
	return t
}

// SafeReuse 已合并进 Reuse，保持兼容
func (t *BormTable) SafeReuse() *BormTable { return t.Reuse() }

// NoSafeReuse 已合并进 Reuse，保持兼容
func (t *BormTable) NoSafeReuse() *BormTable { return t }

// buildShapeKey 基于调用点key和参数形状构建复用key
func buildShapeKey(baseKey string, op string, args []BormItem) string {
	var b strings.Builder
	b.WriteString(baseKey)
	b.WriteString("|")
	b.WriteString(op)
	for _, a := range args {
		b.WriteString("|")
		b.WriteString(strconv.Itoa(a.Type()))
		var sb strings.Builder
		a.BuildSQL(&sb)
		b.WriteString(sb.String())
	}
	return b.String()
}

// Fields .
func Fields(fields ...string) *fieldsItem {
	return &fieldsItem{Fields: fields}
}

// Join .
func Join(stmt string) *joinItem {
	return &joinItem{Stmt: stmt}
}

// Where .
func Where(conds ...interface{}) *whereItem {
	if l := len(conds); l > 0 {
		if s, ok := conds[0].(string); ok {
			return &whereItem{Conds: []interface{}{
				&ormCond{
					Op:   s,
					Args: conds[1:l],
				},
			}}
		}
		w := &whereItem{}
		for _, c := range conds {
			if condEx, ok := c.(*ormCondEx); ok {
				if len(condEx.Conds) <= 0 {
					continue
				}
			}
			w.Conds = append(w.Conds, c)
		}
		return w
	}
	panic("too few conditions")
}

// GroupBy .
func GroupBy(fields ...string) *groupByItem {
	return &groupByItem{Fields: fields}
}

// Having .
func Having(conds ...interface{}) *havingItem {
	if l := len(conds); l > 0 {
		if s, ok := conds[0].(string); ok {
			return &havingItem{Conds: []interface{}{
				&ormCond{
					Op:   s,
					Args: conds[1:l],
				},
			}}
		}
		h := &havingItem{}
		for _, c := range conds {
			if condEx, ok := c.(*ormCondEx); ok {
				if len(condEx.Conds) <= 0 {
					continue
				}
			}
			h.Conds = append(h.Conds, c)
		}
		return h
	}
	panic("too few conditions")
}

// OrderBy .
func OrderBy(orders ...string) *orderByItem {
	return &orderByItem{Orders: orders}
}

// Limit .
func Limit(i ...interface{}) *limitItem {
	switch len(i) {
	case 1, 2:
		return &limitItem{I: i}
	}
	panic("too few or too many limit params")
}

// OnConflictDoUpdateSet .
func OnConflictDoUpdateSet(fields []string, keyVals V) *onConflictDoUpdateSetItem {
	res := &onConflictDoUpdateSetItem{}
	if len(keyVals) <= 0 {
		return res
	}

	var sb strings.Builder
	sb.WriteString(" on conflict(")
	for i, field := range fields {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, field)
	}
	sb.WriteString(") do update set")
	argCnt := 0
	for k, v := range keyVals {
		if argCnt > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, k)
		if s, ok := v.(U); ok {
			sb.WriteString("=")
			sb.WriteString(string(s))
		} else {
			sb.WriteString("=?")
			res.Vals = append(res.Vals, v)
		}
		argCnt++
	}
	res.Conds = sb.String()
	return res
}

// IndexedBy .
func IndexedBy(idx string) *indexedByItem {
	return &indexedByItem{idx: idx}
}

// Select .
func (t *BormTable) Select(res interface{}, args ...BormItem) (int, error) {
	if len(args) <= 0 {
		return 0, errors.New("argument 2 cannot be omitted")
	}

	var (
		rt         = reflect2.TypeOf(res)
		isArray    bool
		isPtrArray bool
		rtElem     = rt

		item     *DataBindingItem
		stmtArgs []interface{}
	)

	switch rt.Kind() {
	case reflect.Ptr:
		rt = rt.(reflect2.PtrType).Elem()
		rtElem = rt
		if rt.Kind() == reflect.Slice {
			rtElem = rt.(reflect2.ListType).Elem()
			isArray = true

			if rtElem.Kind() == reflect.Ptr {
				rtElem = rtElem.(reflect2.PtrType).Elem()
				isPtrArray = true
			}
		}
	case reflect.Map:
		// 直接支持map类型
		rtElem = rt
	default:
		return 0, errors.New("argument 2 should be map or ptr")
	}

	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, data, n, e := checkMock(t.Name, "Select", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			rt.UnsafeSet(reflect2.PtrOf(res), reflect2.PtrOf(data))
			return n, e
		}
	}

	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey := buildShapeKey(callSite.Key, "Select", args)
		if i, ok := _dataBindingCache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil {
		// struct类型
		if rtElem.Kind() == reflect.Struct {
			if args[0].Type() == _fields {
				args = args[1:]
			}
		} else if rtElem.Kind() == reflect.Map {
			// map类型需要Fields，跳过缓存
			if args[0].Type() == _fields {
				args = args[1:]
			}
		} else {
			args = args[1:]
		}

		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		item = &DataBindingItem{Type: rtElem}

		var sb strings.Builder
		sb.WriteString("select ")

		if isArray {
			item.Elem = rtElem.New()
		} else {
			item.Elem = res
		}

		// struct类型
		if rtElem.Kind() == reflect.Struct {
			s := rtElem.(reflect2.StructType)

			if args[0].Type() == _fields {
				m := t.getStructFieldMap(s)

				for _, field := range args[0].(*fieldsItem).Fields {
					f := m[field]
					item.Cols = append(item.Cols, &scanner{
						Type: f.Type(),
						Val:  f.UnsafeGet(reflect2.PtrOf(item.Elem)),
					})
				}

				(args[0]).BuildSQL(&sb)
				args = args[1:]

			} else {
				for i := 0; i < s.NumField(); i++ {
					f := s.Field(i)
					ft := f.Tag().Get("borm")

					if !t.Cfg.UseNameWhenTagEmpty && ft == "" {
						continue
					}

					if len(item.Cols) > 0 {
						sb.WriteString(",")
					}

					if ft == "" {
						fieldEscape(&sb, f.Name())
					} else {
						fieldEscape(&sb, ft)
					}

					item.Cols = append(item.Cols, &scanner{
						Type: f.Type(),
						Val:  f.UnsafeGet(reflect2.PtrOf(item.Elem)),
					})
				}
			}
		} else if rtElem.Kind() == reflect.Map {
			// map类型必须指定Fields
			if args[0].Type() != _fields {
				return 0, errors.New("map type requires Fields() to specify columns")
			}

			fi := args[0].(*fieldsItem)
			// 存储Fields信息到item中，供后续使用
			item.Fields = fi.Fields

			for i, field := range fi.Fields {
				if i > 0 {
					sb.WriteString(",")
				}
				fieldEscape(&sb, field)

				// 为map创建interface{}类型的scanner
				var temp interface{}
				item.Cols = append(item.Cols, &scanner{
					Type: reflect2.TypeOf((*interface{})(nil)).(reflect2.PtrType).Elem(),
					Val:  unsafe.Pointer(&temp), // 临时指针，稍后会被替换
				})
			}
			args = args[1:]
		} else {
			// 必须有fields且为1
			if args[0].Type() != _fields {
				return 0, errors.New("argument 3 need ONE Fields(\"name\") with ONE field")
			}

			fi := args[0].(*fieldsItem)
			if len(fi.Fields) < 1 {
				return 0, errors.New("too few fields")
			}

			item.Cols = append(item.Cols, &scanner{
				Type: rtElem,
				Val:  reflect2.PtrOf(item.Elem),
			})

			fieldEscape(&sb, fi.Fields[0])
			args = args[1:]
		}

		sb.WriteString(" from ")

		fieldEscape(&sb, t.Name)

		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()

		if t.Cfg.Reuse {
			callSite := getCallSite()
			shapeKey := buildShapeKey(callSite.Key, "Select", args)
			_dataBindingCache.Store(shapeKey, item)
		}
	}

	if t.Cfg.Debug {
		log.Println(item.SQL, stmtArgs)
	}

	if !isArray {
		// fire
		if rtElem.Kind() == reflect.Map {
			// Map类型需要特殊处理
			values := make([]interface{}, len(item.Cols))
			for i := range values {
				values[i] = &values[i]
			}

			err := t.DB.QueryRowContext(t.ctx, item.SQL, stmtArgs...).Scan(values...)
			if err != nil {
				if err == sql.ErrNoRows {
					return 0, nil
				}
				return 0, err
			}

			// 构建map
			mapVal := reflect.MakeMap(rtElem.(reflect2.MapType).Type1())
			for i, field := range item.Fields {
				mapVal.SetMapIndex(reflect.ValueOf(field), reflect.ValueOf(values[i]))
			}

			// 设置到结果
			reflect.ValueOf(res).Elem().Set(mapVal)
			return 1, nil
		} else {
			err := t.DB.QueryRowContext(t.ctx, item.SQL, stmtArgs...).Scan(item.Cols...)
			if err != nil {
				if err == sql.ErrNoRows {
					return 0, nil
				}
				return 0, err
			}
			return 1, err
		}
	}

	// fire
	rows, err := t.DB.QueryContext(t.ctx, item.SQL, stmtArgs...)
	if err != nil {
		return 0, err
	}

	count := 0
	for rows.Next() {
		if rtElem.Kind() == reflect.Map {
			// Map类型需要特殊处理
			values := make([]interface{}, len(item.Cols))
			for i := range values {
				values[i] = &values[i]
			}

			err = rows.Scan(values...)
			if err != nil {
				break
			}

			// 构建map
			mapVal := reflect.MakeMap(rtElem.(reflect2.MapType).Type1())
			for i, field := range item.Fields {
				mapVal.SetMapIndex(reflect.ValueOf(field), reflect.ValueOf(values[i]))
			}

			// 添加到slice
			if isPtrArray {
				rt.(reflect2.SliceType).UnsafeAppend(reflect2.PtrOf(res), unsafe.Pointer(&mapVal))
			} else {
				// 使用reflect包来append
				reflect.ValueOf(res).Elem().Set(reflect.Append(reflect.ValueOf(res).Elem(), mapVal))
			}
		} else {
			err = rows.Scan(item.Cols...)
			if err != nil {
				break
			}

			if isPtrArray {
				copyElem := rtElem.UnsafeNew()
				rtElem.UnsafeSet(copyElem, reflect2.PtrOf(item.Elem))
				rt.(reflect2.SliceType).UnsafeAppend(reflect2.PtrOf(res), unsafe.Pointer(&copyElem))
			} else {
				rt.(reflect2.SliceType).UnsafeAppend(reflect2.PtrOf(res), reflect2.PtrOf(item.Elem))
			}
		}
		count++
	}
	rows.Close()
	return count, err
}

// InsertIgnore .
func (t *BormTable) InsertIgnore(objs interface{}, args ...BormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "InsertIgnore", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	return t.insert("insert or ignore into ", objs, args)
}

// ReplaceInto .
func (t *BormTable) ReplaceInto(objs interface{}, args ...BormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "ReplaceInto", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	return t.insert("replace into ", objs, args)
}

// Insert .
func (t *BormTable) Insert(objs interface{}, args ...BormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Insert", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	return t.insert("insert into ", objs, args)
}

func (t *BormTable) insert(prefix string, objs interface{}, args []BormItem) (int, error) {
	var (
		rt         = reflect2.TypeOf(objs)
		isArray    bool
		isPtrArray bool
		rtPtr      reflect2.Type
		rtElem     = rt

		sb       strings.Builder
		stmtArgs []interface{}
		cols     []reflect2.StructField

		item *DataBindingItem
	)

	// Reuse缓存检查
	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey := buildShapeKey(callSite.Key, "Insert", args)
		if i, ok := _dataBindingCache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil && rtElem.Kind() == reflect.Struct {
		// 使用缓存的SQL和字段信息
		// 构建参数
		if isArray {
			slice := rt.(reflect2.SliceType)
			length := slice.UnsafeLengthOf(reflect2.PtrOf(objs))
			for i := 0; i < length; i++ {
				ptr := slice.UnsafeGetIndex(reflect2.PtrOf(objs), i)
				for _, f := range cols {
					var val interface{}
					if isPtrArray {
						val = f.UnsafeGet(ptr)
					} else {
						val = f.UnsafeGet(ptr)
					}
					stmtArgs = append(stmtArgs, val)
				}
			}
		} else {
			for _, f := range cols {
				val := f.UnsafeGet(reflect2.PtrOf(objs))
				stmtArgs = append(stmtArgs, val)
			}
		}

		// 处理额外的args
		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// 构建SQL和字段信息
		item = &DataBindingItem{Type: rtElem}

		sb.WriteString(prefix)
		fieldEscape(&sb, t.Name)
		sb.WriteString(" (")

		switch rt.Kind() {
		case reflect.Ptr:
			rt = rt.(reflect2.PtrType).Elem()
			rtElem = rt
			if rt.Kind() == reflect.Slice {
				rtElem = rtElem.(reflect2.ListType).Elem()
				isArray = true

				if rtElem.Kind() == reflect.Ptr {
					rtPtr = rtElem
					rtElem = rtElem.(reflect2.PtrType).Elem()
					isPtrArray = true
				}
			}
		case reflect.Map:
			// 处理map类型
			mapType := rt.(reflect2.MapType)
			keyType := mapType.Key()

			// 只支持string key的map
			if keyType.Kind() != reflect.String {
				return 0, errors.New("map key must be string type")
			}

			// 使用通用字段收集函数处理map
			var fieldInfos []FieldInfo
			if err := t.collectFieldsGeneric(objs, rt, &sb, &fieldInfos); err != nil {
				return 0, err
			}

			// 构建VALUES部分
			sb.WriteString(") values ")
			sb.WriteString("(")
			for i := range fieldInfos {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString("?")
			}
			sb.WriteString(")")

			// 构建参数
			for _, fieldInfo := range fieldInfos {
				stmtArgs = append(stmtArgs, fieldInfo.GetValue(nil))
			}

			// 处理额外的args
			for _, arg := range args {
				arg.BuildSQL(&sb)
				arg.BuildArgs(&stmtArgs)
			}

			// 执行SQL
			result, err := t.DB.ExecContext(t.ctx, sb.String(), stmtArgs...)
			if err != nil {
				return 0, err
			}

			affected, err := result.RowsAffected()
			if err != nil {
				return 0, err
			}

			return int(affected), nil
		default:
			return 0, errors.New("argument 2 should be map or ptr")
		}

		// Fields or None
		// struct类型
		if rtElem.Kind() != reflect.Struct {
			return 0, errors.New("non-structure type not supported yet")
		}

		s := rtElem.(reflect2.StructType)
		if len(args) > 0 && args[0].Type() == _fields {
			m := t.getStructFieldMap(s)

			for _, field := range args[0].(*fieldsItem).Fields {
				f := m[field]
				if f != nil {
					cols = append(cols, f)
				}
			}

			(args[0]).BuildSQL(&sb)
			args = args[1:]

		} else {
			t.collectFieldsForInsert(s, &sb, &cols)
		}

		sb.WriteString(") values ")

		sbTmp := &sb
		if isArray {
			sbTmp = &strings.Builder{}
		}

		sbTmp.WriteString("(")
		for j := range cols {
			if j > 0 {
				sbTmp.WriteString(",")
			}
			sbTmp.WriteString("?")
		}
		sbTmp.WriteString(")")

		// inputArgs objs
		if isArray {
			// 数组
			for i := 0; i < rt.(reflect2.SliceType).UnsafeLengthOf(reflect2.PtrOf(objs)); i++ {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(sbTmp.String())
				t.inputArgs(&stmtArgs, cols, rtPtr, s, isPtrArray, rt.(reflect2.ListType).UnsafeGetIndex(reflect2.PtrOf(objs), i))
			}
		} else {
			// 普通元素
			t.inputArgs(&stmtArgs, cols, rtPtr, s, false, reflect2.PtrOf(objs))
		}

		// on duplicate key update
		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()
		item.Cols = make([]interface{}, len(cols))
		for i, f := range cols {
			item.Cols[i] = f
		}

		// 存储到缓存
		if t.Cfg.Reuse {
			callSite := getCallSite()
			shapeKey := buildShapeKey(callSite.Key, "Insert", args)
			_dataBindingCache.Store(shapeKey, item)
		}
	}

	if t.Cfg.Debug {
		log.Println(item.SQL, stmtArgs)
	}

	res, err := t.DB.ExecContext(t.ctx, item.SQL, stmtArgs...)
	if err != nil {
		return 0, err
	}

	if !isArray && rtElem.Kind() == reflect.Struct {
		s := rtElem.(reflect2.StructType)
		if f := s.FieldByName("BormLastId"); f != nil {
			id, _ := res.LastInsertId()
			f.UnsafeSet(reflect2.PtrOf(objs), reflect2.PtrOf(id))
		}
	}

	row, _ := res.RowsAffected()
	return int(row), nil
}

// Update .
func (t *BormTable) Update(obj interface{}, args ...BormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Update", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	if len(args) <= 0 {
		return 0, errors.New("argument 2 cannot be omitted")
	}

	var (
		sb       strings.Builder
		stmtArgs []interface{}
		item     *DataBindingItem
	)

	// Reuse缓存检查
	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey := buildShapeKey(callSite.Key, "Update", args)
		if i, ok := _dataBindingCache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil {
		// 使用缓存的SQL
		// 构建参数
		if m, ok := obj.(V); ok {
			// Map类型
			for _, field := range item.Fields {
				v := m[field]
				if v != nil {
					if _, ok := v.(U); ok {
						// U类型不需要参数
					} else {
						stmtArgs = append(stmtArgs, v)
					}
				}
			}
		} else {
			// Struct类型
			rt := reflect2.TypeOf(obj)
			if rt.Kind() != reflect.Ptr {
				return 0, errors.New("update requires pointer to struct")
			}
			rt = rt.(reflect2.PtrType).Elem()
			if rt.Kind() == reflect.Struct {
				s := rt.(reflect2.StructType)
				cols := make([]reflect2.StructField, len(item.Cols))
				for i, f := range item.Cols {
					cols[i] = f.(reflect2.StructField)
				}
				t.inputArgs(&stmtArgs, cols, nil, s, false, reflect2.PtrOf(obj))
			} else {
				return 0, errors.New("non-structure type not supported yet")
			}
		}

		// 处理Where条件
		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// 构建SQL和字段信息
		item = &DataBindingItem{Type: reflect2.TypeOf(obj)}

		sb.WriteString("update ")
		fieldEscape(&sb, t.Name)
		sb.WriteString(" set ")

		// 处理SET部分
		if m, ok := obj.(V); ok {
			// Map类型处理
			if args[0].Type() == _fields {
				argCnt := 0
				for _, field := range args[0].(*fieldsItem).Fields {
					v := m[field]
					if v != nil {
						if argCnt > 0 {
							sb.WriteString(",")
						}
						fieldEscape(&sb, field)
						if s, ok := v.(U); ok {
							sb.WriteString("=")
							sb.WriteString(string(s))
						} else {
							sb.WriteString("=?")
							stmtArgs = append(stmtArgs, v)
						}
						argCnt++
					}
				}
				item.Fields = args[0].(*fieldsItem).Fields
				args = args[1:]
			} else {
				argCnt := 0
				for k, v := range m {
					if argCnt > 0 {
						sb.WriteString(",")
					}
					fieldEscape(&sb, k)
					if s, ok := v.(U); ok {
						sb.WriteString("=")
						sb.WriteString(string(s))
					} else {
						sb.WriteString("=?")
						stmtArgs = append(stmtArgs, v)
					}
					argCnt++
				}
			}
		} else {
			// Struct类型处理
			rt := reflect2.TypeOf(obj)
			if rt.Kind() != reflect.Ptr {
				return 0, errors.New("update requires pointer to struct")
			}
			rt = rt.(reflect2.PtrType).Elem()
			if rt.Kind() == reflect.Struct {
				s := rt.(reflect2.StructType)
				// 如果传入了 Fields(...)，仅更新这些字段，并消费该参数
				if len(args) > 0 && args[0].Type() == _fields {
					m := t.getStructFieldMap(s)
					fields := args[0].(*fieldsItem).Fields
					for i, name := range fields {
						f := m[name]
						if i > 0 {
							sb.WriteString(",")
						}
						fieldEscape(&sb, name)
						sb.WriteString("=?")
						val := f.Get(s.PackEFace(reflect2.PtrOf(obj)))
						if f.Type().String() == "time.Time" {
							if t.Cfg.ToTimestamp {
								val = val.(*time.Time).UTC().Unix()
							} else {
								val = val.(*time.Time).UTC().Format(_timeLayout)
							}
						}
						stmtArgs = append(stmtArgs, val)
					}
					item.Fields = fields
					args = args[1:]
				} else {
					argCnt := 0
					for i := 0; i < s.NumField(); i++ {
						f := s.Field(i)
						ft := f.Tag().Get("borm")
						if !t.Cfg.UseNameWhenTagEmpty && ft == "" {
							continue
						}
						if ft == "-" {
							continue
						}
						if argCnt > 0 {
							sb.WriteString(",")
						}
						if ft == "" {
							fieldEscape(&sb, f.Name())
						} else {
							fieldEscape(&sb, ft)
						}
						sb.WriteString("=?")
						val := f.Get(s.PackEFace(reflect2.PtrOf(obj)))
						if f.Type().String() == "time.Time" {
							if t.Cfg.ToTimestamp {
								val = val.(*time.Time).UTC().Unix()
							} else {
								val = val.(*time.Time).UTC().Format(_timeLayout)
							}
						}
						stmtArgs = append(stmtArgs, val)
						argCnt++
					}
					item.Fields = make([]string, argCnt)
					idx := 0
					for i := 0; i < s.NumField(); i++ {
						f := s.Field(i)
						ft := f.Tag().Get("borm")
						if !t.Cfg.UseNameWhenTagEmpty && ft == "" {
							continue
						}
						if ft == "-" {
							continue
						}
						if ft == "" {
							item.Fields[idx] = f.Name()
						} else {
							item.Fields[idx] = ft
						}
						idx++
					}
				}
			} else {
				return 0, errors.New("non-structure type not supported yet")
			}
		}

		// 处理Where条件
		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()

		// 存储到缓存
		if t.Cfg.Reuse {
			callSite := getCallSite()
			shapeKey := buildShapeKey(callSite.Key, "Update", args)
			_dataBindingCache.Store(shapeKey, item)
		}
	}

	if t.Cfg.Debug {
		log.Println(item.SQL, stmtArgs)
	}

	res, err := t.DB.ExecContext(t.ctx, item.SQL, stmtArgs...)
	if err != nil {
		return 0, err
	}

	row, _ := res.RowsAffected()
	return int(row), nil
}

// Delete .
func (t *BormTable) Delete(args ...BormItem) (int, error) {
	if len(args) <= 0 {
		return 0, errors.New("argument 1 cannot be omitted")
	}

	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Delete", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	var (
		sb       strings.Builder
		stmtArgs []interface{}
		item     *DataBindingItem
	)

	// Reuse缓存检查
	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey := buildShapeKey(callSite.Key, "Delete", args)
		if i, ok := _dataBindingCache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil {
		// 使用缓存的SQL
		// 构建参数
		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// 构建SQL
		item = &DataBindingItem{Type: nil}

		sb.WriteString("delete from ")
		fieldEscape(&sb, t.Name)

		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()

		// 存储到缓存
		if t.Cfg.Reuse {
			callSite := getCallSite()
			shapeKey := buildShapeKey(callSite.Key, "Delete", args)
			_dataBindingCache.Store(shapeKey, item)
		}
	}

	if t.Cfg.Debug {
		log.Println(item.SQL, stmtArgs)
	}

	res, err := t.DB.ExecContext(t.ctx, item.SQL, stmtArgs...)
	if err != nil {
		return 0, err
	}

	row, _ := res.RowsAffected()
	return int(row), nil
}

func (t *BormTable) inputArgs(stmtArgs *[]interface{}, cols []reflect2.StructField, rtPtr, s reflect2.Type, ptr bool, x unsafe.Pointer) {
	for _, col := range cols {
		var v interface{}
		if ptr {
			v = col.Get(rtPtr.UnsafeIndirect(x))
		} else {
			v = col.Get(s.PackEFace(x))
		}

		// 时间类型特殊处理
		if col.Type().String() == "time.Time" {
			if t.Cfg.ToTimestamp {
				v = v.(*time.Time).UTC().Unix()
			} else {
				v = v.(*time.Time).UTC().Format(_timeLayout)
			}
		}

		*stmtArgs = append(*stmtArgs, v)
	}
}

// BormDBIFace .
type BormDBIFace interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// BormTable .
type BormTable struct {
	DB   BormDBIFace
	Name string
	Cfg  Config
	ctx  context.Context

	// 字段映射缓存，避免重复计算
	fieldMapCache sync.Map
}

func fieldEscape(sb *strings.Builder, field string) {
	if field == "" {
		return
	}
	if !strings.ContainsAny(field, ",( `.") {
		// 优化：一次性写入，减少函数调用
		sb.WriteString("`" + field + "`")
	} else {
		// TODO: 处理alias场景
		sb.WriteString(field)
	}
}

func (t *BormTable) getStructFieldMap(s reflect2.StructType) map[string]reflect2.StructField {
	// 使用结构体类型作为缓存key
	typeKey := s.String()

	// 尝试从缓存中获取
	if cached, ok := t.fieldMapCache.Load(typeKey); ok {
		return cached.(map[string]reflect2.StructField)
	}

	// 缓存未命中，构建字段映射
	m := make(map[string]reflect2.StructField)
	t.collectStructFields(s, m, "")

	// 存储到缓存中
	t.fieldMapCache.Store(typeKey, m)
	return m
}

// collectStructFields 递归收集结构体字段，支持embedded struct
func (t *BormTable) collectStructFields(s reflect2.StructType, m map[string]reflect2.StructField, prefix string) {
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("borm")

		// 忽略borm tag为"-"的字段
		if ft == "-" {
			continue
		}

		// 处理embedded struct
		if f.Anonymous() && f.Type().Kind() == reflect.Struct {
			embeddedStruct := f.Type().(reflect2.StructType)
			t.collectStructFields(embeddedStruct, m, prefix)
			continue
		}

		// 处理普通字段
		fieldName := f.Name()
		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		if ft != "" {
			m[ft] = f
		} else if t.Cfg.UseNameWhenTagEmpty {
			m[fieldName] = f
		}
	}
}

// collectFieldsForInsert 收集字段用于INSERT操作，支持embedded struct
func (t *BormTable) collectFieldsForInsert(s reflect2.StructType, sb *strings.Builder, cols *[]reflect2.StructField) {
	t.collectFieldsForInsertWithPrefix(s, sb, cols, "")
}

// collectFieldsForInsertWithPrefix 递归收集字段，支持embedded struct和前缀
func (t *BormTable) collectFieldsForInsertWithPrefix(s reflect2.StructType, sb *strings.Builder, cols *[]reflect2.StructField, prefix string) {
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("borm")

		// 忽略borm tag为"-"的字段
		if ft == "-" {
			continue
		}

		// 处理embedded struct
		if f.Anonymous() && f.Type().Kind() == reflect.Struct {
			embeddedStruct := f.Type().(reflect2.StructType)
			t.collectFieldsForInsertWithPrefix(embeddedStruct, sb, cols, prefix)
			continue
		}

		// 处理普通字段
		if !t.Cfg.UseNameWhenTagEmpty && ft == "" {
			continue
		}

		if len(*cols) > 0 {
			sb.WriteString(",")
		}

		// 确定字段名
		fieldName := f.Name()
		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		if ft == "" {
			fieldEscape(sb, fieldName)
		} else {
			fieldEscape(sb, ft)
		}

		*cols = append(*cols, f)
	}
}

// collectFieldsGeneric 通用字段收集函数，支持struct和map
func (t *BormTable) collectFieldsGeneric(objs interface{}, rt reflect2.Type, sb *strings.Builder, fieldInfos *[]FieldInfo) error {
	switch rt.Kind() {
	case reflect.Struct:
		return t.collectStructFieldsGeneric(rt.(reflect2.StructType), sb, fieldInfos, "")
	case reflect.Map:
		return t.collectMapFieldsGeneric(objs, rt.(reflect2.MapType), sb, fieldInfos)
	default:
		return errors.New("unsupported type for field collection")
	}
}

// collectStructFieldsGeneric 收集struct字段，返回通用FieldInfo
func (t *BormTable) collectStructFieldsGeneric(s reflect2.StructType, sb *strings.Builder, fieldInfos *[]FieldInfo, prefix string) error {
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("borm")

		// 忽略borm tag为"-"的字段
		if ft == "-" {
			continue
		}

		// 处理embedded struct
		if f.Anonymous() && f.Type().Kind() == reflect.Struct {
			embeddedStruct := f.Type().(reflect2.StructType)
			if err := t.collectStructFieldsGeneric(embeddedStruct, sb, fieldInfos, prefix); err != nil {
				return err
			}
			continue
		}

		// 处理普通字段
		if !t.Cfg.UseNameWhenTagEmpty && ft == "" {
			continue
		}

		if len(*fieldInfos) > 0 {
			sb.WriteString(",")
		}

		// 确定字段名
		fieldName := f.Name()
		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		if ft == "" {
			fieldEscape(sb, fieldName)
		} else {
			fieldEscape(sb, ft)
		}

		*fieldInfos = append(*fieldInfos, &StructFieldInfo{field: f})
	}
	return nil
}

// collectMapFieldsGeneric 收集map字段，返回通用FieldInfo
func (t *BormTable) collectMapFieldsGeneric(objs interface{}, mapType reflect2.MapType, sb *strings.Builder, fieldInfos *[]FieldInfo) error {
	// 检查key类型
	keyType := mapType.Key()
	if keyType.Kind() != reflect.String {
		return errors.New("map key must be string type")
	}

	// 使用reflect包获取map的迭代器
	rv := reflect.ValueOf(objs)
	mapIter := rv.MapRange()

	// 用于存储字段信息的临时结构
	type mapFieldData struct {
		key       string
		value     interface{}
		valueType reflect2.Type
	}

	var fieldDataList []mapFieldData

	// 收集map中的所有字段
	for mapIter.Next() {
		key := mapIter.Key()
		value := mapIter.Value()
		keyStr := key.String()
		fieldDataList = append(fieldDataList, mapFieldData{
			key:       keyStr,
			value:     value.Interface(),
			valueType: mapType.Elem(),
		})
	}

	// 按key排序，确保字段顺序一致
	sort.Slice(fieldDataList, func(i, j int) bool {
		return fieldDataList[i].key < fieldDataList[j].key
	})

	// 构建SQL字段部分和FieldInfo
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(sb, fieldData.key)

		*fieldInfos = append(*fieldInfos, &MapFieldInfo{
			key:       fieldData.key,
			value:     fieldData.value,
			valueType: fieldData.valueType,
		})
	}

	return nil
}

// FieldInfo 通用字段信息接口
type FieldInfo interface {
	GetName() string
	GetValue(ptr unsafe.Pointer) interface{}
	GetType() reflect2.Type
}

// StructFieldInfo struct字段信息实现
type StructFieldInfo struct {
	field reflect2.StructField
}

func (f *StructFieldInfo) GetName() string {
	return f.field.Name()
}

func (f *StructFieldInfo) GetValue(ptr unsafe.Pointer) interface{} {
	return f.field.UnsafeGet(ptr)
}

func (f *StructFieldInfo) GetType() reflect2.Type {
	return f.field.Type()
}

// MapFieldInfo map字段信息实现
type MapFieldInfo struct {
	key       string
	value     interface{}
	valueType reflect2.Type
}

func (f *MapFieldInfo) GetName() string {
	return f.key
}

func (f *MapFieldInfo) GetValue(ptr unsafe.Pointer) interface{} {
	return f.value
}

func (f *MapFieldInfo) GetType() reflect2.Type {
	return f.valueType
}

// BormItem .
type BormItem interface {
	Type() int
	BuildSQL(*strings.Builder)
	BuildArgs(*[]interface{})
}

type fieldsItem struct {
	Fields []string
}

func (w *fieldsItem) Type() int {
	return _fields
}

func (w *fieldsItem) BuildSQL(sb *strings.Builder) {
	for i, field := range w.Fields {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(sb, field)
	}
}

func (w *fieldsItem) BuildArgs(stmtArgs *[]interface{}) {
}

type onConflictDoUpdateSetItem struct {
	Conds string
	Vals  []interface{}
}

func (w *onConflictDoUpdateSetItem) Type() int {
	return _onConflictDoUpdateSet
}

func (w *onConflictDoUpdateSetItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(w.Conds)
}

func (w *onConflictDoUpdateSetItem) BuildArgs(stmtArgs *[]interface{}) {
	*stmtArgs = append(*stmtArgs, w.Vals...)
}

type joinItem struct {
	Stmt string
}

func (w *joinItem) Type() int {
	return _join
}

func (w *joinItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(" ")
	sb.WriteString(w.Stmt)
}

func (w *joinItem) BuildArgs(stmtArgs *[]interface{}) {
}

type indexedByItem struct {
	idx string
}

func (w *indexedByItem) Type() int {
	return _indexedBy
}

func (w *indexedByItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(" indexed by " + w.idx)
}

func (w *indexedByItem) BuildArgs(stmtArgs *[]interface{}) {
	return
}

type whereItem struct {
	Conds []interface{}
}

func (w *whereItem) Type() int {
	return _where
}

func (w *whereItem) BuildSQL(sb *strings.Builder) {
	if len(w.Conds) <= 0 {
		return
	}
	sb.WriteString(" where ")
	for i, c := range w.Conds {
		if i > 0 {
			sb.WriteString(" and ")
		}
		if cond, ok := c.(*ormCond); ok {
			cond.BuildSQL(sb)
		} else if condEx, ok := c.(*ormCondEx); ok {
			if condEx.Ty > _andCondEx && len(condEx.Conds) > 1 && len(w.Conds) > 1 {
				sb.WriteString("(")
			}
			condEx.BuildSQL(sb)
			if condEx.Ty > _andCondEx && len(condEx.Conds) > 1 && len(w.Conds) > 1 {
				sb.WriteString(")")
			}
		}
	}
}

func (w *whereItem) BuildArgs(stmtArgs *[]interface{}) {
	for _, c := range w.Conds {
		if cond, ok := c.(*ormCond); ok {
			*stmtArgs = append(*stmtArgs, cond.Args...)
		} else if condEx, ok := c.(*ormCondEx); ok {
			condEx.BuildArgs(stmtArgs)
		}
	}
}

type groupByItem struct {
	Fields []string
}

func (w *groupByItem) Type() int {
	return _groupBy
}

func (w *groupByItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(" group by ")

	for i, field := range w.Fields {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(sb, field)
	}
}

func (w *groupByItem) BuildArgs(stmtArgs *[]interface{}) {
}

type havingItem struct {
	Conds []interface{}
}

func (h *havingItem) Type() int {
	return _having
}

func (h *havingItem) BuildSQL(sb *strings.Builder) {
	if len(h.Conds) <= 0 {
		return
	}
	sb.WriteString(" having ")
	for i, c := range h.Conds {
		if i > 0 {
			sb.WriteString(" and ")
		}
		if cond, ok := c.(*ormCond); ok {
			cond.BuildSQL(sb)
		} else if condEx, ok := c.(*ormCondEx); ok {
			if condEx.Ty > _andCondEx && len(condEx.Conds) > 1 && len(h.Conds) > 1 {
				sb.WriteString("(")
			}
			condEx.BuildSQL(sb)
			if condEx.Ty > _andCondEx && len(condEx.Conds) > 1 && len(h.Conds) > 1 {
				sb.WriteString(")")
			}
		}
	}
}

func (h *havingItem) BuildArgs(stmtArgs *[]interface{}) {
	for _, c := range h.Conds {
		if cond, ok := c.(*ormCond); ok {
			*stmtArgs = append(*stmtArgs, cond.Args...)
		} else if condEx, ok := c.(*ormCondEx); ok {
			condEx.BuildArgs(stmtArgs)
		}
	}
}

type orderByItem struct {
	Orders []string
}

func (o *orderByItem) Type() int {
	return _orderBy
}

func (o *orderByItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(" order by ")

	for i, order := range o.Orders {
		if i > 0 {
			sb.WriteString(",")
		}
		// TODO: 带升降序关键词的字段转义
		fieldEscape(sb, order)
	}
}

func (o *orderByItem) BuildArgs(stmtArgs *[]interface{}) {
}

type limitItem struct {
	I []interface{}
}

func (l *limitItem) Type() int {
	return _limit
}

func (l *limitItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(" limit ?")
	if len(l.I) > 1 {
		sb.WriteString(" offset ?")
	}
}

func (l *limitItem) BuildArgs(stmtArgs *[]interface{}) {
	*stmtArgs = append(*stmtArgs, l.I...)
}

func strconvErr(err error) error {
	if ne, ok := err.(*strconv.NumError); ok {
		return ne.Err
	}
	return err
}

type scanner struct {
	Type reflect2.Type
	Val  unsafe.Pointer
}

func numberToString(k reflect.Kind, src interface{}) string {
	switch k {
	case reflect.Bool:
		if src.(bool) {
			return "true"
		}
		return "false"
	case reflect.Int64:
		return fmt.Sprintf("%d", src.(int64))
	case reflect.Float64:
		return fmt.Sprintf("%g", src.(float64))
	}
	return ""
}

func toUnix(year, month, day, hour, min, sec int) int64 {
	if month < 1 || month > 12 {
		return -1
	}

	leap := 0
	if (year%4 == 0 && (year%100 != 0 || year%400 == 0)) && month >= 3 {
		leap = 1 // February 29
	}

	return int64((365*year-719528+day-1+(year+3)/4-(year+99)/100+(year+399)/400+int([]int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}[month-1])+leap)*86400 + hour*3600 + min*60 + sec)
}

// parseTimeString 优化的时间字符串解析函数
func parseTimeString(s string) (time.Time, error) {
	// 处理空字符串或NULL值
	if s == "" || s == "NULL" || s == "null" {
		return time.Time{}, nil
	}

	// 快速检查字符串长度和特征
	if len(s) < 10 {
		return time.Time{}, fmt.Errorf("time string too short")
	}

	// 检查是否包含时区信息
	hasTimezone := false
	hasNano := false

	// 检查时区标识符
	if s[len(s)-1] == 'Z' ||
		(len(s) >= 6 && (s[len(s)-6:] == "+00:00" || s[len(s)-6:] == "-00:00")) ||
		(len(s) >= 5 && (s[len(s)-5:] == "+0000" || s[len(s)-5:] == "-0000")) {
		hasTimezone = true
	} else if len(s) >= 6 {
		// 检查是否有 +/- 时区标识
		for i := len(s) - 6; i < len(s); i++ {
			if s[i] == '+' || s[i] == '-' {
				hasTimezone = true
				break
			}
		}
	}

	// 特殊处理：纯日期格式（如 2019-03-01）不应该被当作有时区处理
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		hasTimezone = false
	}

	// 检查是否有毫秒/微秒
	if len(s) > 19 && s[19] == '.' {
		hasNano = true
	}

	// 根据特征选择解析格式
	var layout string
	if hasTimezone {
		if hasNano {
			// 带时区和毫秒的格式
			if len(s) > 10 && s[10] == 'T' {
				layout = time.RFC3339Nano // 2006-01-02T15:04:05.999999999Z07:00
			} else {
				layout = _timeLayoutWithNanoTZ
			}
		} else {
			// 带时区但无毫秒的格式
			if len(s) > 10 && s[10] == 'T' {
				layout = time.RFC3339 // 2006-01-02T15:04:05Z07:00
			} else if s[len(s)-1] == 'Z' {
				layout = _timeLayoutWithZ
			} else {
				layout = _timeLayoutWithTZ
			}
		}
	} else {
		// 无时区信息，根据长度选择格式
		if len(s) == 10 && s[4] == '-' && s[7] == '-' {
			// 纯日期格式
			layout = "2006-01-02"
		} else {
			// 标准日期时间格式
			layout = "2006-01-02 15:04:05"
		}
	}

	return time.Parse(layout, s)
}

func scanFromString(isTime bool, st reflect2.Type, dt reflect2.Type, ptrVal unsafe.Pointer, tmp string) error {
	dk := dt.Kind()

	// 时间格式(DATE/DATETIME) => number/time.Time
	if isTime || (dk >= reflect.Int && dk <= reflect.Float64) {
		// 优化的时间解析：先分析字符串特征，再选择对应的解析方法
		if isTime {
			parsedTime, err := parseTimeString(tmp)
			if err == nil {
				*(*time.Time)(ptrVal) = parsedTime.UTC()
				return nil
			}
		}

		// 如果带时区解析失败，尝试原有的简单格式解析
		var year, month, day, hour, min, sec int
		n, _ := fmt.Sscanf(tmp, "%4d-%2d-%2d %2d:%2d:%2d", &year, &month, &day, &hour, &min, &sec)
		if n == 3 || n == 6 {
			if isTime {
				*(*time.Time)(ptrVal) = time.Unix(toUnix(year, month, day, hour, min, sec), 0).UTC()
				return nil
			}
			ts := toUnix(year, month, day, hour, min, sec)
			switch dk {
			case reflect.Int:
				*(*int)(ptrVal) = int(ts)
			case reflect.Int8:
				*(*int8)(ptrVal) = int8(ts)
			case reflect.Int16:
				*(*int16)(ptrVal) = int16(ts)
			case reflect.Int32:
				*(*int32)(ptrVal) = int32(ts)
			case reflect.Int64:
				*(*int64)(ptrVal) = ts
			case reflect.Uint:
				*(*uint)(ptrVal) = uint(ts)
			case reflect.Uint8:
				*(*uint8)(ptrVal) = uint8(ts)
			case reflect.Uint16:
				*(*uint16)(ptrVal) = uint16(ts)
			case reflect.Uint32:
				*(*uint32)(ptrVal) = uint32(ts)
			case reflect.Uint64:
				*(*uint64)(ptrVal) = uint64(ts)
			case reflect.Float32:
				*(*float32)(ptrVal) = float32(ts)
			case reflect.Float64:
				*(*float64)(ptrVal) = float64(ts)
			}
			return nil
		} else if isTime {
			// 获取数值时间戳
			i64, err := strconv.ParseInt(tmp, 10, 64)
			if err != nil {
				return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
			}
			*(*time.Time)(ptrVal) = time.Unix(i64, 0).UTC()
			return nil
		}
	}

	// 非时间格式
	switch dk {
	case reflect.Bool:
		*(*bool)(ptrVal) = (tmp == "true")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i64, err := strconv.ParseInt(tmp, 10, dt.Type1().Bits())
		if err != nil {
			return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
		}
		switch dk {
		case reflect.Int64:
			*(*int64)(ptrVal) = i64
		case reflect.Int32:
			*(*int32)(ptrVal) = int32(i64)
		case reflect.Int16:
			*(*int16)(ptrVal) = int16(i64)
		case reflect.Int8:
			*(*int8)(ptrVal) = int8(i64)
		case reflect.Int:
			*(*int)(ptrVal) = int(i64)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u64, err := strconv.ParseUint(tmp, 10, dt.Type1().Bits())
		if err != nil {
			return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
		}
		switch dk {
		case reflect.Uint64:
			*(*uint64)(ptrVal) = u64
		case reflect.Uint32:
			*(*uint32)(ptrVal) = uint32(u64)
		case reflect.Uint16:
			*(*uint16)(ptrVal) = uint16(u64)
		case reflect.Uint8:
			*(*uint8)(ptrVal) = uint8(u64)
		case reflect.Uint:
			*(*uint)(ptrVal) = uint(u64)
		}
	case reflect.Float32, reflect.Float64:
		f64, err := strconv.ParseFloat(tmp, dt.Type1().Bits())
		if err != nil {
			return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
		}
		if dk == reflect.Float32 {
			*(*float32)(ptrVal) = float32(f64)
		} else {
			*(*float64)(ptrVal) = f64
		}
	case reflect.String:
		*(*string)(ptrVal) = tmp
	default:
		// string => []byte
		if dk == reflect.Slice && dt.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
			*(*[]byte)(ptrVal) = reflect2.UnsafeCastString(tmp)
			return nil
		}
		// TODO 自定义类型，尝试转换
		return fmt.Errorf("converting driver.Value type %s (%s) to a %s", st.String(), tmp, dt.String())
	}
	return nil
}

func (dest *scanner) Scan(src interface{}) error {
	var (
		st = reflect2.TypeOf(src)
		dt = dest.Type
	)

	// NULL值
	if src == nil || st.UnsafeIsNil(reflect2.PtrOf(src)) {
		// 设置成默认值，如果是指针，那么是空指针
		dt.UnsafeSet(dest.Val, dt.UnsafeNew())
		return nil
	}

	var (
		sk = st.Kind()
		dk = dt.Kind()
	)

	// 相同类型，直接赋值
	if dk == sk {
		dt.UnsafeSet(dest.Val, reflect2.PtrOf(src))
		return nil
	}

	isTime := dt.String() == "time.Time"
	// int64 => time.Time
	if sk == reflect.Int64 && isTime {
		*(*time.Time)(dest.Val) = time.Unix(src.(int64), 0).UTC()
		return nil
	}

	if sk == reflect.String {
		return scanFromString(isTime, st, dt, dest.Val, src.(string))
	} else if sk == reflect.Slice && st.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
		return scanFromString(isTime, st, dt, dest.Val, string(src.([]byte)))
	} else if st.String() == "time.Time" {
		if dk == reflect.String {
			return dest.Scan(src.(time.Time).UTC().Format(_timeLayout))
		}
		return dest.Scan(src.(time.Time).UTC().Unix())
	}

	switch dk {
	case reflect.Bool:
		switch sk {
		case reflect.Int64:
			*(*bool)(dest.Val) = (src.(int64) != 0)
		case reflect.Float64:
			*(*bool)(dest.Val) = (src.(float64) != 0)
		}
	case reflect.Int64:
		switch sk {
		case reflect.Bool:
			if src.(bool) {
				*(*int64)(dest.Val) = int64(1)
			} else {
				*(*int64)(dest.Val) = int64(0)
			}
		case reflect.Float64:
			*(*int64)(dest.Val) = int64(src.(float64))
		}
	case reflect.Float64:
		switch sk {
		case reflect.Bool:
			if src.(bool) {
				*(*float64)(dest.Val) = float64(1)
			} else {
				*(*float64)(dest.Val) = float64(0)
			}
		case reflect.Int64:
			*(*float64)(dest.Val) = float64(src.(int64))
		}
	case reflect.String:
		*(*string)(dest.Val) = numberToString(sk, src)
	default:
		// number => []byte
		if dk == reflect.Slice && dt.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
			*(*[]byte)(dest.Val) = reflect2.UnsafeCastString(numberToString(sk, src))
			return nil
		}
		return scanFromString(isTime, st, dt, dest.Val, fmt.Sprint(src))
	}
	return nil
}

type ormCond struct {
	Field string
	Op    string
	Args  []interface{}
}

func (c *ormCond) Type() int {
	return _cond
}

func (c *ormCond) BuildSQL(sb *strings.Builder) {
	if c.Field != "" {
		fieldEscape(sb, c.Field)
	}
	sb.WriteString(c.Op)
}

func (c *ormCond) BuildArgs(stmtArgs *[]interface{}) {
	*stmtArgs = append(*stmtArgs, c.Args...)
}

type ormCondEx struct {
	Ty    int
	Conds []interface{}
}

func (cx *ormCondEx) Type() int {
	return cx.Ty
}

func (cx *ormCondEx) BuildSQL(sb *strings.Builder) {
	for i, c := range cx.Conds {
		if i > 0 {
			switch cx.Ty {
			case _andCondEx:
				sb.WriteString(" and ")
			case _orCondEx:
				sb.WriteString(" or ")
			}
		}
		if cond, ok := c.(*ormCond); ok {
			cond.BuildSQL(sb)
		} else if condEx, ok := c.(*ormCondEx); ok {
			if len(condEx.Conds) > 1 && len(cx.Conds) > 1 {
				sb.WriteString("(")
			}
			condEx.BuildSQL(sb)
			if len(condEx.Conds) > 1 && len(cx.Conds) > 1 {
				sb.WriteString(")")
			}
		}
	}
}

func (cx *ormCondEx) BuildArgs(stmtArgs *[]interface{}) {
	for _, c := range cx.Conds {
		if cond, ok := c.(*ormCond); ok {
			*stmtArgs = append(*stmtArgs, cond.Args...)
		} else if condEx, ok := c.(*ormCondEx); ok {
			condEx.BuildArgs(stmtArgs)
		}
	}
}

/*
   条件逻辑运算
*/

// And .
func And(conds ...interface{}) *ormCondEx {
	return &ormCondEx{Ty: _andCondEx, Conds: conds}
}

// Or .
func Or(conds ...interface{}) *ormCondEx {
	return &ormCondEx{Ty: _orCondEx, Conds: conds}
}

// Cond .
func Cond(c string, args ...interface{}) *ormCond {
	return &ormCond{Op: c, Args: args}
}

// Eq .
func Eq(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: "=?", Args: []interface{}{i}}
}

// Neq .
func Neq(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: "<>?", Args: []interface{}{i}}
}

// Gt .
func Gt(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: ">?", Args: []interface{}{i}}
}

// Gte .
func Gte(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: ">=?", Args: []interface{}{i}}
}

// Lt .
func Lt(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: "<?", Args: []interface{}{i}}
}

// Lte .
func Lte(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: "<=?", Args: []interface{}{i}}
}

// Between .
func Between(field string, i interface{}, j interface{}) *ormCond {
	return &ormCond{Field: field, Op: " between ? and ?", Args: []interface{}{i, j}}
}

// Like .
func Like(field string, pattern string) *ormCond {
	return &ormCond{Field: field, Op: " like ?", Args: []interface{}{pattern}}
}

// GLOB .
func GLOB(field string, pattern string) *ormCond {
	return &ormCond{Field: field, Op: " glob ?", Args: []interface{}{pattern}}
}

// In .
func In(field string, args ...interface{}) *ormCond {
RETRY:
	switch len(args) {
	case 0:
		return &ormCond{Op: "1=1"}
	case 1:
		rt := reflect2.TypeOf(args[0])
		// 如果第一个参数是数组，转化成interface数组
		if rt.Kind() == reflect.Slice {
			len := rt.(reflect2.SliceType).UnsafeLengthOf(reflect2.PtrOf(args[0]))
			argsAux := make([]interface{}, len)
			rtElem := rt.(reflect2.ListType).Elem()
			for i := 0; i < len; i++ {
				argsAux[i] = rtElem.PackEFace(rt.(reflect2.ListType).UnsafeGetIndex(reflect2.PtrOf(args[0]), i))
			}
			args = argsAux
			goto RETRY
		} else {
			// 单条不用in，用等于
			return Eq(field, args[0])
		}
	}

	var sb strings.Builder
	sb.WriteString(" in (")
	for i := 0; i < len(args); i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("?")
	}
	sb.WriteString(")")
	return &ormCond{Field: field, Op: sb.String(), Args: args}
}

/*
	data-binding相关
*/

// DataBindingItem .

type DataBindingItem struct {
	SQL    string
	Cols   []interface{}
	Type   reflect2.Type
	Elem   interface{}
	Fields []string // 用于Map类型的字段名
}

var _dataBindingCache sync.Map

/*
Mock相关
*/
var (
	_mockData []*MockMatcher
	_mutex    sync.Mutex
)

func matchString(src string, matcher string, caseSens bool) bool {
	if matcher == "" {
		return true
	}

	isAlpha := func(x byte) bool {
		return x >= 0x61 && x <= 0x7A
	}

	caseSensEq := func(x byte, y byte, cs bool) bool {
		return (y == '?' || x == y || (!cs && isAlpha(x|0x20) && (x^y) == 0x20))
	}

	s := make([][]int, 0)
	i := 0
	j := 0
	ml := len(matcher)
	// scan from start
	for i < len(src) {
		// match asterisk
		if j < ml && matcher[j] == '*' {
			// skip continuous asterisks
			for j < ml && matcher[j] == '*' {
				j++
			}
			// forward to first match char of src
			for i < len(src) && (j >= ml || !caseSensEq(src[i], matcher[j], caseSens)) {
				i++
			}
			// record current position for back-track
			s = append(s, []int{i + 1, j - 1})
		} else if j < ml && caseSensEq(src[i], matcher[j], caseSens) {
			// eat one character
			i++
			j++
		} else {
			// hit mismatch, then back-track
			if len(s) <= 0 {
				return false
			}
			i, j = s[0][0], s[0][1]
			s = s[1:]
		}
	}
	// ignore ending asterisks
	for j < ml && matcher[j] == '*' {
		j++
	}
	return i == len(src) && j == ml
}

// MockMatcher .
type MockMatcher struct {
	Tbl    string
	Func   string
	Caller string
	File   string
	Pkg    string
	Data   interface{}
	Ret    int
	Err    error
}

func checkMock(tbl, fun, caller, file, pkg string) (mocked bool, data interface{}, ret int, err error) {
	_mutex.Lock()
	defer _mutex.Unlock()

	for i := 0; i < len(_mockData); i++ {
		data := _mockData[i]
		if matchString(tbl, data.Tbl, false) &&
			matchString(fun, data.Func, false) &&
			matchString(caller, data.Caller, false) &&
			matchString(file, data.File, false) &&
			matchString(pkg, data.Pkg, false) {
			_mockData = append(_mockData[0:i], _mockData[i+1:]...)
			return true, data.Data, data.Ret, data.Err
		}
	}
	return false, nil, 0, nil
}

func checkInTestFile(fileName string) {
	if !strings.HasSuffix(fileName, "_test.go") {
		panic("DONT USE THIS FUNCTION IN PRODUCTION ENVIRONMENT!")
	}
}

// BormMock .
func BormMock(tbl, fun, caller, file, pkg string, data interface{}, ret int, err error) {
	_, fileName, _, _ := runtime.Caller(1)
	checkInTestFile(fileName)

	config.Mock = true

	_mutex.Lock()
	defer _mutex.Unlock()

	_mockData = append(_mockData, &MockMatcher{
		Tbl:    tbl,
		Func:   fun,
		Caller: caller,
		File:   file,
		Pkg:    pkg,
		Data:   data,
		Ret:    ret,
		Err:    err,
	})
	return
}

// BormMockFinish .
func BormMockFinish() error {
	_mutex.Lock()
	defer _mutex.Unlock()

	mockData := _mockData
	_mockData = make([]*MockMatcher, 0)
	if len(mockData) > 0 {
		return fmt.Errorf("Some of the mock data left behind: %+v", mockData)
	}
	return nil
}

// 优化后的缓存操作函数
func buildCacheKey(file string, line int) string {
	builder := _cacheKeyPool.Get().(*strings.Builder)
	builder.Reset()
	builder.WriteString(file)
	builder.WriteString(":")
	builder.WriteString(fmt.Sprintf("%d", line))
	key := builder.String()
	_cacheKeyPool.Put(builder)
	return key
}

func getCallSite() *CallSite {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc) // 跳过getCallSite和调用者
	callerPC := pc[0]

	if cached, ok := _callSiteCache.Load(callerPC); ok {
		return cached.(*CallSite)
	}

	_, file, line, _ := runtime.Caller(1)
	key := buildCacheKey(file, line)
	callSite := &CallSite{
		File: file,
		Line: line,
		Key:  key,
	}
	_callSiteCache.Store(callerPC, callSite)
	return callSite
}

func storeToCache(file string, line int, item *DataBindingItem) {
	key := buildCacheKey(file, line)
	_dataBindingCache.Store(key, item)
}

func loadFromCache(file string, line int) *DataBindingItem {
	key := buildCacheKey(file, line)
	if i, ok := _dataBindingCache.Load(key); ok {
		return i.(*DataBindingItem)
	}
	return nil
}

// 优化版本：使用预缓存的调用位置
func storeToCacheOptimized(callSite *CallSite, item *DataBindingItem) {
	_dataBindingCache.Store(callSite.Key, item)
}

func loadFromCacheOptimized(callSite *CallSite) *DataBindingItem {
	if i, ok := _dataBindingCache.Load(callSite.Key); ok {
		return i.(*DataBindingItem)
	}
	return nil
}

var (
	_callSiteCache sync.Map // map[uintptr]*CallSite
	_cacheKeyPool  = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}
)

type CallSite struct {
	File string
	Line int
	Key  string
}
