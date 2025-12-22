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
	Reuse               bool // Enabled by default, provides 2-14x performance improvement
	UseNameWhenTagEmpty bool
	ToTimestamp         bool
}

// Table .
func Table(db BormDBIFace, name string) *BormTable {
	return &BormTable{
		DB:   db,
		Name: name,
		ctx:  context.Background(),
		Cfg:  Config{Reuse: true}, // Enable Reuse by default
	}
}

// TableContext creates a Table with Context, parameter order: context, db, name
func TableContext(ctx context.Context, db BormDBIFace, name string) *BormTable {
	return &BormTable{
		DB:   db,
		Name: name,
		ctx:  ctx,
		Cfg:  Config{Reuse: true}, // Enable Reuse by default
	}
}

// Reuse .
func (t *BormTable) Reuse() *BormTable {
	t.Cfg.Reuse = true
	return t
}

// NoReuse disables Reuse functionality (if cache optimization is not needed)
func (t *BormTable) NoReuse() *BormTable {
	t.Cfg.Reuse = false
	return t
}

// SafeReuse has been merged into Reuse, kept for compatibility
func (t *BormTable) SafeReuse() *BormTable { return t.Reuse() }

// NoSafeReuse has been merged into Reuse, kept for compatibility
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

	// Map type selection: requires explicit Fields, and does not use reuse cache
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

		// Build scan target
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
				// Convert []byte to string
				if b, ok := val.([]byte); ok {
					m[name] = string(b)
				} else {
					m[name] = val
				}
			}
			// Set to *map[string]interface{}
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
				// Convert []byte to string
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
		// struct type
		if rtElem.Kind() == reflect.Struct {
			if args[0].Type() == _fields {
				args = args[1:]
			}
			// map type
			// } else if rt.Kind() == reflect.Map {
			// TODO
			// other types
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

		// struct type
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
			// map type
			// } else if rt.Kind() == reflect.Map {
			// TODO
			// other types
		} else {
			// Must have fields and be 1
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

	// Check if it's V type (map[string]interface{})
	if m, ok := objs.(V); ok {
		return t.insertMapWithPrefix("insert ignore into ", m, args...)
	}

	// Check if it's a generic map type
	rt := reflect2.TypeOf(objs)
	if rt.Kind() == reflect.Map {
		mapType := rt.(reflect2.MapType)
		if mapType.Key().Kind() != reflect.String {
			return 0, errors.New("map key must be string type")
		}
		return t.insertGenericMapWithPrefix("insert ignore into ", objs, mapType, args...)
	}

	// Handle struct type
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

	// Check if it's V type (map[string]interface{})
	if m, ok := objs.(V); ok {
		return t.insertMapWithPrefix("replace into ", m, args...)
	}

	// Check if it's a generic map type
	rt := reflect2.TypeOf(objs)
	if rt.Kind() == reflect.Map {
		mapType := rt.(reflect2.MapType)
		if mapType.Key().Kind() != reflect.String {
			return 0, errors.New("map key must be string type")
		}
		return t.insertGenericMapWithPrefix("replace into ", objs, mapType, args...)
	}

	// Handle struct type
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

	// Check if it's V type (map[string]interface{})
	if m, ok := objs.(V); ok {
		return t.insertMap(m, args...)
	}

	// Check if it's a generic map type
	rt := reflect2.TypeOf(objs)
	if rt.Kind() == reflect.Map {
		mapType := rt.(reflect2.MapType)
		if mapType.Key().Kind() != reflect.String {
			return 0, errors.New("map key must be string type")
		}
		return t.insertGenericMap(objs, mapType, args...)
	}

	// Handle struct type
	return t.insertStruct(objs, args...)
}

