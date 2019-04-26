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
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/modern-go/reflect2"
)

const (
	_fields = 1 >> iota
	_where
	_groupBy
	_having
	_orderBy
	_limit
	_onDuplicateKeyUpdate

	_andCondEx = iota
	_orCondEx
)

const (
	_timeLayout = "2006-01-02 15:04:05"
)

var config struct {
	Mock bool
}

// V - an alias object value type
type V map[string]interface{}

// Config .
type Config struct {
	Debug               bool
	Reuse               bool
	UseNameWhenTagEmpty bool

	ReplaceInto  bool
	InsertIgnore bool
	ToTimestamp  bool
}

// Table .
func Table(db dbIFace, name string, ctx ...context.Context) *table {
	if len(ctx) > 0 {
		return &table{
			DB:   db,
			Name: name,
			ctx:  ctx[0],
		}
	}
	return &table{
		DB:   db,
		Name: name,
		ctx:  context.Background(),
	}
}

// Reuse .
func (t *table) Reuse() *table {
	t.Cfg.Reuse = true
	return t
}

// Debug .
func (t *table) Debug() *table {
	t.Cfg.Debug = true
	return t
}

func (t *table) UseNameWhenTagEmpty() *table {
	t.Cfg.UseNameWhenTagEmpty = true
	return t
}

func (t *table) ReplaceInto() *table {
	t.Cfg.ReplaceInto = true
	return t
}

func (t *table) InsertIgnore() *table {
	t.Cfg.InsertIgnore = true
	return t
}

func (t *table) ToTimestamp() *table {
	t.Cfg.ToTimestamp = true
	return t
}

// Fields .
func Fields(fields ...string) *fieldsItem {
	return &fieldsItem{Fields: fields}
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
	return &onDuplicateKeyUpdateItem{KVs: keyVals}
}

