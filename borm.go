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
	_where
	_groupBy
	_having
	_orderBy
	_limit
	_onDuplicateKeyUpdate
	_forceIndex

	_andCondEx = iota
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
func Table(db BormDBIFace, name string) *BormTable {
	return &BormTable{
		DB:   db,
		Name: name,
		ctx:  context.Background(),
		Cfg:  Config{Reuse: true}, // 默认开启Reuse功能
	}
}

// TableContext 创建带Context的Table，参数顺序：context, db, name
func TableContext(ctx context.Context, db BormDBIFace, name string) *BormTable {
	return &BormTable{
		DB:   db,
		Name: name,
		ctx:  ctx,
		Cfg:  Config{Reuse: true}, // 默认开启Reuse功能
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

// SafeReuse 已合并进 Reuse，保持兼容
func (t *BormTable) SafeReuse() *BormTable { return t.Reuse() }

// NoSafeReuse 已合并进 Reuse，保持兼容
func (t *BormTable) NoSafeReuse() *BormTable { return t }

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

// OnDuplicateKeyUpdate .
func OnDuplicateKeyUpdate(keyVals V) *onDuplicateKeyUpdateItem {
	res := &onDuplicateKeyUpdateItem{}
	if len(keyVals) <= 0 {
		return res
	}

	var sb strings.Builder
	sb.WriteString(" on duplicate key update ")
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

// ForceIndex .
func ForceIndex(idx string) *forceIndexItem {
	return &forceIndexItem{idx: idx}
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
		isMap      bool

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
		if rtElem.Kind() == reflect.Map {
			isMap = true
		}
	default:
		return 0, errors.New("argument 2 should be map or ptr")
	}

	// Map类型选择：要求显式Fields，且不走复用缓存
	if isMap {
		if len(args) <= 0 || args[0].Type() != _fields {
			return 0, errors.New("map select requires Fields(\"col\", ...) explicitly")
		}
		fi := args[0].(*fieldsItem)
		if len(fi.Fields) < 1 {
			return 0, errors.New("too few fields")
		}

		var sb strings.Builder
		sb.WriteString("select ")
		fi.BuildSQL(&sb)
		sb.WriteString(" from ")
		fieldEscape(&sb, t.Name)
		var stmtArgs []interface{}
		for _, arg := range args[1:] {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		sqlStr := sb.String()
		if t.Cfg.Debug {
			log.Println(sqlStr, stmtArgs)
		}

		// 构建scan目标
		buildScanDests := func(n int) ([]interface{}, []interface{}) {
			vals := make([]interface{}, n)
			dests := make([]interface{}, n)
			for i := 0; i < n; i++ {
				dests[i] = &vals[i]
			}
			return dests, vals
		}

		if !isArray {
			dests, vals := buildScanDests(len(fi.Fields))
			err := t.DB.QueryRowContext(t.ctx, sqlStr, stmtArgs...).Scan(dests...)
			if err != nil {
				if err == sql.ErrNoRows {
					return 0, nil
				}
				return 0, err
			}
			m := make(map[string]interface{}, len(fi.Fields))
			for i, name := range fi.Fields {
				val := vals[i]
				// 将[]byte转换为string
				if b, ok := val.([]byte); ok {
					m[name] = string(b)
				} else {
					m[name] = val
				}
			}
			// 设置到 *map[string]interface{}
			reflect.ValueOf(res).Elem().Set(reflect.ValueOf(m))
			return 1, nil
		}

		rows, err := t.DB.QueryContext(t.ctx, sqlStr, stmtArgs...)
		if err != nil {
			return 0, err
		}
		defer rows.Close()

		sliceVal := reflect.ValueOf(res).Elem()
		count := 0
		for rows.Next() {
			dests, vals := buildScanDests(len(fi.Fields))
			if err := rows.Scan(dests...); err != nil {
				return 0, err
			}
			m := make(map[string]interface{}, len(fi.Fields))
			for i, name := range fi.Fields {
				val := vals[i]
				// 将[]byte转换为string
				if b, ok := val.([]byte); ok {
					m[name] = string(b)
				} else {
					m[name] = val
				}
			}
			sliceVal.Set(reflect.Append(sliceVal, reflect.ValueOf(m)))
			count++
		}
		return count, rows.Err()
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
			// map类型
			// } else if rt.Kind() == reflect.Map {
			// TODO
			// 其他类型
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

					if ft == "-" {
						continue
					}

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
			// map类型
			// } else if rt.Kind() == reflect.Map {
			// TODO
			// 其他类型
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
		err := t.DB.QueryRowContext(t.ctx, item.SQL, stmtArgs...).Scan(item.Cols...)
		if err != nil {
			if err == sql.ErrNoRows {
				return 0, nil
			}
			return 0, err
		}
		return 1, err
	}

	// fire
	rows, err := t.DB.QueryContext(t.ctx, item.SQL, stmtArgs...)
	if err != nil {
		return 0, err
	}

	count := 0
	for rows.Next() {
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

	// 检查是否为V类型（map[string]interface{}）
	if m, ok := objs.(V); ok {
		return t.insertMapWithPrefix("insert ignore into ", m, args...)
	}

	// 检查是否为通用map类型
	rt := reflect2.TypeOf(objs)
	if rt.Kind() == reflect.Map {
		mapType := rt.(reflect2.MapType)
		if mapType.Key().Kind() != reflect.String {
			return 0, errors.New("map key must be string type")
		}
		return t.insertGenericMapWithPrefix("insert ignore into ", objs, mapType, args...)
	}

	// 处理结构体类型
	return t.insertStructWithPrefix("insert ignore into ", objs, args...)
}

// ReplaceInto .
func (t *BormTable) ReplaceInto(objs interface{}, args ...BormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "ReplaceInto", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	// 检查是否为V类型（map[string]interface{}）
	if m, ok := objs.(V); ok {
		return t.insertMapWithPrefix("replace into ", m, args...)
	}

	// 检查是否为通用map类型
	rt := reflect2.TypeOf(objs)
	if rt.Kind() == reflect.Map {
		mapType := rt.(reflect2.MapType)
		if mapType.Key().Kind() != reflect.String {
			return 0, errors.New("map key must be string type")
		}
		return t.insertGenericMapWithPrefix("replace into ", objs, mapType, args...)
	}

	// 处理结构体类型
	return t.insertStructWithPrefix("replace into ", objs, args...)
}

// Insert .
func (t *BormTable) Insert(objs interface{}, args ...BormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Insert", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	// 检查是否为V类型（map[string]interface{}）
	if m, ok := objs.(V); ok {
		return t.insertMap(m, args...)
	}

	// 检查是否为通用map类型
	rt := reflect2.TypeOf(objs)
	if rt.Kind() == reflect.Map {
		mapType := rt.(reflect2.MapType)
		if mapType.Key().Kind() != reflect.String {
			return 0, errors.New("map key must be string type")
		}
		return t.insertGenericMap(objs, mapType, args...)
	}

	// 处理结构体类型
	return t.insertStruct(objs, args...)
}

// insertMap 处理V类型（map[string]interface{}）的插入
func (t *BormTable) insertMap(m V, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString("insert into ")
	fieldEscape(&sb, t.Name)
	sb.WriteString(" (")

	// 检查是否有Fields参数
	hasFields := len(args) > 0 && args[0].Type() == _fields
	var fieldsToProcess []string

	if hasFields {
		fi := args[0].(*fieldsItem)
		fieldsToProcess = fi.Fields
		args = args[1:] // 移除Fields参数
	} else {
		// 处理所有map字段
		for k, v := range m {
			if v != nil {
				fieldsToProcess = append(fieldsToProcess, k)
			}
		}
		// 检查空map
		if len(fieldsToProcess) == 0 {
			return 0, errors.New("empty map: no fields to insert")
		}
	}

	// 构建字段列表
	for i, field := range fieldsToProcess {
		v := m[field]
		if v != nil {
			if i > 0 {
				sb.WriteString(",")
			}
			fieldEscape(&sb, field)
		}
	}

	sb.WriteString(") values (")

	// 构建VALUES部分
	for i, field := range fieldsToProcess {
		v := m[field]
		if v != nil {
			if i > 0 {
				sb.WriteString(",")
			}
			if s, ok := v.(U); ok {
				sb.WriteString(string(s))
			} else {
				sb.WriteString("?")
				stmtArgs = append(stmtArgs, v)
			}
		}
	}
	sb.WriteString(")")

	// 构建其他条件
	for _, arg := range args {
		arg.BuildSQL(&sb)
		arg.BuildArgs(&stmtArgs)
	}

	sqlStr := sb.String()
	if t.Cfg.Debug {
		log.Printf("%s %v", sqlStr, stmtArgs)
	}

	result, err := t.DB.ExecContext(t.ctx, sqlStr, stmtArgs...)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	return int(affected), err
}

// insertGenericMap 处理通用map类型的插入
func (t *BormTable) insertGenericMap(obj interface{}, mapType reflect2.MapType, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString("insert into ")
	fieldEscape(&sb, t.Name)
	sb.WriteString(" (")

	// 使用reflect包获取map的迭代器
	rv := reflect.ValueOf(obj)
	mapIter := rv.MapRange()

	// 用于存储字段信息的临时结构
	type mapFieldData struct {
		key   string
		value interface{}
	}

	var fieldDataList []mapFieldData

	// 收集map中的所有字段
	for mapIter.Next() {
		key := mapIter.Key()
		value := mapIter.Value()
		keyStr := key.String()
		fieldDataList = append(fieldDataList, mapFieldData{
			key:   keyStr,
			value: value.Interface(),
		})
	}

	// 按key排序，确保字段顺序一致
	sort.Slice(fieldDataList, func(i, j int) bool {
		return fieldDataList[i].key < fieldDataList[j].key
	})

	// 构建字段列表
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, fieldData.key)
	}
	sb.WriteString(") values (")

	// 构建VALUES部分
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("?")
		stmtArgs = append(stmtArgs, fieldData.value)
	}
	sb.WriteString(")")

	// 构建其他条件
	for _, arg := range args {
		arg.BuildSQL(&sb)
		arg.BuildArgs(&stmtArgs)
	}

	sqlStr := sb.String()
	if t.Cfg.Debug {
		log.Printf("%s %v", sqlStr, stmtArgs)
	}

	result, err := t.DB.ExecContext(t.ctx, sqlStr, stmtArgs...)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	return int(affected), err
}

// insertStruct 处理结构体类型的插入
func (t *BormTable) insertStruct(objs interface{}, args ...BormItem) (int, error) {
	var (
		item     *DataBindingItem
		stmtArgs []interface{}
	)

	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey := buildShapeKey(callSite.Key, "Insert", args)
		if i, ok := _dataBindingCache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil {
		// 使用缓存的SQL，但需要重新构建参数
		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// 构建新的SQL
		item = &DataBindingItem{}
		var sb strings.Builder
		sb.WriteString("insert into ")
		fieldEscape(&sb, t.Name)

		rt := reflect2.TypeOf(objs)
		var isArray bool
		var isPtrArray bool
		var rtPtr reflect2.Type
		switch rt.Kind() {
		case reflect.Ptr:
			rt = rt.(reflect2.PtrType).Elem()
			if rt.Kind() == reflect.Slice {
				isArray = true
				rtElem := rt.(reflect2.SliceType).Elem()
				if rtElem.Kind() == reflect.Ptr {
					rtPtr = rtElem
					rt = rtElem.(reflect2.PtrType).Elem()
					isPtrArray = true
				} else {
					rt = rtElem
				}
			}
		default:
			return 0, errors.New("argument 2 should be map or ptr")
		}

		var cols []reflect2.StructField

		// Fields or None
		// struct类型
		if rt.Kind() != reflect.Struct {
			return 0, errors.New("non-structure type not supported yet")
		}

		// Fields or KeyVals or None
		s := rt.(reflect2.StructType)
		if len(args) > 0 && args[0].Type() == _fields {
			m := t.getStructFieldMap(s)

			sb.WriteString(" (")
			for i, field := range args[0].(*fieldsItem).Fields {
				f := m[field]
				if f != nil {
					cols = append(cols, f)
				}

				if i > 0 {
					sb.WriteString(",")
				}
				fieldEscape(&sb, field)
			}
			sb.WriteString(") values (")

			args = args[1:]

		} else {
			sb.WriteString(" (")
			for i := 0; i < s.NumField(); i++ {
				f := s.Field(i)
				ft := f.Tag().Get("borm")

				if !t.Cfg.UseNameWhenTagEmpty && ft == "" {
					continue
				}

				if len(cols) > 0 {
					sb.WriteString(",")
				}

				if ft == "" {
					fieldEscape(&sb, f.Name())
				} else {
					fieldEscape(&sb, ft)
				}

				cols = append(cols, f)
			}
			sb.WriteString(") values (")
		}

		// 构建VALUES部分的占位符模板
		valuesTemplate := "("
		for i := range cols {
			if i > 0 {
				valuesTemplate += ","
			}
			valuesTemplate += "?"
		}
		valuesTemplate += ")"

		if isArray {
			// 批量插入：为每个元素添加VALUES
			sliceType := reflect2.TypeOf(objs).(reflect2.PtrType).Elem().(reflect2.SliceType)
			length := sliceType.UnsafeLengthOf(reflect2.PtrOf(objs))
			for i := 0; i < length; i++ {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(valuesTemplate)
				elemPtr := sliceType.UnsafeGetIndex(reflect2.PtrOf(objs), i)
				t.inputArgs(&stmtArgs, cols, rtPtr, s, isPtrArray, elemPtr)
			}
		} else {
			// 单条插入
			sb.WriteString(valuesTemplate)
			t.inputArgs(&stmtArgs, cols, rt, s, false, reflect2.PtrOf(objs))
		}

		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()

		if t.Cfg.Reuse {
			callSite := getCallSite()
			shapeKey := buildShapeKey(callSite.Key, "Insert", args)
			// 存字段列，避免二次反射
			item := &DataBindingItem{Cols: make([]interface{}, len(cols))}
			for i := range cols {
				item.Cols[i] = cols[i]
			}
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

	// 处理BormLastId字段
	rt := reflect2.TypeOf(objs)
	if rt.Kind() == reflect.Ptr {
		rt = rt.(reflect2.PtrType).Elem()
		if rt.Kind() == reflect.Struct {
			s := rt.(reflect2.StructType)
			if f := s.FieldByName("BormLastId"); f != nil {
				id, _ := res.LastInsertId()
				f.UnsafeSet(reflect2.PtrOf(objs), reflect2.PtrOf(id))
			}
		}
	}

	row, _ := res.RowsAffected()
	return int(row), nil
}

// insertMapWithPrefix 处理V类型（map[string]interface{}）的插入，支持前缀
func (t *BormTable) insertMapWithPrefix(prefix string, m V, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString(prefix)
	fieldEscape(&sb, t.Name)
	sb.WriteString(" (")

	// 检查是否有Fields参数
	hasFields := len(args) > 0 && args[0].Type() == _fields
	var fieldsToProcess []string

	if hasFields {
		fi := args[0].(*fieldsItem)
		fieldsToProcess = fi.Fields
		args = args[1:] // 移除Fields参数
	} else {
		// 处理所有map字段
		for k, v := range m {
			if v != nil {
				fieldsToProcess = append(fieldsToProcess, k)
			}
		}
		// 检查空map
		if len(fieldsToProcess) == 0 {
			return 0, errors.New("empty map: no fields to insert")
		}
	}

	// 构建字段列表
	for i, field := range fieldsToProcess {
		v := m[field]
		if v != nil {
			if i > 0 {
				sb.WriteString(",")
			}
			fieldEscape(&sb, field)
		}
	}

	sb.WriteString(") values (")

	// 构建VALUES部分
	for i, field := range fieldsToProcess {
		v := m[field]
		if v != nil {
			if i > 0 {
				sb.WriteString(",")
			}
			if s, ok := v.(U); ok {
				sb.WriteString(string(s))
			} else {
				sb.WriteString("?")
				stmtArgs = append(stmtArgs, v)
			}
		}
	}
	sb.WriteString(")")

	// 构建其他条件
	for _, arg := range args {
		arg.BuildSQL(&sb)
		arg.BuildArgs(&stmtArgs)
	}

	sqlStr := sb.String()
	if t.Cfg.Debug {
		log.Printf("%s %v", sqlStr, stmtArgs)
	}

	result, err := t.DB.ExecContext(t.ctx, sqlStr, stmtArgs...)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	return int(affected), err
}

// insertGenericMapWithPrefix 处理通用map类型的插入，支持前缀
func (t *BormTable) insertGenericMapWithPrefix(prefix string, obj interface{}, mapType reflect2.MapType, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString(prefix)
	fieldEscape(&sb, t.Name)
	sb.WriteString(" (")

	// 使用reflect包获取map的迭代器
	rv := reflect.ValueOf(obj)
	mapIter := rv.MapRange()

	// 用于存储字段信息的临时结构
	type mapFieldData struct {
		key   string
		value interface{}
	}

	var fieldDataList []mapFieldData

	// 收集map中的所有字段
	for mapIter.Next() {
		key := mapIter.Key()
		value := mapIter.Value()
		keyStr := key.String()
		fieldDataList = append(fieldDataList, mapFieldData{
			key:   keyStr,
			value: value.Interface(),
		})
	}

	// 按key排序，确保字段顺序一致
	sort.Slice(fieldDataList, func(i, j int) bool {
		return fieldDataList[i].key < fieldDataList[j].key
	})

	// 构建字段列表
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, fieldData.key)
	}
	sb.WriteString(") values (")

	// 构建VALUES部分
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("?")
		stmtArgs = append(stmtArgs, fieldData.value)
	}
	sb.WriteString(")")

	// 构建其他条件
	for _, arg := range args {
		arg.BuildSQL(&sb)
		arg.BuildArgs(&stmtArgs)
	}

	sqlStr := sb.String()
	if t.Cfg.Debug {
		log.Printf("%s %v", sqlStr, stmtArgs)
	}

	result, err := t.DB.ExecContext(t.ctx, sqlStr, stmtArgs...)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	return int(affected), err
}