// insertMap handles insertion of V type (map[string]interface{})
func (t *BormTable) insertMap(m V, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString("insert into ")
	fieldEscape(&sb, t.Name)
	sb.WriteString(" (")

	// Check if there are Fields parameters
	hasFields := len(args) > 0 && args[0].Type() == _fields
	var fieldsToProcess []string

	if hasFields {
		fi := args[0].(*fieldsItem)
		fieldsToProcess = fi.Fields
		args = args[1:] // Remove Fields parameter
	} else {
		// Process all map fields
		for k, v := range m {
			if v != nil {
				fieldsToProcess = append(fieldsToProcess, k)
			}
		}
		// Check empty map
		if len(fieldsToProcess) == 0 {
			return 0, errors.New("empty map: no fields to insert")
		}
	}

	// Build field list
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

	// Build VALUES section
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

	// Build other conditions
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

// insertGenericMap handles insertion of generic map types
func (t *BormTable) insertGenericMap(obj interface{}, mapType reflect2.MapType, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString("insert into ")
	fieldEscape(&sb, t.Name)
	sb.WriteString(" (")

	// Use reflect package to get map iterator
	rv := reflect.ValueOf(obj)
	mapIter := rv.MapRange()

	// Temporary structure for storing field information
	type mapFieldData struct {
		key   string
		value interface{}
	}

	var fieldDataList []mapFieldData

	// Collect all fields from map
	for mapIter.Next() {
		key := mapIter.Key()
		value := mapIter.Value()
		keyStr := key.String()
		fieldDataList = append(fieldDataList, mapFieldData{
			key:   keyStr,
			value: value.Interface(),
		})
	}

	// Sort by key to ensure consistent field order
	sort.Slice(fieldDataList, func(i, j int) bool {
		return fieldDataList[i].key < fieldDataList[j].key
	})

	// Build field list
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, fieldData.key)
	}
	sb.WriteString(") values (")

	// Build VALUES section
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("?")
		stmtArgs = append(stmtArgs, fieldData.value)
	}
	sb.WriteString(")")

	// Build other conditions
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

// insertStruct handles insertion of struct types
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
		// Use cached SQL, but need to rebuild parameters
		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// Build new SQL
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
		// struct type
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

		// Check if there are fields to insert
		if len(cols) == 0 {
			return 0, errors.New("no fields to insert")
		}

		// Build placeholder template for VALUES section (without parentheses, as " values (" was already written)
		valuesTemplate := ""
		for i := range cols {
			if i > 0 {
				valuesTemplate += ","
			}
			valuesTemplate += "?"
		}

		if isArray {
			// Batch insert: add VALUES for each element
			sliceType := reflect2.TypeOf(objs).(reflect2.PtrType).Elem().(reflect2.SliceType)
			length := sliceType.UnsafeLengthOf(reflect2.PtrOf(objs))
			for i := 0; i < length; i++ {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString("(")
				sb.WriteString(valuesTemplate)
				sb.WriteString(")")
				elemPtr := sliceType.UnsafeGetIndex(reflect2.PtrOf(objs), i)
				t.inputArgs(&stmtArgs, cols, rtPtr, s, isPtrArray, elemPtr)
			}
		} else {
			// Single insert
			sb.WriteString(valuesTemplate)
			t.inputArgs(&stmtArgs, cols, rt, s, false, reflect2.PtrOf(objs))
		}
		sb.WriteString(")")

		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()

		if t.Cfg.Reuse {
			callSite := getCallSite()
			shapeKey := buildShapeKey(callSite.Key, "Insert", args)
			// Store field columns to avoid second reflection
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

	// Handle BormLastId field
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

// insertMapWithPrefix handles insertion of V type (map[string]interface{}), supports prefix
func (t *BormTable) insertMapWithPrefix(prefix string, m V, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString(prefix)
	fieldEscape(&sb, t.Name)
	sb.WriteString(" (")

	// Check if there are Fields parameters
	hasFields := len(args) > 0 && args[0].Type() == _fields
	var fieldsToProcess []string

	if hasFields {
		fi := args[0].(*fieldsItem)
		fieldsToProcess = fi.Fields
		args = args[1:] // Remove Fields parameter
	} else {
		// Process all map fields
		for k, v := range m {
			if v != nil {
				fieldsToProcess = append(fieldsToProcess, k)
			}
		}
		// Check empty map
		if len(fieldsToProcess) == 0 {
			return 0, errors.New("empty map: no fields to insert")
		}
	}

	// Build field list
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

	// Build VALUES section
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

	// Build other conditions
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

// insertGenericMapWithPrefix handles insertion of generic map types, supports prefix
func (t *BormTable) insertGenericMapWithPrefix(prefix string, obj interface{}, mapType reflect2.MapType, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString(prefix)
	fieldEscape(&sb, t.Name)
	sb.WriteString(" (")

	// Use reflect package to get map iterator
	rv := reflect.ValueOf(obj)
	mapIter := rv.MapRange()

	// Temporary structure for storing field information
	type mapFieldData struct {
		key   string
		value interface{}
	}

	var fieldDataList []mapFieldData

	// Collect all fields from map
	for mapIter.Next() {
		key := mapIter.Key()
		value := mapIter.Value()
		keyStr := key.String()
		fieldDataList = append(fieldDataList, mapFieldData{
			key:   keyStr,
			value: value.Interface(),
		})
	}

	// Sort by key to ensure consistent field order
	sort.Slice(fieldDataList, func(i, j int) bool {
		return fieldDataList[i].key < fieldDataList[j].key
	})

	// Build field list
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, fieldData.key)
	}
	sb.WriteString(") values (")

	// Build VALUES section
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("?")
		stmtArgs = append(stmtArgs, fieldData.value)
	}
	sb.WriteString(")")

	// Build other conditions
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

