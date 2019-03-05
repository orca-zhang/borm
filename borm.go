/*
    borm is beautiful orm library in Go.

    MIT License

    Copyright (c) 2010-2019 <zhangwei01@bilibili.com>

    Permission is hereby granted, free of charge, to any person obtaining a copy
    of this software and associated documentation files (the "Software"), to deal
    in the Software without restriction, including without limitation the rights
    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
    copies of the Software, and to permit persons to whom the Software is
    furnished to do so, subject to the following conditions:

    The above copyright notice and this permission notice shall be included in all
    copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
    SOFTWARE.
*/
package borm

import (
    "database/sql"
    "sync"
    "strings"
    "fmt"
    "time"
    "reflect"
    "context"
    "errors"
    "unsafe"
    "strconv"
    "log"
    "runtime"
    "path"

    "github.com/modern-go/reflect2"
)

const (
    _fields = 1 >> iota
    _where
    _groupBy
    _orderBy
    _limit
    _onDuplicateKeyUpdate
)

const (
    _timeLayout = "2006-01-02 15:04:05"
)

const (
    _selectMask = _fields | _where | _groupBy | _orderBy | _limit
    _insertMask = _fields | _onDuplicateKeyUpdate
    _updateMask = _fields | _where
)

var config struct {
    Mock bool
}

type Config struct {
    Debug bool
    Reuse bool
    NameWhenTagEmpty bool

    ReplaceInto bool
    InsertIgnore bool
    ToTimestamp bool
}

func Table(db dbIFace, name string, cfg... Config) *table {
    if len(cfg) > 0 {
        return &table{
            DB: db,
            Name: name,
            Cfg: cfg[0],
        }
    }
    return &table{
        DB: db,
        Name: name,
    }
}

func (t *table) Debug() *table {
    t.Cfg.Debug = true
    return t
}

func Fields(fields... string) *fieldsItem {
    return &fieldsItem{Fields: fields}
}

func Where(conds... interface{}) *whereItem {
    if l := len(conds); l > 0 {
        if s, ok := conds[0].(string); ok {
            return &whereItem{Conds: []interface{}{
                &ormCond{
                    Op: s,
                    Args: conds[1:l],
                },
            }}
        }
        return &whereItem{Conds: conds}
    }
    panic("too few conditions")
}

func GroupBy(fields... string) *groupByItem {
    return &groupByItem{Fields: fields}
}

func OrderBy(orders... string) *orderByItem {
    return &orderByItem{Orders: orders}
}

func Limit(i... interface{}) *limitItem {
    switch len(i) {
    case 1, 2:
        return &limitItem{I: i}
    }
    panic("too few or too many limit params")
}

func OnDuplicateKeyUpdate(keyVals map[string]interface{}) *onDuplicateKeyUpdateItem {
    return &onDuplicateKeyUpdateItem{KVS: keyVals}
}