// insertStructWithPrefix 处理结构体类型的插入，支持前缀
func (t *BormTable) insertStructWithPrefix(prefix string, objs interface{}, args ...BormItem) (int, error) {
	var (
		item     *DataBindingItem
		stmtArgs []interface{}
	)

	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey := buildShapeKey(callSite.Key, "Insert", args)
		if i, ok := _dataBindingCache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil {
		// 使用缓存的SQL，但需要重新构建参数
		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// 构建新的SQL
		item = &DataBindingItem{}
		var sb strings.Builder
		sb.WriteString(prefix)
		fieldEscape(&sb, t.Name)

		rt := reflect2.TypeOf(objs)
		switch rt.Kind() {
		case reflect.Ptr:
			rt = rt.(reflect2.PtrType).Elem()
		default:
			return 0, errors.New("argument 2 should be map or ptr")
		}

		var cols []reflect2.StructField

		// Fields or None
		// struct类型
		if rt.Kind() != reflect.Struct {
			return 0, errors.New("non-structure type not supported yet")
		}

		// Fields or KeyVals or None
		s := rt.(reflect2.StructType)
		if len(args) > 0 && args[0].Type() == _fields {
			m := t.getStructFieldMap(s)

			sb.WriteString(" (")
			for i, field := range args[0].(*fieldsItem).Fields {
				f := m[field]
				if f != nil {
					cols = append(cols, f)
				}

				if i > 0 {
					sb.WriteString(",")
				}
				fieldEscape(&sb, field)
			}
			sb.WriteString(") values (")

			args = args[1:]

		} else {
			sb.WriteString(" (")
			for i := 0; i < s.NumField(); i++ {
				f := s.Field(i)
				ft := f.Tag().Get("borm")

				if !t.Cfg.UseNameWhenTagEmpty && ft == "" {
					continue
				}

				if len(cols) > 0 {
					sb.WriteString(",")
				}

				if ft == "" {
					fieldEscape(&sb, f.Name())
				} else {
					fieldEscape(&sb, ft)
				}

				cols = append(cols, f)
			}
			sb.WriteString(") values (")
		}

		// 构建VALUES部分的占位符
		for i := range cols {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("?")
		}
		sb.WriteString(")")

		t.inputArgs(&stmtArgs, cols, rt, s, false, reflect2.PtrOf(objs))

		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()

		if t.Cfg.Reuse {
			callSite := getCallSite()
			shapeKey := buildShapeKey(callSite.Key, "Insert", args)
			// 存字段列，避免二次反射
			item := &DataBindingItem{Cols: make([]interface{}, len(cols))}
			for i := range cols {
				item.Cols[i] = cols[i]
			}
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

	// 处理BormLastId字段
	rt := reflect2.TypeOf(objs)
	if rt.Kind() == reflect.Ptr {
		rt = rt.(reflect2.PtrType).Elem()
		if rt.Kind() == reflect.Struct {
			s := rt.(reflect2.StructType)
			if f := s.FieldByName("BormLastId"); f != nil {
				id, _ := res.LastInsertId()
				f.UnsafeSet(reflect2.PtrOf(objs), reflect2.PtrOf(id))
			}
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

	// 检查是否为V类型（map[string]interface{}）
	if m, ok := obj.(V); ok {
		return t.updateMap(m, args...)
	}

	// 检查是否为通用map类型
	rt := reflect2.TypeOf(obj)
	if rt.Kind() == reflect.Map {
		mapType := rt.(reflect2.MapType)
		if mapType.Key().Kind() != reflect.String {
			return 0, errors.New("map key must be string type")
		}
		return t.updateGenericMap(obj, mapType, args...)
	}

	// 处理结构体类型
	return t.updateStruct(obj, args...)
}

// updateMap 处理V类型（map[string]interface{}）的更新
func (t *BormTable) updateMap(m V, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString("update ")
	fieldEscape(&sb, t.Name)
	sb.WriteString(" set ")

	// 检查是否有Fields参数
	hasFields := len(args) > 0 && args[0].Type() == _fields
	var fieldsToProcess []string

	if hasFields {
		fi := args[0].(*fieldsItem)
		fieldsToProcess = fi.Fields
		args = args[1:] // 移除Fields参数
	} else {
		// 处理所有map字段
		for k, v := range m {
			if v != nil {
				fieldsToProcess = append(fieldsToProcess, k)
			}
		}
	}

	// 构建SET部分
	for i, field := range fieldsToProcess {
		v := m[field]
		if v != nil {
			if i > 0 {
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
		}
	}

	// 构建WHERE条件
	for _, arg := range args {
		arg.BuildSQL(&sb)
		arg.BuildArgs(&stmtArgs)
	}

	sqlStr := sb.String()
	if t.Cfg.Debug {
		log.Printf("%s %v", sqlStr, stmtArgs)
	}

	result, err := t.DB.ExecContext(t.ctx, sqlStr, stmtArgs...)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	return int(affected), err
}

// updateGenericMap 处理通用map类型的更新
func (t *BormTable) updateGenericMap(obj interface{}, mapType reflect2.MapType, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString("update ")
	fieldEscape(&sb, t.Name)
	sb.WriteString(" set ")

	// 使用reflect包获取map的迭代器
	rv := reflect.ValueOf(obj)
	mapIter := rv.MapRange()

	// 用于存储字段信息的临时结构
	type mapFieldData struct {
		key   string
		value interface{}
	}

	var fieldDataList []mapFieldData

	// 收集map中的所有字段
	for mapIter.Next() {
		key := mapIter.Key()
		value := mapIter.Value()
		keyStr := key.String()
		fieldDataList = append(fieldDataList, mapFieldData{
			key:   keyStr,
			value: value.Interface(),
		})
	}

	// 按key排序，确保字段顺序一致
	sort.Slice(fieldDataList, func(i, j int) bool {
		return fieldDataList[i].key < fieldDataList[j].key
	})

	// 构建SET部分
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, fieldData.key)
		sb.WriteString("=?")
		stmtArgs = append(stmtArgs, fieldData.value)
	}

	// 构建WHERE条件
	for _, arg := range args {
		arg.BuildSQL(&sb)
		arg.BuildArgs(&stmtArgs)
	}

	sqlStr := sb.String()
	if t.Cfg.Debug {
		log.Printf("%s %v", sqlStr, stmtArgs)
	}

	result, err := t.DB.ExecContext(t.ctx, sqlStr, stmtArgs...)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	return int(affected), err
}

// updateStruct 处理结构体类型的更新
func (t *BormTable) updateStruct(obj interface{}, args ...BormItem) (int, error) {
	// 对于结构体类型，暂时不使用缓存
	rt := reflect2.TypeOf(obj)
	useCache := t.Cfg.Reuse && rt.Kind() == reflect.Ptr && rt.(reflect2.PtrType).Elem().Kind() == reflect.Struct

	var (
		item     *DataBindingItem
		stmtArgs []interface{}
	)

	if useCache {
		callSite := getCallSite()
		shapeKey := buildShapeKey(callSite.Key, "Update", args)
		if i, ok := _dataBindingCache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil {
		// 走缓存路径时，仍需按对象构建参数
		// 此处省略 SQL 重建
	} else {
		// 构建新的SQL
		item = &DataBindingItem{}
		var sb strings.Builder
		sb.WriteString("update ")
		fieldEscape(&sb, t.Name)
		sb.WriteString(" set ")

		rt := reflect2.TypeOf(obj)
		switch rt.Kind() {
		case reflect.Ptr:
			rt = rt.(reflect2.PtrType).Elem()
		default:
			return 0, errors.New("argument 2 should be map or ptr")
		}

		var cols []reflect2.StructField

		// Fields or None
		// struct类型
		if rt.Kind() != reflect.Struct {
			return 0, errors.New("non-structure type not supported yet")
		}

		// Fields or KeyVals or None
		s := rt.(reflect2.StructType)
		if len(args) > 0 && args[0].Type() == _fields {
			m := t.getStructFieldMap(s)

			for i, field := range args[0].(*fieldsItem).Fields {
				f := m[field]
				if f != nil {
					cols = append(cols, f)
				}

				if i > 0 {
					sb.WriteString(",")
				}
				fieldEscape(&sb, field)
				sb.WriteString("=?")
			}

			args = args[1:]

		} else {
			for i := 0; i < s.NumField(); i++ {
				f := s.Field(i)
				ft := f.Tag().Get("borm")

				if !t.Cfg.UseNameWhenTagEmpty && ft == "" {
					continue
				}

				if len(cols) > 0 {
					sb.WriteString(",")
				}

				if ft == "" {
					fieldEscape(&sb, f.Name())
					sb.WriteString("=?")
				} else {
					fieldEscape(&sb, ft)
					sb.WriteString("=?")
				}

				cols = append(cols, f)
			}
		}

		t.inputArgs(&stmtArgs, cols, rt, s, false, reflect2.PtrOf(obj))

		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()

		if useCache {
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
		item     *DataBindingItem
		stmtArgs []interface{}
	)

	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey := buildShapeKey(callSite.Key, "Delete", args)
		if i, ok := _dataBindingCache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil {
		// 使用缓存的SQL和参数
		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// 构建新的SQL
		item = &DataBindingItem{}
		var sb strings.Builder
		sb.WriteString("delete from ")
		fieldEscape(&sb, t.Name)

		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()

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
				v = v.(*time.Time).Unix()
			} else {
				v = v.(*time.Time).Format(_timeLayout)
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
	DB            BormDBIFace
	Name          string
	Cfg           Config
	ctx           context.Context
	fieldMapCache sync.Map // 字段映射缓存
}

func fieldEscape(sb *strings.Builder, field string) {
	if field == "" {
		return
	}
	if !strings.ContainsAny(field, ",( `.") {
		sb.WriteString("`" + field + "`")
	} else {
		// TODO: 处理alias场景
		sb.WriteString(field)
	}
}

func (t *BormTable) getStructFieldMap(s reflect2.StructType) map[string]reflect2.StructField {
	// 检查缓存
	if cached, ok := t.fieldMapCache.Load(s); ok {
		return cached.(map[string]reflect2.StructField)
	}

	// 收集字段
	m := t.collectStructFields(s, "")

	// 缓存结果
	t.fieldMapCache.Store(s, m)
	return m
}

// collectStructFields 递归收集结构体字段，支持embedded struct和字段忽略
func (t *BormTable) collectStructFields(s reflect2.StructType, prefix string) map[string]reflect2.StructField {
	m := make(map[string]reflect2.StructField)
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("borm")

		// 检查是否忽略字段
		if ft == "-" {
			continue
		}

		// 处理embedded struct
		if f.Anonymous() {
			embeddedType := f.Type()
			if embeddedType.Kind() == reflect.Struct {
				if embeddedStructType, ok := embeddedType.(reflect2.StructType); ok {
					// 递归收集embedded struct的字段
					embeddedFields := t.collectStructFields(embeddedStructType, prefix)
					for k, v := range embeddedFields {
						m[k] = v
					}
				}
			}
			continue
		}

		// 处理普通字段
		if ft != "" {
			m[ft] = f
		} else if t.Cfg.UseNameWhenTagEmpty {
			m[f.Name()] = f
		}
	}
	return m
}

// FieldInfo 通用字段信息接口
type FieldInfo interface {
	GetName() string
	GetValue(ptr unsafe.Pointer) interface{}
	GetType() reflect2.Type
}

// StructFieldInfo 结构体字段信息
type StructFieldInfo struct {
	Field reflect2.StructField
}

func (f *StructFieldInfo) GetName() string {
	return f.Field.Name()
}

func (f *StructFieldInfo) GetValue(ptr unsafe.Pointer) interface{} {
	return f.Field.Get(ptr)
}

func (f *StructFieldInfo) GetType() reflect2.Type {
	return f.Field.Type()
}

// MapFieldInfo map字段信息
type MapFieldInfo struct {
	Key   string
	Value interface{}
	Type  reflect2.Type
}

func (f *MapFieldInfo) GetName() string {
	return f.Key
}

func (f *MapFieldInfo) GetValue(ptr unsafe.Pointer) interface{} {
	return f.Value
}

func (f *MapFieldInfo) GetType() reflect2.Type {
	return f.Type
}

// BormItem .
type BormItem interface {
	Type() int
	BuildSQL(*strings.Builder)
	BuildArgs(*[]interface{})
}

// collectFieldsGeneric 通用字段收集函数
func (t *BormTable) collectFieldsGeneric(objs interface{}, sb *strings.Builder, fieldInfos *[]FieldInfo) error {
	rt := reflect2.TypeOf(objs)

	switch rt.Kind() {
	case reflect.Struct:
		return t.collectStructFieldsGeneric(objs, rt.(reflect2.StructType), sb, fieldInfos)
	case reflect.Map:
		return t.collectMapFieldsGeneric(objs, rt.(reflect2.MapType), sb, fieldInfos)
	default:
		return errors.New("unsupported type for field collection")
	}
}

// collectStructFieldsGeneric 收集结构体字段
func (t *BormTable) collectStructFieldsGeneric(objs interface{}, structType reflect2.StructType, sb *strings.Builder, fieldInfos *[]FieldInfo) error {
	ptr := reflect2.PtrOf(objs)

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		ft := field.Tag().Get("borm")

		// 检查是否忽略字段
		if ft == "-" {
			continue
		}

		// 处理embedded struct
		if field.Anonymous() {
			embeddedType := field.Type()
			if embeddedType.Kind() == reflect.Struct {
				if embeddedStructType, ok := embeddedType.(reflect2.StructType); ok {
					// 递归收集embedded struct的字段
					embeddedPtr := unsafe.Pointer(uintptr(ptr) + field.Offset())
					embeddedObj := embeddedStructType.New()
					// 将embedded对象复制到正确的位置
					*(*unsafe.Pointer)(unsafe.Pointer(&embeddedObj)) = embeddedPtr
					err := t.collectStructFieldsGeneric(embeddedObj, embeddedStructType, sb, fieldInfos)
					if err != nil {
						return err
					}
				}
			}
			continue
		}

		// 处理普通字段
		var fieldName string
		if ft != "" {
			fieldName = ft
		} else if t.Cfg.UseNameWhenTagEmpty {
			fieldName = field.Name()
		} else {
			continue
		}

		fieldEscape(sb, fieldName)
		sb.WriteString(",")

		*fieldInfos = append(*fieldInfos, &StructFieldInfo{Field: field})
	}

	return nil
}

// collectMapFieldsGeneric 收集map字段
func (t *BormTable) collectMapFieldsGeneric(objs interface{}, mapType reflect2.MapType, sb *strings.Builder, fieldInfos *[]FieldInfo) error {
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

	// 构建SQL和字段信息
	for _, fieldData := range fieldDataList {
		fieldEscape(sb, fieldData.key)
		sb.WriteString(",")

		*fieldInfos = append(*fieldInfos, &MapFieldInfo{
			Key:   fieldData.key,
			Value: fieldData.value,
			Type:  fieldData.valueType,
		})
	}

	return nil
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

type onDuplicateKeyUpdateItem struct {
	Conds string
	Vals  []interface{}
}

func (w *onDuplicateKeyUpdateItem) Type() int {
	return _onDuplicateKeyUpdate
}

func (w *onDuplicateKeyUpdateItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(w.Conds)
}

func (w *onDuplicateKeyUpdateItem) BuildArgs(stmtArgs *[]interface{}) {
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

type forceIndexItem struct {
	idx string
}

func (w *forceIndexItem) Type() int {
	return _forceIndex
}

func (w *forceIndexItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(" force index(" + w.idx + ")")
}

func (w *forceIndexItem) BuildArgs(stmtArgs *[]interface{}) {
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
			fieldEscape(sb, cond.Field)
			sb.WriteString(cond.Op)
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
			fieldEscape(sb, cond.Field)
			sb.WriteString(cond.Op)
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
		sb.WriteString(",?")
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

// parseTimeString 优化的时间解析函数，智能检测格式
func parseTimeString(s string) (time.Time, error) {
	if s == "" || s == "NULL" || s == "null" {
		return time.Time{}, nil
	}

	// 处理MySQL无效日期格式 0000-00-00 和 0000-00-00 00:00:00
	if strings.HasPrefix(s, "0000-00-00") {
		return time.Time{}, nil
	}

	// 纯日期格式检测
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return time.Parse("2006-01-02", s)
	}

	// 带时区的格式检测
	if len(s) > 10 {
		// 检测是否包含时区信息
		if s[len(s)-6] == '+' || s[len(s)-6] == '-' || s[len(s)-1] == 'Z' {
			// 带纳秒的时区格式
			if len(s) > 26 && s[19] == '.' {
				return time.Parse(_timeLayoutWithNanoTZ, s)
			}
			// 带时区的格式
			return time.Parse(_timeLayoutWithTZ, s)
		}
		// Z结尾的格式
		if s[len(s)-1] == 'Z' {
			return time.Parse(_timeLayoutWithZ, s)
		}
	}

	// 默认格式
	return time.Parse(_timeLayout, s)
}

func scanFromString(isTime bool, st reflect2.Type, dt reflect2.Type, ptrVal unsafe.Pointer, tmp string) error {
	dk := dt.Kind()

	// 时间格式(DATE/DATETIME) => number/time.Time
	if isTime || (dk >= reflect.Int && dk <= reflect.Float64) {
		if isTime {
			// 使用优化的时间解析函数
			parsedTime, err := parseTimeString(tmp)
			if err != nil {
				// 尝试解析为时间戳
				i64, parseErr := strconv.ParseInt(tmp, 10, 64)
				if parseErr != nil {
					return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
				}
				parsedTime = time.Unix(i64, 0)
			}
			*(*time.Time)(ptrVal) = parsedTime.UTC()
			return nil
		}

		// 对于数字类型，先尝试解析时间字符串
		parsedTime, err := parseTimeString(tmp)
		if err == nil {
			ts := parsedTime.Unix()
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
		}

		// 如果时间解析失败，尝试直接解析为数字
		// 对于浮点类型，使用ParseFloat
		if dk == reflect.Float32 || dk == reflect.Float64 {
			f64, err := strconv.ParseFloat(tmp, 64)
			if err != nil {
				return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
			}
			if dk == reflect.Float32 {
				*(*float32)(ptrVal) = float32(f64)
			} else {
				*(*float64)(ptrVal) = f64
			}
			return nil
		}
		
		// 对于整数类型，使用ParseInt
		i64, err := strconv.ParseInt(tmp, 10, 64)
		if err != nil {
			return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
		}
		switch dk {
		case reflect.Int:
			*(*int)(ptrVal) = int(i64)
		case reflect.Int8:
			*(*int8)(ptrVal) = int8(i64)
		case reflect.Int16:
			*(*int16)(ptrVal) = int16(i64)
		case reflect.Int32:
			*(*int32)(ptrVal) = int32(i64)
		case reflect.Int64:
			*(*int64)(ptrVal) = i64
		case reflect.Uint:
			*(*uint)(ptrVal) = uint(i64)
		case reflect.Uint8:
			*(*uint8)(ptrVal) = uint8(i64)
		case reflect.Uint16:
			*(*uint16)(ptrVal) = uint16(i64)
		case reflect.Uint32:
			*(*uint32)(ptrVal) = uint32(i64)
		case reflect.Uint64:
			*(*uint64)(ptrVal) = uint64(i64)
		}
		return nil
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
		// 如果是指针类型，设置为nil
		if dt.Kind() == reflect.Ptr {
			ptrType := dt.(reflect2.PtrType)
			ptrType.UnsafeSet(dest.Val, nil)
		} else {
			// 设置成默认值
			dt.UnsafeSet(dest.Val, dt.UnsafeNew())
		}
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
		*(*time.Time)(dest.Val) = time.Unix(src.(int64), 0)
		return nil
	}

	if sk == reflect.String {
		return scanFromString(isTime, st, dt, dest.Val, src.(string))
	} else if sk == reflect.Slice && st.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
		return scanFromString(isTime, st, dt, dest.Val, string(src.([]byte)))
	} else if st.String() == "time.Time" {
		if dk == reflect.String {
			return dest.Scan(src.(time.Time).Format(_timeLayout))
		}
		return dest.Scan(src.(time.Time).Unix())
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
			fieldEscape(sb, cond.Field)
			sb.WriteString(cond.Op)
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
			// 在Reuse模式下，单条也使用In，保持一致性
			// 这样可以避免缓存不一致问题，性能差距也不大
			var sb strings.Builder
			sb.WriteString(" in (?)")
			return &ormCond{Field: field, Op: sb.String(), Args: args}
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
	SQL  string
	Cols []interface{}
	Type reflect2.Type
	Elem interface{}
}

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
	_dataBindingCache sync.Map
	_callSiteCache    sync.Map // map[uintptr]*CallSite
	_cacheKeyPool     = sync.Pool{
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