// insertStructWithPrefix handles insertion of struct types, supports prefix
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
		// Use cached SQL, but need to rebuild parameters
		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// Build new SQL
		item = &DataBindingItem{}
		var sb strings.Builder
		sb.WriteString(prefix)
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
		// struct type
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

		// Check if there are fields to insert
		if len(cols) == 0 {
			return 0, errors.New("no fields to insert")
		}

		// Build placeholder template for VALUES section (without parentheses, as " values (" was already written)
		valuesTemplate := ""
		for i := range cols {
			if i > 0 {
				valuesTemplate += ","
			}
			valuesTemplate += "?"
		}

		if isArray {
			// Batch insert: add VALUES for each element
			sliceType := reflect2.TypeOf(objs).(reflect2.PtrType).Elem().(reflect2.SliceType)
			length := sliceType.UnsafeLengthOf(reflect2.PtrOf(objs))
			for i := 0; i < length; i++ {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString("(")
				sb.WriteString(valuesTemplate)
				sb.WriteString(")")
				elemPtr := sliceType.UnsafeGetIndex(reflect2.PtrOf(objs), i)
				t.inputArgs(&stmtArgs, cols, rtPtr, s, isPtrArray, elemPtr)
			}
		} else {
			// Single insert
			sb.WriteString(valuesTemplate)
			t.inputArgs(&stmtArgs, cols, rt, s, false, reflect2.PtrOf(objs))
		}
		sb.WriteString(")")

		for _, arg := range args {
			arg.BuildSQL(&sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()

		if t.Cfg.Reuse {
			callSite := getCallSite()
			shapeKey := buildShapeKey(callSite.Key, "Insert", args)
			// Store field columns to avoid second reflection
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

	// Handle BormLastId field
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

	// Check if it's V type (map[string]interface{})
	if m, ok := obj.(V); ok {
		return t.updateMap(m, args...)
	}

	// Check if it's a generic map type
	rt := reflect2.TypeOf(obj)
	if rt.Kind() == reflect.Map {
		mapType := rt.(reflect2.MapType)
		if mapType.Key().Kind() != reflect.String {
			return 0, errors.New("map key must be string type")
		}
		return t.updateGenericMap(obj, mapType, args...)
	}

	// Handle struct type
	return t.updateStruct(obj, args...)
}

// updateMap handles update of V type (map[string]interface{})
func (t *BormTable) updateMap(m V, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString("update ")
	fieldEscape(&sb, t.Name)
	sb.WriteString(" set ")

	// Check if there are Fields parameters
	hasFields := len(args) > 0 && args[0].Type() == _fields
	var fieldsToProcess []string

	if hasFields {
		fi := args[0].(*fieldsItem)
		fieldsToProcess = fi.Fields
		args = args[1:] // Remove Fields parameter
	} else {
		// Process all map fields
		for k, v := range m {
			if v != nil {
				fieldsToProcess = append(fieldsToProcess, k)
			}
		}
	}

	// Build SET section
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

	// Build WHERE conditions
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

// updateGenericMap handles update of generic map types
func (t *BormTable) updateGenericMap(obj interface{}, mapType reflect2.MapType, args ...BormItem) (int, error) {
	var sb strings.Builder
	var stmtArgs []interface{}

	sb.WriteString("update ")
	fieldEscape(&sb, t.Name)
	sb.WriteString(" set ")

	// Use reflect package to get map iterator
	rv := reflect.ValueOf(obj)
	mapIter := rv.MapRange()

	// Temporary structure for storing field information
	type mapFieldData struct {
		key   string
		value interface{}
	}

	var fieldDataList []mapFieldData

	// Collect all fields from map
	for mapIter.Next() {
		key := mapIter.Key()
		value := mapIter.Value()
		keyStr := key.String()
		fieldDataList = append(fieldDataList, mapFieldData{
			key:   keyStr,
			value: value.Interface(),
		})
	}

	// Sort by key to ensure consistent field order
	sort.Slice(fieldDataList, func(i, j int) bool {
		return fieldDataList[i].key < fieldDataList[j].key
	})

	// Build SET section
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, fieldData.key)
		sb.WriteString("=?")
		stmtArgs = append(stmtArgs, fieldData.value)
	}

	// Build WHERE conditions
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

// updateStruct handles update of struct types
func (t *BormTable) updateStruct(obj interface{}, args ...BormItem) (int, error) {
	// For struct types, cache is not used for now
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
		// When using cache path, still need to build parameters by object
		// SQL reconstruction is omitted here
	} else {
		// Build new SQL
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
		// struct type
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
		// Use cached SQL and parameters
		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// Build new SQL
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

		// Special handling for time type
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
	fieldMapCache sync.Map // Field mapping cache
}

func fieldEscape(sb *strings.Builder, field string) {
	if field == "" {
		return
	}
	if !strings.ContainsAny(field, ",( `.") {
		sb.WriteString("`" + field + "`")
	} else {
		// TODO: Handle alias scenarios
		sb.WriteString(field)
	}
}

func (t *BormTable) getStructFieldMap(s reflect2.StructType) map[string]reflect2.StructField {
	// Check cache
	if cached, ok := t.fieldMapCache.Load(s); ok {
		return cached.(map[string]reflect2.StructField)
	}

	// Collect fields
	m := t.collectStructFields(s, "")

	// Cache result
	t.fieldMapCache.Store(s, m)
	return m
}

// collectStructFields recursively collects struct fields, supports embedded struct and field ignoring
func (t *BormTable) collectStructFields(s reflect2.StructType, prefix string) map[string]reflect2.StructField {
	m := make(map[string]reflect2.StructField)
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("borm")

		// Check if field should be ignored
		if ft == "-" {
			continue
		}

		// Handle embedded struct
		if f.Anonymous() {
			embeddedType := f.Type()
			if embeddedType.Kind() == reflect.Struct {
				if embeddedStructType, ok := embeddedType.(reflect2.StructType); ok {
					// Recursively collect fields from embedded struct
					embeddedFields := t.collectStructFields(embeddedStructType, prefix)
					for k, v := range embeddedFields {
						m[k] = v
					}
				}
			}
			continue
		}

		// Handle normal fields
		if ft != "" {
			m[ft] = f
		} else if t.Cfg.UseNameWhenTagEmpty {
			m[f.Name()] = f
		}
	}
	return m
}

// FieldInfo generic field information interface
type FieldInfo interface {
	GetName() string
	GetValue(ptr unsafe.Pointer) interface{}
	GetType() reflect2.Type
}

// StructFieldInfo struct field information
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

// MapFieldInfo map field information
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

// collectFieldsGeneric generic field collection function
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

// collectStructFieldsGeneric collects struct fields
func (t *BormTable) collectStructFieldsGeneric(objs interface{}, structType reflect2.StructType, sb *strings.Builder, fieldInfos *[]FieldInfo) error {
	ptr := reflect2.PtrOf(objs)

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		ft := field.Tag().Get("borm")

		// Check if field should be ignored
		if ft == "-" {
			continue
		}

		// Handle embedded struct
		if field.Anonymous() {
			embeddedType := field.Type()
			if embeddedType.Kind() == reflect.Struct {
				if embeddedStructType, ok := embeddedType.(reflect2.StructType); ok {
					// Recursively collect fields from embedded struct
					embeddedPtr := unsafe.Pointer(uintptr(ptr) + field.Offset())
					embeddedObj := embeddedStructType.New()
					// Copy embedded object to correct position
					*(*unsafe.Pointer)(unsafe.Pointer(&embeddedObj)) = embeddedPtr
					err := t.collectStructFieldsGeneric(embeddedObj, embeddedStructType, sb, fieldInfos)
					if err != nil {
						return err
					}
				}
			}
			continue
		}

		// Handle normal fields
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

// collectMapFieldsGeneric collects map fields
func (t *BormTable) collectMapFieldsGeneric(objs interface{}, mapType reflect2.MapType, sb *strings.Builder, fieldInfos *[]FieldInfo) error {
	keyType := mapType.Key()
	if keyType.Kind() != reflect.String {
		return errors.New("map key must be string type")
	}

	// Use reflect package to get map iterator
	rv := reflect.ValueOf(objs)
	mapIter := rv.MapRange()

	// Temporary structure for storing field information
	type mapFieldData struct {
		key       string
		value     interface{}
		valueType reflect2.Type
	}

	var fieldDataList []mapFieldData

	// Collect all fields from map
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

	// Sort by key to ensure consistent field order
	sort.Slice(fieldDataList, func(i, j int) bool {
		return fieldDataList[i].key < fieldDataList[j].key
	})

	// Build SQL and field information
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
		// TODO: Field escaping with ascending/descending keywords
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

// parseTimeString optimized time parsing function with intelligent format detection
func parseTimeString(s string) (time.Time, error) {
	if s == "" || s == "NULL" || s == "null" {
		return time.Time{}, nil
	}

	// Handle MySQL invalid date format 0000-00-00 and 0000-00-00 00:00:00
	if strings.HasPrefix(s, "0000-00-00") {
		return time.Time{}, nil
	}

	// Pure date format detection
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return time.Parse("2006-01-02", s)
	}

	// Timezone format detection
	if len(s) > 10 {
		// Detect if timezone information is included
		if s[len(s)-6] == '+' || s[len(s)-6] == '-' || s[len(s)-1] == 'Z' {
			// Timezone format with nanoseconds
			if len(s) > 26 && s[19] == '.' {
				return time.Parse(_timeLayoutWithNanoTZ, s)
			}
			// Format with timezone
			return time.Parse(_timeLayoutWithTZ, s)
		}
		// Format ending with Z
		if s[len(s)-1] == 'Z' {
			return time.Parse(_timeLayoutWithZ, s)
		}
	}

	// Default format
	return time.Parse(_timeLayout, s)
}

func scanFromString(isTime bool, st reflect2.Type, dt reflect2.Type, ptrVal unsafe.Pointer, tmp string) error {
	dk := dt.Kind()

	// Time format (DATE/DATETIME) => number/time.Time
	if isTime || (dk >= reflect.Int && dk <= reflect.Float64) {
		if isTime {
			// Use optimized time parsing function
			parsedTime, err := parseTimeString(tmp)
			if err != nil {
				// Try to parse as timestamp
				i64, parseErr := strconv.ParseInt(tmp, 10, 64)
				if parseErr != nil {
					return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
				}
				parsedTime = time.Unix(i64, 0)
			}
			*(*time.Time)(ptrVal) = parsedTime.UTC()
			return nil
		}

		// For numeric types, first try to parse time string
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

		// If time parsing fails, try to parse directly as number
		// For float types, use ParseFloat
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

		// For integer types, use ParseInt
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

	// Non-time format
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
		// TODO custom types, try conversion
		return fmt.Errorf("converting driver.Value type %s (%s) to a %s", st.String(), tmp, dt.String())
	}
	return nil
}

func (dest *scanner) Scan(src interface{}) error {
	var (
		st reflect2.Type
		dt = dest.Type
	)

	// NULL value
	if src == nil {
		// If it's a pointer type, set to nil
		if dt.Kind() == reflect.Ptr {
			// Set to nil directly through pointer dereference
			// dest.Val points to a pointer variable, we need to set that pointer variable to nil
			ptrType := dt.(reflect2.PtrType)
			// Create an unsafe.Pointer of nil pointer
			var nilPtr unsafe.Pointer
			ptrType.UnsafeSet(dest.Val, nilPtr)
		} else {
			// Set to default value
			dt.UnsafeSet(dest.Val, dt.UnsafeNew())
		}
		return nil
	}

	st = reflect2.TypeOf(src)
	var (
		sk = st.Kind()
		dk = dt.Kind()
	)

	// Same type, assign directly
	if dk == sk {
		ptrVal := reflect2.PtrOf(src)
		// Check if ptrVal is nil to avoid panic
		if ptrVal == nil && dk == reflect.Ptr {
			// For pointer types, set to nil
			ptrType := dt.(reflect2.PtrType)
			var nilPtr unsafe.Pointer
			ptrType.UnsafeSet(dest.Val, nilPtr)
		} else {
			dt.UnsafeSet(dest.Val, ptrVal)
		}
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
   Conditional logic operations
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
		// If the first argument is an array, convert to interface array
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
			// In Reuse mode, single value also uses In to maintain consistency
			// This avoids cache inconsistency issues, and the performance difference is minimal
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
	data-binding related
*/

// DataBindingItem .
type DataBindingItem struct {
	SQL  string
	Cols []interface{}
	Type reflect2.Type
	Elem interface{}
}

// buildShapeKey builds reuse key based on call site key and parameter shape
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

// Optimized cache operation functions
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
	runtime.Callers(2, pc) // Skip getCallSite and caller
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

// Optimized version: use pre-cached call location
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
Mock related
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