func (t *table) Select(ctx context.Context, res interface{}, args... interface{}) (int, error) {
    if len(args) <= 0 {
        return 0, errors.New("Argument 3 cannot be omited.")
    }

    var sb strings.Builder
    sb.WriteString("select ")

    ty := reflect2.TypeOf(res)
    k := ty.Kind()

    // 指针类型
    if k == reflect.Ptr {
        ty = ty.(reflect2.PtrType).Elem()
        k = ty.Kind()
    } else if k == reflect.Map {
        // TODO
    } else {
        return 0, errors.New("Argument 2 should be map or ptr.")
    }

    if config.Mock {
        pc, fileName, _, _ := runtime.Caller(1)
        if ok, data, n, e := checkMock(t.Name, "Select", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
            ty.UnsafeSet(reflect2.PtrOf(res), reflect2.PtrOf(data))
            return n, e
        }
    }

    var isArray bool
    var isPtrArray bool
    tyElem := ty
    // 是数组
    if k == reflect.Slice {
        // TODO
        tyElem = ty.(reflect2.ListType).Elem()
        k = tyElem.Kind()
        isArray = true

        if k == reflect.Ptr {
            tyElem = tyElem.(reflect2.PtrType).Elem()
            k = tyElem.Kind()
            isPtrArray = true
        }
    }

    var cols []interface{}
    var elem interface{}

    // struct类型
    if k == reflect.Struct {
        // sql和存储可复用，全局复用记得加锁或者切分存储
        s := tyElem.(reflect2.StructType)
        if isArray {
            elem = tyElem.New()
        } else {
            elem = res
        }

        if args[0].(ormItem).Type() == _fields {
            m := t.getStructFieldMap(s)

            for _, field := range args[0].(*fieldsItem).Fields {
                f := m[field]
                cols = append(cols, &scanner{
                    Type: f.Type(),
                    Val: f.Get(elem),
                })
            }

            (args[0]).(ormItem).BuildSQL(&sb)
            args = args[1:]

        } else {
            for i := 0; i < s.NumField(); i++ {
                f := s.Field(i)
                ft := f.Tag().Get("borm")

                if !t.Cfg.NameWhenTagEmpty && ft == "" {
                    continue
                }

                if len(cols) > 0 { sb.WriteString(",") }
    
                if ft == "" {
                    fieldEscape(&sb, f.Name())
                } else {
                    fieldEscape(&sb, ft)
                }

                cols = append(cols, &scanner{
                    Type: f.Type(),
                    Val: f.Get(elem),
                })
            }
        }
    // map类型
    } else if k == reflect.Map {
        // TODO
    // 其他类型
    } else {
        // 必须有fields且为1
        if args[0].(ormItem).Type() != _fields {
            return 0, errors.New("Argument 3 need ONE Fields(\"name\") with ONE field.")
        }

        fi := args[0].(*fieldsItem)
        if len(fi.Fields) < 1 {
            return 0, errors.New("Too few fields.")
        }

        cols = append(cols, &scanner{
            Type: tyElem,
            Val: res,
        })

        fieldEscape(&sb, fi.Fields[0])
        args = args[1:]
    }

    sb.WriteString(" from ")

    fieldEscape(&sb, t.Name)

    var stmtArgs []interface{}
    for _, arg := range args {
        (arg).(ormItem).BuildSQL(&sb)
        (arg).(ormItem).BuildArgs(&stmtArgs)
    }

    if t.Cfg.Debug {
        log.Println(sb.String(), stmtArgs)
    }

    if !isArray {
        // fire
        err := t.DB.QueryRowContext(ctx, sb.String(), stmtArgs...).Scan(cols...)
        if err != nil {
            if err == sql.ErrNoRows {
                return 0, nil
            }
            return 0, err
        }
        return 1, err
    } else {
        // fire
        rows, err := t.DB.QueryContext(ctx, sb.String(), stmtArgs...)
        if err != nil {
            return 0, err
        }

        defer rows.Close()

        count := 0
        for rows.Next() {
            err := rows.Scan(cols...)
            if err != nil {
                return 0, err
            }

            if isPtrArray {
                copyElem := tyElem.UnsafeNew()
                tyElem.UnsafeSet(copyElem, reflect2.PtrOf(elem))
                ty.(reflect2.SliceType).UnsafeAppend(reflect2.PtrOf(res), unsafe.Pointer(&copyElem))
            } else {
                ty.(reflect2.SliceType).UnsafeAppend(reflect2.PtrOf(res), reflect2.PtrOf(elem))
            }
            count++
        }
        return count, nil
    }
}