func (t *table) Select(res interface{}, args ...ormItem) (int, error) {
	if len(args) <= 0 {
		return 0, errors.New("argument 3 cannot be omitted")
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
	// case reflect.Map:
	// TODO
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
		_, fileName, line, _ := runtime.Caller(1)
		item = loadFromCache(fileName, line)
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

		// struct类型
		if rtElem.Kind() == reflect.Struct {
			s := rtElem.(reflect2.StructType)
			if isArray {
				item.Elem = rtElem.New()
			} else {
				item.Elem = res
			}

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
				Val:  unsafe.Pointer(&res),
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
			_, fileName, line, _ := runtime.Caller(1)
			storeToCache(fileName, line, item)
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

func (t *table) Insert(objs interface{}, args ...ormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Insert", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	var (
		rt         = reflect2.TypeOf(objs)
		isArray    bool
		isPtrArray bool
		rtPtr      reflect2.Type
		rtElem     = rt

		sb       strings.Builder
		stmtArgs []interface{}
		cols     []reflect2.StructField
	)

	if t.Cfg.ReplaceInto {
		sb.WriteString("replace into ")
	} else {
		if t.Cfg.InsertIgnore {
			sb.WriteString("insert ignore into ")
		} else {
			sb.WriteString("insert into ")
		}
	}

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
	// case reflect.Map:
	// TODO
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

	if t.Cfg.Debug {
		log.Println(sb.String(), stmtArgs)
	}

	res, err := t.DB.ExecContext(t.ctx, sb.String(), stmtArgs...)
	if err != nil {
		return 0, err
	}

	row, _ := res.RowsAffected()
	return int(row), nil
}

func (t *table) Update(obj interface{}, args ...ormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Update", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	if len(args) <= 0 {
		return 0, errors.New("argument 3 cannot be omitted")
	}

	var sb strings.Builder
	sb.WriteString("update ")
	fieldEscape(&sb, t.Name)
	sb.WriteString(" set ")

	var stmtArgs []interface{}

	if m, ok := obj.(V); ok {
		if args[0].Type() == _fields {
			for _, field := range args[0].(*fieldsItem).Fields {
				v := m[field]
				if v != nil {
					if len(stmtArgs) > 0 {
						sb.WriteString(",")
					}
					fieldEscape(&sb, field)
					sb.WriteString("=?")
					stmtArgs = append(stmtArgs, v)
				}
			}

			args = args[1:]

		} else {
			for k, v := range m {
				if len(stmtArgs) > 0 {
					sb.WriteString(",")
				}
				fieldEscape(&sb, k)
				sb.WriteString("=?")
				stmtArgs = append(stmtArgs, v)
			}
		}
	} else {
		rt := reflect2.TypeOf(obj)

		switch rt.Kind() {
		case reflect.Ptr:
			rt = rt.(reflect2.PtrType).Elem()
		// case reflect.Map:
		// TODO
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
		if args[0].Type() == _fields {
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
	}

	for _, arg := range args {
		arg.BuildSQL(&sb)
		arg.BuildArgs(&stmtArgs)
	}

	if t.Cfg.Debug {
		log.Println(sb.String(), stmtArgs)
	}

	res, err := t.DB.ExecContext(t.ctx, sb.String(), stmtArgs...)
	if err != nil {
		return 0, err
	}

	row, _ := res.RowsAffected()
	return int(row), nil
}

func (t *table) Delete(args ...ormItem) (int, error) {
	if len(args) <= 0 {
		return 0, errors.New("argument 1 cannot be omitted")
	}

	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Delete", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	var sb strings.Builder
	sb.WriteString("delete from ")
	fieldEscape(&sb, t.Name)

	var stmtArgs []interface{}

	for _, arg := range args {
		arg.BuildSQL(&sb)
		arg.BuildArgs(&stmtArgs)
	}

	if t.Cfg.Debug {
		log.Println(sb.String(), stmtArgs)
	}

	res, err := t.DB.ExecContext(t.ctx, sb.String(), stmtArgs...)
	if err != nil {
		return 0, err
	}

	row, _ := res.RowsAffected()
	return int(row), nil
}

func (t *table) inputArgs(stmtArgs *[]interface{}, cols []reflect2.StructField, rtPtr, s reflect2.Type, ptr bool, x unsafe.Pointer) {
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

type dbIFace interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type table struct {
	DB   dbIFace
	Name string
	Cfg  Config
	ctx  context.Context
}

func fieldEscape(sb *strings.Builder, field string) {
	if field == "" {
		return
	}
	if strings.IndexAny(field, "( `") == -1 {
		sb.WriteString("`")
		sb.WriteString(field)
		sb.WriteString("`")
	} else {
		// TODO: 处理alias场景
		sb.WriteString(field)
	}
}

func (t *table) getStructFieldMap(s reflect2.StructType) map[string]reflect2.StructField {
	m := make(map[string]reflect2.StructField)
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("borm")

		if ft != "" {
			m[ft] = f
		} else if t.Cfg.UseNameWhenTagEmpty {
			m[f.Name()] = f
		}
	}
	return m
}

type ormItem interface {
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

type onDuplicateKeyUpdateItem struct {
	KVs V
}

func (w *onDuplicateKeyUpdateItem) Type() int {
	return _onDuplicateKeyUpdate
}

func (w *onDuplicateKeyUpdateItem) BuildSQL(sb *strings.Builder) {
	if len(w.KVs) <= 0 {
		return
	}
	sb.WriteString(" on duplicate key update ")
	i := 0
	for k := range w.KVs {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(sb, k)
		sb.WriteString("=?")
		i++
	}
}

func (w *onDuplicateKeyUpdateItem) BuildArgs(stmtArgs *[]interface{}) {
	for _, v := range w.KVs {
		*stmtArgs = append(*stmtArgs, v)
	}
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

func scanFromString(isTime bool, st reflect2.Type, dt reflect2.Type, ptrVal unsafe.Pointer, tmp string) error {
	dk := dt.Kind()

	// 时间格式(DATE/DATETIME) => number
	if isTime || (dk >= reflect.Int && dk <= reflect.Float64) {
		var year, month, day, hour, min, sec int
		n, _ := fmt.Sscanf(tmp, "%4d-%2d-%2d %2d:%2d:%2d", &year, &month, &day, &hour, &min, &sec)
		if n == 3 || n == 6 {
			if isTime {
				*(*time.Time)(ptrVal) = time.Unix(toUnix(year, month, day, hour, min, sec), 0)
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
		// 转时间类型
		if isTime {
			// 获取数值时间戳
			i64, err := strconv.ParseInt(tmp, 10, 64)
			if err != nil {
				return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
			}
			*(*time.Time)(ptrVal) = time.Unix(i64, 0)
		} else {
			// string => []byte
			if dk == reflect.Slice && dt.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
				*(*[]byte)(ptrVal) = reflect2.UnsafeCastString(tmp)
				return nil
			}
			// TODO 自定义类型，尝试转换
			return fmt.Errorf("converting driver.Value type %s (%s) to a %s", st.String(), tmp, dt.String())
		}
	}
	return nil
}

func (dest *scanner) Scan(src interface{}) error {
	var (
		st = reflect2.TypeOf(src)
		dt = dest.Type
		sk = st.Kind()
		dk = dt.Kind()
	)

	// NULL值
	if st.UnsafeIsNil(reflect2.PtrOf(src)) {
		// 设置成默认值，如果是指针，那么是空指针
		dt.UnsafeSet(dest.Val, dt.UnsafeNew())
		return nil
	}

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
		// TODO 自定义类型，尝试转换
		return fmt.Errorf("converting driver.Value type %T to a %s", src, dest.Type.String())
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
		// 单条不用in，用等于
		if arg, ok := args[0].([]interface{}); ok {
			args = arg
			goto RETRY
		} else {
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
	SQL  string
	Cols []interface{}
	Type reflect2.Type
	Elem interface{}
}

func storeToCache(file string, line int, item *DataBindingItem) {
	_dataBindingCache.Store(fmt.Sprintf("%s:%d", file, line), item)
	return
}

func loadFromCache(file string, line int) *DataBindingItem {
	if i, ok := _dataBindingCache.Load(fmt.Sprintf("%s:%d", file, line)); ok {
		return i.(*DataBindingItem)
	}
	return nil
}

var (
	_dataBindingCache sync.Map
)

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