func (t *table) Insert(ctx context.Context, objs interface{}, args... interface{}) (int, error) {
    if config.Mock {
        pc, fileName, _, _ := runtime.Caller(1)
        if ok, _, n, e := checkMock(t.Name, "Insert", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
            return n, e
        }
    }

    var sb strings.Builder
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

    ty := reflect2.TypeOf(objs)
    k := ty.Kind()

    // 指针类型
    if k == reflect.Ptr {
        ty = ty.(reflect2.PtrType).Elem()
        k = ty.Kind()
    } else if k == reflect.Map {
        // TODO
    } else {
        return 0, errors.New("Argument 2 should be map or ptr.")
    }

    var isArray bool
    var isPtrArray bool

    tyElem := ty

    var sp reflect2.Type

    // 是数组
    if k == reflect.Slice {
        // TODO
        tyElem = ty.(reflect2.ListType).Elem()
        k = tyElem.Kind()
        isArray = true

        if k == reflect.Ptr {
            sp = tyElem
            tyElem = tyElem.(reflect2.PtrType).Elem()
            k = tyElem.Kind()
            isPtrArray = true
        }
    }

    var stmtArgs []interface{}
    var cols []reflect2.StructField

    // Fields or None
    // struct类型
    if k != reflect.Struct {
        return 0, errors.New("Non-struct type not supported yet.")
    }

    s := tyElem.(reflect2.StructType)
    if len(args) > 0 && args[0].(ormItem).Type() == _fields {
        m := t.getStructFieldMap(s)

        for _, field := range args[0].(*fieldsItem).Fields {    
            f := m[field]
            if f != nil {
                cols = append(cols, f)
            }
        }

        (args[0]).(ormItem).BuildSQL(&sb)
        args = args[1:]

    } else {
        for i := 0; i < s.NumField(); i++ {
            f := s.Field(i)
            ft := f.Tag().Get("borm")

            if !t.Cfg.NameWhenTagEmpty && ft == "" {
                continue
            }

            if len(cols) > 0 { sb.WriteString(",") }

            if ft == "" {
                fieldEscape(&sb, f.Name())
            } else {
                fieldEscape(&sb, ft)
            }

            cols = append(cols, f)
        }
    }

    sb.WriteString(") values ")

    var sbTmpl strings.Builder

    sbTmp := &sb
    if isArray {
        sbTmp = &sbTmpl
    }

    sbTmp.WriteString("(")
    for j := range cols {
        if j > 0 { sbTmp.WriteString(",") }
        sbTmp.WriteString("?")
    }
    sbTmp.WriteString(")")

    // inputArgs objs
    if isArray {
        // 数组
        for i := 0; i < ty.(reflect2.SliceType).UnsafeLengthOf(reflect2.PtrOf(objs)); i++ {
            if i > 0 { sb.WriteString(",") }
            sb.WriteString(sbTmp.String())
            t.inputArgs(&stmtArgs, cols, sp, s, isPtrArray, ty.(reflect2.ListType).UnsafeGetIndex(reflect2.PtrOf(objs), i))
        }
    } else {
        // 普通元素
        t.inputArgs(&stmtArgs, cols, sp, s, false, reflect2.PtrOf(objs))
    }

    // on duplicate key update
    for _, arg := range args {
        (arg).(ormItem).BuildSQL(&sb)
        (arg).(ormItem).BuildArgs(&stmtArgs)
    }

    if t.Cfg.Debug {
        log.Println(sb.String(), stmtArgs)
    }

    res, err := t.DB.ExecContext(ctx, sb.String(), stmtArgs...)
    if err != nil {
        return 0, err
    }

    row, err := res.RowsAffected()
    if err != nil {
        return 0, err
    }
    return int(row), nil
}

func (t *table) Update(ctx context.Context, obj interface{}, args... interface{}) (int, error) {
    if config.Mock {
        pc, fileName, _, _ := runtime.Caller(1)
        if ok, _, n, e := checkMock(t.Name, "Update", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
            return n, e
        }
    }

    if len(args) <= 0 {
        return 0, errors.New("Argument 3 cannot be omited.")
    }

    var sb strings.Builder
    sb.WriteString("update ")
    fieldEscape(&sb, t.Name)
    sb.WriteString(" set ")

    var stmtArgs []interface{}

    if m, ok := obj.(map[string]interface{}); ok {
        if args[0].(ormItem).Type() == _fields {    
            for _, field := range args[0].(*fieldsItem).Fields {    
                v := m[field]
                if v != nil {
                    if len(stmtArgs) > 0 {sb.WriteString(",")}
                    fieldEscape(&sb, field)
                    sb.WriteString("=?")
                    stmtArgs = append(stmtArgs, v)
                }
            }

            args = args[1:]

        } else {
            for k, v := range m {
                if len(stmtArgs) > 0 {sb.WriteString(",")}
                fieldEscape(&sb, k)
                sb.WriteString("=?")
                stmtArgs = append(stmtArgs, v)
            }
        }
    } else {
        ty := reflect2.TypeOf(obj)
        k := ty.Kind()

        // 指针类型
        if k == reflect.Ptr {
            ty = ty.(reflect2.PtrType).Elem()
            k = ty.Kind()
        } else if k == reflect.Map {
            // TODO
        } else {
            return 0, errors.New("Argument 2 should be map or ptr.")
        }

        var cols []reflect2.StructField

        // Fields or None
        // struct类型
        if k != reflect.Struct {
            return 0, errors.New("Non-struct type not supported yet.")
        }

        // Fields or KeyVals or None
        s := ty.(reflect2.StructType)
        if args[0].(ormItem).Type() == _fields {
            m := t.getStructFieldMap(s)
    
            for i, field := range args[0].(*fieldsItem).Fields {    
                f := m[field]
                if f != nil {
                    cols = append(cols, f)
                }
                
                if i > 0 {sb.WriteString(",")}
                fieldEscape(&sb, field)
                sb.WriteString("=?")
            }
    
            args = args[1:]
    
        } else {
            for i := 0; i < s.NumField(); i++ {
                f := s.Field(i)
                ft := f.Tag().Get("borm")
    
                if !t.Cfg.NameWhenTagEmpty && ft == "" {
                    continue
                }
    
                if len(cols) > 0 { sb.WriteString(",") }
    
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
        log.Println(sb.String(), stmtArgs)

        t.inputArgs(&stmtArgs, cols, ty, s, false, reflect2.PtrOf(obj))
    }

    for _, arg := range args {
        (arg).(ormItem).BuildSQL(&sb)
        (arg).(ormItem).BuildArgs(&stmtArgs)
    }

    if t.Cfg.Debug {
        log.Println(sb.String(), stmtArgs)
    }

    res, err := t.DB.ExecContext(ctx, sb.String(), stmtArgs...)
    if err != nil {
        return 0, err
    }

    row, err := res.RowsAffected()
    if err != nil {
        return 0, err
    }
    return int(row), nil
}

func (t *table) Delete(ctx context.Context, arg interface{}) (int, error) {
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
    (arg).(ormItem).BuildSQL(&sb)
    (arg).(ormItem).BuildArgs(&stmtArgs)

    if t.Cfg.Debug {
        log.Println(sb.String(), stmtArgs)
    }

    res, err := t.DB.ExecContext(ctx, sb.String(), stmtArgs...)
    if err != nil {
        return 0, err
    }

    row, err := res.RowsAffected()
    if err != nil {
        return 0, err
    }
    return int(row), nil
}

func (t *table) inputArgs(stmtArgs *[]interface{}, cols []reflect2.StructField, sp, s reflect2.Type, ptr bool, x unsafe.Pointer) {
    for _, col := range cols {
        var v interface{}
        if ptr {
            v = col.Get(sp.UnsafeIndirect(x))
        } else {
            v = col.Get(s.PackEFace(x))
        }

        // 时间类型特殊处理
        if col.Type().String() == "time.Time" {
            if !t.Cfg.ToTimestamp {
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
        } else if t.Cfg.NameWhenTagEmpty {
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
        if i > 0 {sb.WriteString(",")}
        fieldEscape(sb, field)
    }
}

func (w *fieldsItem) BuildArgs(stmtArgs *[]interface{}) {
}

type onDuplicateKeyUpdateItem struct {
    KVS map[string]interface{}
}

func (w *onDuplicateKeyUpdateItem) Type() int {
    return _onDuplicateKeyUpdate
}

func (w *onDuplicateKeyUpdateItem) BuildSQL(sb *strings.Builder) {
    sb.WriteString(" on duplicate key update ")
    i := 0
    for k := range w.KVS {
        if i > 0 {sb.WriteString(",")}
        fieldEscape(sb, k)
        sb.WriteString("=?")
        i++
    }
}

func (w *onDuplicateKeyUpdateItem) BuildArgs(stmtArgs *[]interface{}) {
    for _, v := range w.KVS {
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
    sb.WriteString(" where ")
    for i, c := range w.Conds {
        if i > 0 {
            sb.WriteString(" and ")
        }
        if cond, ok := c.(*ormCond); ok {
            fieldEscape(sb, cond.Field)
            sb.WriteString(cond.Op)
        }
    }
}

func (w *whereItem) BuildArgs(stmtArgs *[]interface{}) {
    for _, c := range w.Conds {
        if cond, ok := c.(*ormCond); ok {
            *stmtArgs = append(*stmtArgs, cond.Args...)
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
        if i > 0 {sb.WriteString(",")}
        fieldEscape(sb, field)
    }
}

func (w *groupByItem) BuildArgs(stmtArgs *[]interface{}) {
}

type orderByItem struct {
    Orders []string
}

func (w *orderByItem) Type() int {
    return _orderBy
}

func (w *orderByItem) BuildSQL(sb *strings.Builder) {
    sb.WriteString(" order by ")

    for i, order := range w.Orders {
        if i > 0 {sb.WriteString(",")}
        // TODO: 带升降序前的字段转义
        fieldEscape(sb, order)
    }
}

func (w *orderByItem) BuildArgs(stmtArgs *[]interface{}) {
}

type limitItem struct {
    I []interface{}
}

func (w *limitItem) Type() int {
    return _limit
}

func (w *limitItem) BuildSQL(sb *strings.Builder) {
    sb.WriteString(" limit ?")
    if len(w.I) > 1 {
        sb.WriteString(",?")
    }
}

func (w *limitItem) BuildArgs(stmtArgs *[]interface{}) {
    *stmtArgs = append(*stmtArgs, w.I...)
}

func strconvErr(err error) error {
	if ne, ok := err.(*strconv.NumError); ok {
		return ne.Err
	}
	return err
}

type scanner struct {
    Type reflect2.Type
    Val interface{}
}

func isNumber(k reflect.Kind) bool {
    return (k >= reflect.Int && k <= reflect.Float64)
}

func isTimeFormat(s string) bool {
    isNum := func(ch byte) bool {
        return ch >= '0' && ch <= '9'
    }
    switch len(s) {
    case 10, // DATE
        19: //DATETIME
        return s[ 4] == '-' && s[ 7] == '-' && 
            isNum(s[0]) && isNum(s[1]) && isNum(s[2]) && isNum(s[3]) && 
            isNum(s[5]) && isNum(s[6]) && isNum(s[8]) && isNum(s[9]) && 
        (len(s) == 10 ||  (s[10] == ' ' && isNum(s[11]) && isNum(s[12]) && 
                           s[13] == ':' && isNum(s[14]) && isNum(s[15]) && 
                           s[16] == ':' && isNum(s[17]) && isNum(s[18])))
    }
    return false
}

func (dest *scanner) Scan(src interface{}) error {
    st := reflect2.TypeOf(src)
    dk := dest.Type.Kind()
    sk := st.Kind()
    ptrVal := reflect2.PtrOf(dest.Val)

    // NULL值
    if st.UnsafeIsNil(reflect2.PtrOf(src)) {
        // 设置成默认值，如果是指针，那么是空指针
        dest.Type.UnsafeSet(ptrVal, dest.Type.UnsafeNew())
        return nil
    }

    // 相同类型，直接赋值
    if dk == sk {
        dest.Type.UnsafeSet(ptrVal, reflect2.PtrOf(src))
        return nil
    }

    pt, isTime := dest.Val.(*time.Time)
    // int64 => time.Time
    if sk == reflect.Int64 && isTime {
        *pt = time.Unix(src.(int64), 0)
        return nil
    }

    scanFromString := func(tmp string) error {
        // 时间格式(DATE/DATETIME) => number
        if (isTime || isNumber(dk)) && isTimeFormat(tmp) {
            layout := _timeLayout[0:len(tmp)]
            if isTime {
                *pt, _ = time.Parse(layout, tmp)
                return nil
            }
            t, _ := time.Parse(layout, tmp)
            switch dk {
            case reflect.Int:
                *(*int)(ptrVal) = int(t.Unix())
            case reflect.Int8:
                *(*int8)(ptrVal) = int8(t.Unix())
            case reflect.Int16:
                *(*int16)(ptrVal) = int16(t.Unix())
            case reflect.Int32:
                *(*int32)(ptrVal) = int32(t.Unix())
            case reflect.Int64:
                *(*int64)(ptrVal) = t.Unix()
            case reflect.Uint:
                *(*uint)(ptrVal) = uint(t.Unix())
            case reflect.Uint8:
                *(*uint8)(ptrVal) = uint8(t.Unix())
            case reflect.Uint16:
                *(*uint16)(ptrVal) = uint16(t.Unix())
            case reflect.Uint32:
                *(*uint32)(ptrVal) = uint32(t.Unix())
            case reflect.Uint64:
                *(*uint64)(ptrVal) = uint64(t.Unix())
            case reflect.Float32:
                *(*float32)(ptrVal) = float32(t.Unix())
            case reflect.Float64:
                *(*float64)(ptrVal) = float64(t.Unix())
            }
            return nil
        }

        // 非时间格式
        switch dk {
        case reflect.Bool:
            *(*bool)(ptrVal) = (tmp == "true")
        case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
            i64, err := strconv.ParseInt(tmp, 10, dest.Type.Type1().Bits())
            if err != nil {
                return fmt.Errorf("converting driver.Value type %T (%s) to a %s: %v", src, tmp, dk, strconvErr(err))
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
            }
        case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
            u64, err := strconv.ParseUint(tmp, 10, dest.Type.Type1().Bits())
            if err != nil {
                return fmt.Errorf("converting driver.Value type %T (%s) to a %s: %v", src, tmp, dk, strconvErr(err))
            }
            switch dk {
            case reflect.Uint:
                *(*uint)(ptrVal) = uint(u64)
            case reflect.Uint8:
                *(*uint8)(ptrVal) = uint8(u64)
            case reflect.Uint16:
                *(*uint16)(ptrVal) = uint16(u64)
            case reflect.Uint32:
                *(*uint32)(ptrVal) = uint32(u64)
            case reflect.Uint64:
                *(*uint64)(ptrVal) = u64
            }
        case reflect.Float32, reflect.Float64:
            f64, err := strconv.ParseFloat(tmp, dest.Type.Type1().Bits())
            if err != nil {
                return fmt.Errorf("converting driver.Value type %T (%s) to a %s: %v", src, tmp, dk, strconvErr(err))
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
                    return fmt.Errorf("converting driver.Value type %T (%s) to a %s: %v", src, tmp, dk, strconvErr(err))
                }
                *pt = time.Unix(i64, 0)
            } else {
                // string => []byte
                if dk == reflect.Slice && dest.Type.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
                    *(*[]byte)(ptrVal) = reflect2.UnsafeCastString(tmp)
                    return nil
                }
                // TODO 自定义类型，尝试转换
                return fmt.Errorf("converting driver.Value type %T (%s) to a %s", src, tmp, dest.Type.String())
            }
        }
        return nil
    }

    if sk == reflect.String {
        return scanFromString(src.(string))
    } else if sk == reflect.Slice && st.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
        return scanFromString(string(src.([]byte)))
    }

    numberToString := func(k reflect.Kind, src interface{}) string {
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
    
    switch dk {
    case reflect.Bool:
        switch sk {
        case reflect.Int64:
            *(*bool)(ptrVal) = (src.(int64) != 0)
        case reflect.Float64:
            *(*bool)(ptrVal) = (src.(float64) != 0)
        }
    case reflect.Int64:
        switch sk {
        case reflect.Bool:
            if src.(bool) {
                *(*int64)(ptrVal) = int64(1)
            } else {
                *(*int64)(ptrVal) = int64(0)
            }
        case reflect.Float64:
            *(*int64)(ptrVal) = int64(src.(float64))
        }
    case reflect.Float64:
        switch sk {
        case reflect.Bool:
            if src.(bool) {
                *(*float64)(ptrVal) = float64(1)
            } else {
                *(*float64)(ptrVal) = float64(0)
            }
        case reflect.Int64:
            *(*float64)(ptrVal) = float64(src.(int64))
        }
    case reflect.String:
        *(*string)(ptrVal) = numberToString(sk, src)
    default:
        // number => []byte
        if dk == reflect.Slice && dest.Type.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
            *(*[]byte)(ptrVal) = reflect2.UnsafeCastString(numberToString(sk, src))
            return nil
        }
        // TODO 自定义类型，尝试转换
        return fmt.Errorf("converting driver.Value type %T to a %s", src, dest.Type.String())
    }
    return nil
}

type ormCond struct {
    Field string
    Op string
    Args []interface{}
}

/*
    条件逻辑运算
*/
func Cond(c string, args... interface{}) *ormCond {
    return &ormCond{Op:c, Args: args}
}

func Eq(field string, i interface{}) *ormCond {
    return &ormCond{Field:field, Op: "=?", Args: []interface{}{i}}
}

func Neq(field string, i interface{}) *ormCond {
    return &ormCond{Field:field, Op: "<>?", Args: []interface{}{i}}
}

func Gt(field string, i interface{}) *ormCond {
    return &ormCond{Field:field, Op: ">?", Args: []interface{}{i}}
}

func Gte(field string, i interface{}) *ormCond {
    return &ormCond{Field:field, Op: ">=?", Args: []interface{}{i}}
}

func Lt(field string, i interface{}) *ormCond {
    return &ormCond{Field:field, Op: "<?", Args: []interface{}{i}}
}

func Lte(field string, i interface{}) *ormCond {
    return &ormCond{Field:field, Op: "<=?", Args: []interface{}{i}}
}

func Between(field string, i interface{}, j interface{}) *ormCond {
    return &ormCond{Field:field, Op: " between ? and ?", Args: []interface{}{i, j}}
}

func Like(field string, pattern string) *ormCond {
    return &ormCond{Field:field, Op: " like ", Args: []interface{}{pattern}}
}

func In(field string, args... interface{}) *ormCond {
    if len(args) <= 0 {
        return &ormCond{}
    }

    var sb strings.Builder
    sb.WriteString(" in (")
    for i := 0; i < len(args); i++ {
        if i > 0 {sb.WriteString(",")}
        sb.WriteString("?")
    }
    sb.WriteString(")")
    return &ormCond{Field:field, Op: sb.String(), Args: args}
}

/*
    Mock相关
*/
var (
    _mockData []*MockMatcher
    _mutex sync.Mutex
)

func matchString(src string, matcher string, caseSens bool) bool {
    if matcher == "" {
        return true
    }

    isAlpha := func(x byte) bool {
        return x >= 0x61 && x <= 0x7A
    }

    caseSensEq := func(x byte, y byte, cs bool) bool {
        return (y == '?' || x == y || (!cs && isAlpha(x | 0x20) && (x ^ y) == 0x20))
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
            for j < ml && matcher[j] == '*' {j++}
            // forward to first match char of src
            for i < len(src) && (j >= ml || !caseSensEq(src[i], matcher[j], caseSens)) {i++}
            // record current position for back-track
            s = append(s, []int{i+1, j-1})
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
    for j < ml && matcher[j] == '*' {j++}
    return i == len(src) && j == ml
}

type MockMatcher struct {
    Tbl string
    Func string
    Caller string
    File string
    Pkg  string
    Data interface{}
    Ret  int
    Err  error
}

func checkMock(tbl, fun, caller, file, pkg string) (mocked bool, data interface{}, ret int, err error) {
    _mutex.Lock()
    defer _mutex.Unlock()

    for i := 0; i < len(_mockData) ; i++ {
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

func BormMock(tbl, fun, caller, file, pkg string, data interface{}, ret int, err error) {
    _, fileName, _, _ := runtime.Caller(1)
    if !strings.HasSuffix(fileName, "_test.go") {
        panic("DONT USE THIS FUNCTION IN PRODUCTION ENVIRONMENT!")
    }

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

func BormMockFinish() error {
    _mutex.Lock()
    defer _mutex.Unlock()
    
    mockData := _mockData
    _mockData = make([]*MockMatcher, 0)
    if len(mockData) > 0 {
        return errors.New(fmt.Sprintf("Some of the mock data left behind: %+v", mockData))
    }
    return nil
}
