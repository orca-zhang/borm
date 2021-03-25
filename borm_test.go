package borm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	_ "github.com/go-sql-driver/mysql"
	"github.com/modern-go/reflect2"
	. "github.com/smartystreets/goconvey/convey"
)

var (
	db *sql.DB
)

func init() {
	var err error
	// db, err = sql.Open("mysql", "root:@tcp(localhost:3306)/borm_test?charset=utf8mb4")
	db, err = sql.Open("mysql", "root:semaphoredb@tcp(localhost:3306)/borm_test?charset=utf8mb4")
	if err != nil {
		log.Fatal(err)
	}
}

type x struct {
	X  string    `borm:"name"`
	Y  int64     `borm:"age"`
	Z  time.Time `borm:"ctime4"`
	Z1 int64     `borm:"ctime"`
	Z2 int64     `borm:"ctime2"`
	Z3 int64     `borm:"ctime3"`
}

type xx struct {
	BormLastId int64
	X          string `borm:"name"`
	Y          int64  `borm:"age"`
}

type x1 struct {
	X     string `borm:"name"`
	ctime int64
}

func (x *x1) CTime() int64 {
	return x.ctime
}

type c struct {
	C int64 `borm:"count(1)"`
}

func BenchmarkBormSelect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var o []x
		tbl := Table(db, "test").Reuse()
		tbl.Select(&o, Where("`id` >= 1"))
	}
}

func BenchmarkNormalSelect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var o []*x
		rows, _ := db.QueryContext(context.TODO(), "select `name`,`age`,`ctime4`,`ctime`,`ctime2`,`ctime3` from `test` where `id` >= 1")

		for rows.Next() {
			var t x
			var ctime4 string
			rows.Scan(&t.X, &t.Y, &ctime4, &t.Z1, &t.Z2, &t.Z3)
			t.Z, _ = time.Parse("2006-01-02 15:04:05", ctime4)
			o = append(o, &t)
		}

		rows.Close()
	}
}

func TestForceIndex(t *testing.T) {
	Convey("normal", t, func() {
		So(ForceIndex("idx_ctime").Type(), ShouldEqual, _forceIndex)

		var ids []int64
		tbl := Table(db, "test").Debug()

		n, err := tbl.Select(&ids, Fields("id"), ForceIndex("idx_ctime"), Limit(100))

		So(err, ShouldBeNil)
		So(n, ShouldBeGreaterThan, 1)
		So(len(ids), ShouldBeGreaterThan, 1)
	})
}

func TestSelect(t *testing.T) {

	Convey("normal", t, func() {

		Convey("single select", func() {
			var o x
			tbl := Table(db, "test").Reuse()

			for i := 0; i < 10; i++ {
				n, err := tbl.Select(&o, Where(Cond("`id` >= ?", 1)), GroupBy("id"), Having("id>=?", 0), Limit(100))

				So(err, ShouldBeNil)
				So(n, ShouldEqual, 1)
				fmt.Printf("%+v\n", o)
			}
		})

		Convey("multiple select", func() {
			var o []x
			tbl := Table(db, "test").Debug()

			n, err := tbl.Select(&o, Where(Gte("id", 0), Lte("id", 1000), Between("id", 0, 1000)), OrderBy("id", "name"), Limit(0, 100))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 2)
			fmt.Printf("%+v\n", o)
		})

		Convey("multiple select with pointer", func() {
			var o []*x
			tbl := Table(db, "test").Debug()

			n, err := tbl.Select(&o, Where(In("id", []interface{}{1, 2, 3, 4}...)))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 2)

			for _, v := range o {
				fmt.Printf("%+v\n", v)
			}
		})

		Convey("counter", func() {
			var o c
			tbl := Table(db, "test").Debug()

			n, err := tbl.Select(&o, GroupBy("id", `name`), Having(Gt("id", 0), Neq("name", "")), Limit(100))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)

			fmt.Printf("%+v\n", o)
		})

		Convey("user-defined fields", func() {
			var o x
			tbl := Table(db, "test").Debug()

			n, err := tbl.Select(&o, Fields("name", "ctime", "age"), Where("`id` >= ?", 1), Limit(100))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
			fmt.Printf("%+v\n", o)
		})

		Convey("user-defined fields with simple type", func() {
			var cnt int64
			tbl := Table(db, "test").Debug()

			n, err := tbl.Select(&cnt, Fields("count(1)"), Where(Eq("id", 1)), Limit(100))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
			So(n, ShouldBeGreaterThan, 0)
		})

		Convey("user-defined fields with simple slice type", func() {
			var ids []int64
			tbl := Table(db, "test").Debug()

			n, err := tbl.Select(&ids, Fields("id"), Limit(100))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 1)
			So(len(ids), ShouldBeGreaterThan, 1)
		})
	})
}

func TestInsert(t *testing.T) {

	Convey("normal", t, func() {

		Convey("single insert", func() {
			o := x{
				X:  "Orca1",
				Y:  20,
				Z1: 1551405784,
			}
			tbl := Table(db, "test").Debug()

			n, err := tbl.Insert(&o)

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
		})

		Convey("single insert ToTimestamp", func() {
			o := x{
				X:  "Orca1",
				Y:  20,
				Z1: 1551405784,
			}
			tbl := Table(db, "test").Debug().ToTimestamp()

			n, err := tbl.Insert(&o)

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
		})

		Convey("single replace", func() {
			o := x{
				X:  "Orca1",
				Y:  20,
				Z1: 1551405784,
			}
			tbl := Table(db, "test").Debug()

			n, err := tbl.ReplaceInto(&o)

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
		})

		Convey("multiple insert with ignore", func() {
			o := []*x{
				{
					X:  "Orca4",
					Y:  23,
					Z1: 1551405784,
				},
				{
					X:  "Orca5",
					Y:  24,
					Z1: 1551405784,
				},
			}
			tbl := Table(db, "test").Debug()

			n, err := tbl.InsertIgnore(&o)

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 2)
		})

		Convey("user-defined fields", func() {
			o := x{
				X:  "Orca1",
				Y:  20,
				Z1: 1551405784,
			}
			tbl := Table(db, "test").Debug()

			n, err := tbl.Insert(&o, Fields("name", "ctime", "age"))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
		})

		Convey("on duplicate key update", func() {
			o := x{
				X:  "Orca1",
				Y:  20,
				Z1: 1551405784,
			}
			tbl := Table(db, "test").Debug()

			n, err := tbl.Insert(&o, Fields("name", "ctime", "age"), OnDuplicateKeyUpdate(V{
				"name": "OnDuplicateKeyUpdate",
				"age":  29,
			}))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
		})

		Convey("on duplicate key update with U", func() {
			o := x{
				X:  "Orca1",
				Y:  20,
				Z1: 1551405784,
			}
			tbl := Table(db, "test").Debug()

			n, err := tbl.Insert(&o, Fields("name", "ctime", "age"), OnDuplicateKeyUpdate(V{
				"name": "OnDuplicateKeyUpdate",
				"age":  U("age+1"),
			}))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
		})
	})

	Convey("get last insert id", t, func() {

		Convey("single insert", func() {
			o := xx{
				X: "OrcaZ",
				Y: 30,
			}
			tbl := Table(db, "test2").Debug()

			n, err := tbl.Insert(&o)

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
			So(o.BormLastId, ShouldBeGreaterThan, 0)
		})
	})
}

func TestUpdate(t *testing.T) {

	Convey("normal", t, func() {

		Convey("update", func() {
			o := x{
				X:  "Orca1",
				Y:  20,
				Z1: 1551405784,
			}
			tbl := Table(db, "test").Debug()

			n, err := tbl.Update(&o, Where("id = ?", 0))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})

		Convey("update with map", func() {
			tbl := Table(db, "test").Debug()

			n, err := tbl.Update(V{
				"name": "OrcaUpdated",
				"age":  88,
			}, Where("id = ?", 0))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})

		Convey("update with U", func() {
			tbl := Table(db, "test").Debug()

			n, err := tbl.Update(V{
				"age": U("age+1"),
			}, Where("id = ?", 0))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})

		Convey("update with map & Fields", func() {
			tbl := Table(db, "test").Debug()

			n, err := tbl.Update(V{
				"name": "OrcaUpdatedFields",
				"age":  88,
			}, Fields("name", "age"), Where("id = ?", 0))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})

		Convey("update with U & Fields", func() {
			tbl := Table(db, "test").Debug()

			n, err := tbl.Update(V{
				"name": "OrcaUpdatedFields",
				"age":  U("age+1"),
			}, Fields("age"), Where("id = ?", 0))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})

		Convey("update with user-defined fields", func() {
			o := x{
				X:  "Orca1",
				Y:  20,
				Z1: 1551405784,
			}
			tbl := Table(db, "test").Debug()

			n, err := tbl.Update(&o, Fields("name", "ctime", "age"), Where("id = ?", 0))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})
	})
}

func TestDelete(t *testing.T) {

	Convey("normal", t, func() {

		Convey("single delete", func() {
			tbl := Table(db, "test").Debug()

			n, err := tbl.Delete(Where("`id`=0"), Limit(1))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})

		Convey("bulk delete", func() {
			tbl := Table(db, "test").Debug()

			n, err := tbl.Delete(Where("`id`=0"))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})
	})
}

func TestScanner(t *testing.T) {
	/*
		// The src value will be of one of the following types:
		//
		//    int64
		//    float64
		//    bool
		//    []byte
		//    string
		//    time.Time  => []byte with Format
		//    nil - for NULL values
	*/

	Convey("nil", t, func() {
		Convey("nil to nil", func() {
			a := 1
			ptr := &a
			nilScanner := scanner{
				Type: reflect2.TypeOf(ptr),
				Val:  unsafe.Pointer(&ptr),
			}

			var ptr1 *int
			err := nilScanner.Scan(ptr1)
			log.Println(ptr)
			So(err, ShouldBeNil)
			So(ptr, ShouldEqual, nil)
		})

		Convey("nil to bool", func() {
			/* bool */
			b := true
			boolScanner := scanner{
				Type: reflect2.TypeOf(b),
				Val:  unsafe.Pointer(&b),
			}

			err := boolScanner.Scan(nil)
			So(err, ShouldBeNil)
			So(b, ShouldEqual, false)
		})

		Convey("nil to int64", func() {
			/* int64 */
			i := int64(1)
			int64Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int64Scanner.Scan(nil)
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 0)
		})

		Convey("nil to string", func() {
			/* string */
			s := "xxxx"
			stringScanner := scanner{
				Type: reflect2.TypeOf(s),
				Val:  unsafe.Pointer(&s),
			}

			err := stringScanner.Scan(nil)
			So(err, ShouldBeNil)
			So(s, ShouldEqual, "")
		})

		Convey("nil to []byte", func() {
			/* []byte */
			bs := []byte{byte(1)}
			bytesScanner := scanner{
				Type: reflect2.TypeOf(bs),
				Val:  unsafe.Pointer(&bs),
			}

			err := bytesScanner.Scan(nil)
			So(err, ShouldBeNil)
			So(bs, ShouldResemble, []byte(nil))
		})

		Convey("nil to time.Time", func() {
			/* time */
			t := time.Now()
			stringScanner := scanner{
				Type: reflect2.TypeOf(t),
				Val:  unsafe.Pointer(&t),
			}
			var emptyTime time.Time

			err := stringScanner.Scan(nil)
			So(err, ShouldBeNil)
			So(t, ShouldEqual, emptyTime)
		})
	})

	Convey("bool", t, func() {
		Convey("bool to bool", func() {
			/* bool */
			b := false
			boolScanner := scanner{
				Type: reflect2.TypeOf(b),
				Val:  unsafe.Pointer(&b),
			}

			err := boolScanner.Scan(bool(true))
			So(err, ShouldBeNil)
			So(b, ShouldEqual, true)
		})

		Convey("int64 to bool", func() {
			/* bool */
			b := false
			boolScanner := scanner{
				Type: reflect2.TypeOf(b),
				Val:  unsafe.Pointer(&b),
			}

			err := boolScanner.Scan(int64(23))
			So(err, ShouldBeNil)
			So(b, ShouldEqual, true)
		})

		Convey("float64(false) to bool", func() {
			/* bool */
			b := false
			boolScanner := scanner{
				Type: reflect2.TypeOf(b),
				Val:  unsafe.Pointer(&b),
			}

			err := boolScanner.Scan(float64(0.0))
			So(err, ShouldBeNil)
			So(b, ShouldEqual, false)
		})

		Convey("float64 to bool", func() {
			/* bool */
			b := false
			boolScanner := scanner{
				Type: reflect2.TypeOf(b),
				Val:  unsafe.Pointer(&b),
			}

			err := boolScanner.Scan(float64(23.999))
			So(err, ShouldBeNil)
			So(b, ShouldEqual, true)
		})
	})

	Convey("int64", t, func() {
		Convey("bool to int64", func() {
			/* int64 */
			i := int64(0)
			int64Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int64Scanner.Scan(bool(true))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("bool(false) to int64", func() {
			/* int64 */
			i := int64(0)
			int64Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int64Scanner.Scan(bool(false))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 0)
		})

		Convey("int64 to int64", func() {
			/* int64 */
			i := int64(0)
			int64Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int64Scanner.Scan(int64(123))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("float64 to int64", func() {
			/* int64 */
			i := int64(0)
			int64Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int64Scanner.Scan(float64(123.33))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})
	})

	Convey("integer", t, func() {
		Convey("int64 to int", func() {
			/* int */
			i := int(0)
			intScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := intScanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("int64 to uint", func() {
			/* uint */
			i := uint(0)
			uintScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := uintScanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("int64 to int8", func() {
			/* int8 */
			i := int8(0)
			int8Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int8Scanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("int64 to uint8", func() {
			/* uint8 */
			i := uint8(0)
			uint8Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := uint8Scanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("int64 to int16", func() {
			/* int16 */
			i := int16(0)
			int16Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int16Scanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("int64 to uint16", func() {
			/* uint16 */
			i := uint16(0)
			uint16Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := uint16Scanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("int64 to int32", func() {
			/* int32 */
			i := int32(0)
			int32Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int32Scanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("int64 to uint32", func() {
			/* uint32 */
			i := uint32(0)
			uint32Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := uint32Scanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("int64 to uint64", func() {
			/* uint64 */
			i := uint64(0)
			uint64Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := uint64Scanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("int64 to float32", func() {
			/* float32 */
			i := float32(0)
			float32Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := float32Scanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("int64 to float64", func() {
			/* float64 */
			i := float64(0)
			float64Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := float64Scanner.Scan(int64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})
	})

	Convey("floating", t, func() {
		Convey("float to int", func() {
			/* int */
			i := int(0)
			intScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := intScanner.Scan(float64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("float to uint", func() {
			/* uint */
			i := uint(0)
			uintScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := uintScanner.Scan(float64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("float to int8", func() {
			/* int8 */
			i := int8(0)
			int8Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int8Scanner.Scan(float64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("float to uint8", func() {
			/* uint8 */
			i := uint8(0)
			uint8Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := uint8Scanner.Scan(float64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("float to int16", func() {
			/* int16 */
			i := int16(0)
			int16Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int16Scanner.Scan(float64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("float to uint16", func() {
			/* uint16 */
			i := uint16(0)
			uint16Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := uint16Scanner.Scan(float64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("float to int32", func() {
			/* int32 */
			i := int32(0)
			int32Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := int32Scanner.Scan(float64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("float to uint32", func() {
			/* uint32 */
			i := uint32(0)
			uint32Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := uint32Scanner.Scan(float64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})

		Convey("float to uint64", func() {
			/* uint64 */
			i := uint64(0)
			uint64Scanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := uint64Scanner.Scan(float64(1))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1)
		})
	})

	Convey("float64", t, func() {
		Convey("bool to float64", func() {
			/* float64 */
			f := float64(0)
			float64Scanner := scanner{
				Type: reflect2.TypeOf(f),
				Val:  unsafe.Pointer(&f),
			}

			err := float64Scanner.Scan(bool(true))
			So(err, ShouldBeNil)
			So(f, ShouldEqual, 1)
		})

		Convey("bool(false) to float64", func() {
			/* float64 */
			f := float64(0)
			float64Scanner := scanner{
				Type: reflect2.TypeOf(f),
				Val:  unsafe.Pointer(&f),
			}

			err := float64Scanner.Scan(bool(false))
			So(err, ShouldBeNil)
			So(f, ShouldEqual, 0)
		})

		Convey("int64 to float64", func() {
			/* float64 */
			f := float64(0)
			float64Scanner := scanner{
				Type: reflect2.TypeOf(f),
				Val:  unsafe.Pointer(&f),
			}

			err := float64Scanner.Scan(int64(123))
			So(err, ShouldBeNil)
			So(f, ShouldEqual, 123)
		})

		Convey("float64 to float64", func() {
			/* float64 */
			f := float64(0)
			float64Scanner := scanner{
				Type: reflect2.TypeOf(f),
				Val:  unsafe.Pointer(&f),
			}

			err := float64Scanner.Scan(float64(123.33))
			So(err, ShouldBeNil)
			So(f, ShouldEqual, 123.33)
		})
	})

	Convey("string", t, func() {
		Convey("bool to string", func() {
			/* string */
			s := ""
			stringScanner := scanner{
				Type: reflect2.TypeOf(s),
				Val:  unsafe.Pointer(&s),
			}

			err := stringScanner.Scan(bool(true))
			So(err, ShouldBeNil)
			So(s, ShouldEqual, "true")
		})

		Convey("bool(false) to string", func() {
			/* string */
			s := ""
			stringScanner := scanner{
				Type: reflect2.TypeOf(s),
				Val:  unsafe.Pointer(&s),
			}

			err := stringScanner.Scan(bool(false))
			So(err, ShouldBeNil)
			So(s, ShouldEqual, "false")
		})

		Convey("int64 to string", func() {
			/* string */
			s := ""
			stringScanner := scanner{
				Type: reflect2.TypeOf(s),
				Val:  unsafe.Pointer(&s),
			}

			err := stringScanner.Scan(int64(123))
			So(err, ShouldBeNil)
			So(s, ShouldEqual, "123")
		})

		Convey("float64 to string", func() {
			/* float64 */
			s := ""
			stringScanner := scanner{
				Type: reflect2.TypeOf(s),
				Val:  unsafe.Pointer(&s),
			}

			err := stringScanner.Scan(float64(123.33))
			So(err, ShouldBeNil)
			So(s, ShouldEqual, "123.33")
		})

		Convey("string to string", func() {
			/* string */
			s := "xxx"
			stringScanner := scanner{
				Type: reflect2.TypeOf(s),
				Val:  unsafe.Pointer(&s),
			}

			err := stringScanner.Scan(string("123.33"))
			So(err, ShouldBeNil)
			So(s, ShouldEqual, "123.33")
		})

		Convey("[]byte to string", func() {
			/* string */
			s := "xxx"
			stringScanner := scanner{
				Type: reflect2.TypeOf(s),
				Val:  unsafe.Pointer(&s),
			}

			err := stringScanner.Scan([]byte("123.33"))
			So(err, ShouldBeNil)
			So(s, ShouldEqual, "123.33")
		})
	})

	Convey("[]byte", t, func() {
		Convey("bool(false) to []byte", func() {
			/* []byte */
			var bs []byte
			bytesScanner := scanner{
				Type: reflect2.TypeOf(bs),
				Val:  unsafe.Pointer(&bs),
			}

			err := bytesScanner.Scan(bool(false))
			So(err, ShouldBeNil)
			So(bs, ShouldResemble, []byte("false"))
		})

		Convey("bool to []byte", func() {
			/* []byte */
			var bs []byte
			bytesScanner := scanner{
				Type: reflect2.TypeOf(bs),
				Val:  unsafe.Pointer(&bs),
			}

			err := bytesScanner.Scan(bool(true))
			So(err, ShouldBeNil)
			So(bs, ShouldResemble, []byte("true"))
		})

		Convey("int64 to []byte", func() {
			/* []byte */
			var bs []byte
			bytesScanner := scanner{
				Type: reflect2.TypeOf(bs),
				Val:  unsafe.Pointer(&bs),
			}

			err := bytesScanner.Scan(int64(2233))
			So(err, ShouldBeNil)
			So(bs, ShouldResemble, []byte("2233"))
		})

		Convey("float64 to []byte", func() {
			/* []byte */
			var bs []byte
			bytesScanner := scanner{
				Type: reflect2.TypeOf(bs),
				Val:  unsafe.Pointer(&bs),
			}

			err := bytesScanner.Scan(float64(22.33))
			So(err, ShouldBeNil)
			So(bs, ShouldResemble, []byte("22.33"))
		})

		Convey("string to []byte", func() {
			/* []byte */
			var bs []byte
			bytesScanner := scanner{
				Type: reflect2.TypeOf(bs),
				Val:  unsafe.Pointer(&bs),
			}

			err := bytesScanner.Scan(string("xxxxxx"))
			So(err, ShouldBeNil)
			So(bs, ShouldResemble, []byte("xxxxxx"))
		})
	})

	Convey("string to number", t, func() {
		Convey("string to bool", func() {
			/* bool */
			b := false
			stringScanner := scanner{
				Type: reflect2.TypeOf(b),
				Val:  unsafe.Pointer(&b),
			}

			err := stringScanner.Scan(string("true"))
			So(err, ShouldBeNil)
			So(b, ShouldEqual, true)
		})

		Convey("string to bool(false)", func() {
			/* bool */
			b := true
			stringScanner := scanner{
				Type: reflect2.TypeOf(b),
				Val:  unsafe.Pointer(&b),
			}

			err := stringScanner.Scan(string("false"))
			So(err, ShouldBeNil)
			So(b, ShouldEqual, false)
		})

		Convey("string to int", func() {
			/* int */
			i := int(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("string to int8", func() {
			/* int8 */
			i := int8(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("string to int16", func() {
			/* int16 */
			i := int16(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("string to int32", func() {
			/* int32 */
			i := int32(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("string to int64", func() {
			/* int64 */
			i := int64(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("string to uint", func() {
			/* uint */
			i := uint(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("string to uint8", func() {
			/* uint8 */
			i := uint8(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("string to uint16", func() {
			/* uint16 */
			i := uint16(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("string to uint32", func() {
			/* uint32 */
			i := uint32(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("string to uint64", func() {
			/* uint64 */
			i := uint64(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 123)
		})

		Convey("string to float32", func() {
			/* float32 */
			f := float32(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(f),
				Val:  unsafe.Pointer(&f),
			}

			err := stringScanner.Scan(string("123.33"))
			So(err, ShouldBeNil)
			So(f, ShouldEqual, 123.33)
		})

		Convey("string to float64", func() {
			/* float64 */
			f := float64(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(f),
				Val:  unsafe.Pointer(&f),
			}

			err := stringScanner.Scan(string("123.33"))
			So(err, ShouldBeNil)
			So(f, ShouldEqual, 123.33)
		})

		Convey("bad string to int64", func() {
			/* int64 */
			i := int64(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123abc"))
			So(err, ShouldNotBeNil)
		})

		Convey("bad string to uint64", func() {
			/* uint64 */
			i := uint64(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("123abc"))
			So(err, ShouldNotBeNil)
		})

		Convey("bad string to float64", func() {
			/* float64 */
			f := float64(0)
			stringScanner := scanner{
				Type: reflect2.TypeOf(f),
				Val:  unsafe.Pointer(&f),
			}

			err := stringScanner.Scan(string("123.33abc"))
			So(err, ShouldNotBeNil)
		})
	})

	Convey("time.Time", t, func() {
		Convey("int64 to time.Time", func() {
			/* time */
			var t time.Time
			stringScanner := scanner{
				Type: reflect2.TypeOf(t),
				Val:  unsafe.Pointer(&t),
			}

			err := stringScanner.Scan(int64(1551405784))
			So(err, ShouldBeNil)
			So(t.Unix(), ShouldEqual, time.Date(2019, 3, 1, 2, 3, 4, 0, time.UTC).Unix())
		})

		Convey("[]byte (DATE) to time.Time", func() {
			/* time */
			var t time.Time
			stringScanner := scanner{
				Type: reflect2.TypeOf(t),
				Val:  unsafe.Pointer(&t),
			}

			err := stringScanner.Scan([]byte("2019-03-01"))
			So(err, ShouldBeNil)
			So(t.Unix(), ShouldEqual, time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix())
		})

		Convey("[]byte (DATETIME) to time.Time", func() {
			/* time */
			var t time.Time
			stringScanner := scanner{
				Type: reflect2.TypeOf(t),
				Val:  unsafe.Pointer(&t),
			}

			err := stringScanner.Scan([]byte("2019-03-01 13:05:59"))
			So(err, ShouldBeNil)
			So(t.Unix(), ShouldEqual, time.Date(2019, 3, 1, 13, 5, 59, 0, time.UTC).Unix())
		})

		Convey("string (DATE) to time.Time", func() {
			/* time */
			var t time.Time
			stringScanner := scanner{
				Type: reflect2.TypeOf(t),
				Val:  unsafe.Pointer(&t),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(t.Unix(), ShouldEqual, time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix())
		})

		Convey("string (DATETIME) to time.Time", func() {
			/* time */
			var t time.Time
			stringScanner := scanner{
				Type: reflect2.TypeOf(t),
				Val:  unsafe.Pointer(&t),
			}

			err := stringScanner.Scan(string("2019-03-01 13:05:59"))
			So(err, ShouldBeNil)
			So(t.Unix(), ShouldEqual, time.Date(2019, 3, 1, 13, 5, 59, 0, time.UTC).Unix())
		})

		Convey("string (DATE) to int64", func() {
			/* int64 */
			var i int64
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix())
		})

		Convey("string (DATETIME) to int64", func() {
			/* int64 */
			var i int64
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01 13:05:59"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, time.Date(2019, 3, 1, 13, 5, 59, 0, time.UTC).Unix())
		})

		Convey("string (DATE) to int", func() {
			/* int */
			var i int
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, int(time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
		})

		Convey("string (DATE) to int8", func() {
			/* int8 */
			var i int8
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, int8(time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
		})

		Convey("string (DATE) to int16", func() {
			/* int16 */
			var i int16
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, int16(time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
		})

		Convey("string (DATE) to int32", func() {
			/* int32 */
			var i int32
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, int32(time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
		})

		Convey("string (DATE) to uint", func() {
			/* uint */
			var i uint
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, uint(time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
		})

		Convey("string (DATE) to uint8", func() {
			/* uint8 */
			var i uint8
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, uint8(time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
		})

		Convey("string (DATE) to uint16", func() {
			/* uint16 */
			var i uint16
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, uint16(time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
		})

		Convey("string (DATE) to uint32", func() {
			/* uint32 */
			var i uint32
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, uint32(time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
		})

		Convey("string (DATE) to uint64", func() {
			/* uint64 */
			var i uint64
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix())
		})

		Convey("string (DATE) to float32", func() {
			/* float32 */
			var f float32
			stringScanner := scanner{
				Type: reflect2.TypeOf(f),
				Val:  unsafe.Pointer(&f),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(f, ShouldEqual, float32(time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
		})

		Convey("string (DATE) to float64", func() {
			/* float64 */
			var f float64
			stringScanner := scanner{
				Type: reflect2.TypeOf(f),
				Val:  unsafe.Pointer(&f),
			}

			err := stringScanner.Scan(string("2019-03-01"))
			So(err, ShouldBeNil)
			So(f, ShouldEqual, float64(time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix()))
		})

		Convey("string (DATETIME) to int", func() {
			/* int */
			var i int
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(string("2019-03-01 13:05:59"))
			So(err, ShouldBeNil)
			So(i, ShouldEqual, time.Date(2019, 3, 1, 13, 5, 59, 0, time.UTC).Unix())
		})

		Convey("bad ts string to time.Time", func() {
			/* time */
			var t time.Time
			stringScanner := scanner{
				Type: reflect2.TypeOf(t),
				Val:  unsafe.Pointer(&t),
			}

			err := stringScanner.Scan("1551405784abc")
			So(err, ShouldNotBeNil)
		})

		Convey("ts string to time.Time", func() {
			/* time */
			var t time.Time
			stringScanner := scanner{
				Type: reflect2.TypeOf(t),
				Val:  unsafe.Pointer(&t),
			}

			err := stringScanner.Scan("1551405784")
			So(err, ShouldBeNil)
			So(t.Unix(), ShouldEqual, time.Date(2019, 3, 1, 2, 3, 4, 0, time.UTC).Unix())
		})

		Convey("time.Time to format string", func() {
			/* time */
			t := time.Unix(1551405784, 0)

			var s string
			stringScanner := scanner{
				Type: reflect2.TypeOf(s),
				Val:  unsafe.Pointer(&s),
			}

			err := stringScanner.Scan(t)
			So(err, ShouldBeNil)
			So(s, ShouldEqual, "2019-03-01 02:03:04")
		})

		Convey("time.Time to int64", func() {
			/* time */
			t := time.Unix(1551405784, 0)

			var i int64
			stringScanner := scanner{
				Type: reflect2.TypeOf(i),
				Val:  unsafe.Pointer(&i),
			}

			err := stringScanner.Scan(t)
			So(err, ShouldBeNil)
			So(i, ShouldEqual, 1551405784)
		})
	})

	Convey("unknown type", t, func() {
		Convey("int64 to x", func() {
			var o x
			stringScanner := scanner{
				Type: reflect2.TypeOf(o),
				Val:  unsafe.Pointer(&o),
			}

			err := stringScanner.Scan(int64(1551405784))
			So(err, ShouldNotBeNil)
		})
	})
}

func TestMatchString(t *testing.T) {
	Convey("matchString", t, func() {
		Convey("tests", func() {
			So(matchString("aa", "a*a", true), ShouldBeTrue)
			So(matchString("abcda", "ccc", true), ShouldBeFalse)
			So(matchString("abcda", "a*a", true), ShouldBeTrue)
			So(matchString("dabcsjabcd", "*abc****a?cd**", true), ShouldBeTrue)
			So(matchString("abcaxcd", "*abc*a?cd**", true), ShouldBeTrue)
			So(matchString("dabssjaxcdal", "*abc***a?cd*", true), ShouldBeFalse)
			So(matchString("dabssabcjaxceaxcd9", "*abc*a?cd*", true), ShouldBeTrue)
			So(matchString("A-2-101~122 -7.7_t3.txt", "*.txt", true), ShouldBeTrue)
			So(matchString("ABBCBCCC.TXT", "A*BC??.txt", false), ShouldBeTrue)
			So(matchString("AA", "a*a", false), ShouldBeTrue)
			So(matchString("Aaaa", "a*a", false), ShouldBeTrue)
			So(matchString("ABA", "a?a", false), ShouldBeTrue)
			So(matchString("ABAd", "a?a", false), ShouldBeFalse)
			So(matchString("xcd", "*?CD*", false), ShouldBeTrue)
		})
	})
}

func test(db *sql.DB) (x, int, error) {
	var o x
	tbl := Table(db, "test").Debug()

	n, err := tbl.Select(&o, Where("`id` >= ?", 1), Limit(100))

	So(err, ShouldBeNil)
	So(n, ShouldEqual, 1)
	return o, n, err
}

func testInsert(db *sql.DB) (int, error) {
	var o x
	tbl := Table(db, "test").Debug()

	n, err := tbl.Insert(&o)
	return n, err
}

func testInsertIgnore(db *sql.DB) (int, error) {
	var o x
	tbl := Table(db, "test").Debug()

	n, err := tbl.InsertIgnore(&o)
	return n, err
}

func testReplaceInto(db *sql.DB) (int, error) {
	var o x
	tbl := Table(db, "test").Debug()

	n, err := tbl.ReplaceInto(&o)
	return n, err
}

func testUpdate(db *sql.DB) (int, error) {
	var o x
	tbl := Table(db, "test").Debug()

	n, err := tbl.Update(&o, Where("`id` >= ?", 1), Limit(1))
	return n, err
}

func testDelete(db *sql.DB) (int, error) {
	tbl := Table(db, "test").Debug()

	n, err := tbl.Delete(Where("`id`=0"), Limit(1))
	return n, err
}

func TestMock(t *testing.T) {
	Convey("Mock one func", t, func() {
		Convey("test Select", func() {
			o := x{
				X:  "Orca1",
				Y:  20,
				Z1: 1551405784,
			}

			// 必须在_test.go里面设置mock
			// 注意方法名需要带包名
			BormMock("test", "Select", "*.test", "", "", &o, 1, nil)

			// 调用被测试函数
			o1, n1, err := test(db)

			So(err, ShouldBeNil)
			So(n1, ShouldEqual, 1)
			So(o1, ShouldResemble, o)

			// 检查是否全部命中
			err = BormMockFinish()
			So(err, ShouldBeNil)
		})

		Convey("test Insert", func() {
			// 必须在_test.go里面设置mock
			// 注意方法名需要带包名
			BormMock("test", "Insert", "*.testInsert", "", "", nil, 10, nil)

			// 调用被测试函数
			n1, err := testInsert(db)

			So(err, ShouldBeNil)
			So(n1, ShouldEqual, 10)

			// 检查是否全部命中
			err = BormMockFinish()
			So(err, ShouldBeNil)
		})

		Convey("test InsertIgnore", func() {
			// 必须在_test.go里面设置mock
			// 注意方法名需要带包名
			BormMock("test", "InsertIgnore", "*.testInsertIgnore", "", "", nil, 22, nil)

			// 调用被测试函数
			n1, err := testInsertIgnore(db)

			So(err, ShouldBeNil)
			So(n1, ShouldEqual, 22)

			// 检查是否全部命中
			err = BormMockFinish()
			So(err, ShouldBeNil)
		})

		Convey("test ReplaceInto", func() {
			// 必须在_test.go里面设置mock
			// 注意方法名需要带包名
			BormMock("test", "ReplaceInto", "*.testReplaceInto", "", "", nil, 22, nil)

			// 调用被测试函数
			n1, err := testReplaceInto(db)

			So(err, ShouldBeNil)
			So(n1, ShouldEqual, 22)

			// 检查是否全部命中
			err = BormMockFinish()
			So(err, ShouldBeNil)
		})

		Convey("test Update", func() {
			// 必须在_test.go里面设置mock
			// 注意方法名需要带包名
			BormMock("test", "Update", "*.testUpdate", "", "", nil, 233, nil)

			// 调用被测试函数
			n1, err := testUpdate(db)

			So(err, ShouldBeNil)
			So(n1, ShouldEqual, 233)

			// 检查是否全部命中
			err = BormMockFinish()
			So(err, ShouldBeNil)
		})

		Convey("test Delete", func() {
			// 必须在_test.go里面设置mock
			// 注意方法名需要带包名
			BormMock("test", "Delete", "*.testDelete", "", "", nil, 88, nil)

			// 调用被测试函数
			n1, err := testDelete(db)

			So(err, ShouldBeNil)
			So(n1, ShouldEqual, 88)

			// 检查是否全部命中
			err = BormMockFinish()
			So(err, ShouldBeNil)
		})

		Convey("test any", func() {
			// 必须在_test.go里面设置mock
			// 注意方法名需要带包名
			BormMock("test", "", "", "", "", nil, 111, nil)

			// 调用被测试函数
			n1, err := testDelete(db)

			So(err, ShouldBeNil)
			So(n1, ShouldEqual, 111)

			// 检查是否全部命中
			err = BormMockFinish()
			So(err, ShouldBeNil)
		})

		Convey("test none", func() {
			// 必须在_test.go里面设置mock
			// 注意方法名需要带包名
			BormMock("test", "", "", "", "", nil, 111, nil)

			// 检查是否未全部命中
			err := BormMockFinish()
			So(err, ShouldNotBeNil)

			// 检查是否全部命中
			err = BormMockFinish()
			So(err, ShouldBeNil)
		})
	})
}

func PanicCheck(f func()) (err interface{}) {
	defer func() {
		if t := recover(); t != nil {
			err = t
		}
	}()
	f()
	return
}

func ReuseTest() {
	var o x
	tbl := Table(db, "test").Reuse()

	n, err := tbl.Select(&o, Fields("name", "ctime", "age"), Where("`id` >= ?", 1), Limit(100))

	So(err, ShouldBeNil)
	So(n, ShouldEqual, 1)
	fmt.Printf("%+v\n", o)

	var cnt int64

	n, err = tbl.Select(&cnt, Fields("count(1)"), Where(Eq("id", 1)), Limit(100))

	So(err, ShouldBeNil)
	So(n, ShouldEqual, 1)
	fmt.Printf("%+v\n", cnt)
}

func TestMisc(t *testing.T) {
	Convey("Table", t, func() {
		Convey("Table with context", func() {
			t := Table(db, "test", context.TODO())
			t.UseNameWhenTagEmpty()
			t.ToTimestamp()
		})
	})

	Convey("Where", t, func() {
		Convey("Where panic", func() {
			So(PanicCheck(func() {
				Where()
			}), ShouldNotBeNil)
		})
		Convey("Where In empty slice", func() {
			w := Where(In("id"))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where 1=1")
			So(len(stmtArgs), ShouldEqual, 0)
		})
		Convey("Where In empty slice slice", func() {
			w := Where(In("id", []interface{}{}))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where 1=1")
			So(len(stmtArgs), ShouldEqual, 0)
		})
		Convey("Where In 1 slice", func() {
			w := Where(In("id", []interface{}{1}))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=?")
			So(len(stmtArgs), ShouldEqual, 1)
		})
		Convey("Where In 2 slice", func() {
			w := Where(In("id", []interface{}{1, 2}))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id` in (?,?)")
			So(len(stmtArgs), ShouldEqual, 2)
		})
		Convey("Where - 1st empty And", func() {
			// And empty
			w := Where(And())
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, "")
			So(len(stmtArgs), ShouldEqual, 0)
		})
		Convey("Where - 1st normal 1 arg And", func() {
			// normal And
			w := Where(And(Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=?")
			So(len(stmtArgs), ShouldEqual, 1)
		})
		Convey("Where - 1st normal more arg And", func() {
			// normal And
			w := Where(And(Eq("id", 0), Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=? and `id`=?")
			So(len(stmtArgs), ShouldEqual, 2)
		})
		Convey("Where - 1st empty Or", func() {
			// Or empty
			w := Where(Or())
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, "")
			So(len(stmtArgs), ShouldEqual, 0)
		})
		Convey("Where - 1st normal 1 arg Or", func() {
			// normal Or
			w := Where(Or(Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=?")
			So(len(stmtArgs), ShouldEqual, 1)
		})
		Convey("Where - 1st normal more arg Or", func() {
			// normal Or
			w := Where(Or(Eq("id", 0), Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=? or `id`=?")
			So(len(stmtArgs), ShouldEqual, 2)
		})
		Convey("Where - 2rd empty And", func() {
			// And empty
			w := Where(Eq("id", 0), And())
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=?")
			So(len(stmtArgs), ShouldEqual, 1)
		})
		Convey("Where - 2rd normal 1 arg And", func() {
			// normal And
			w := Where(Eq("id", 0), And(Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=? and `id`=?")
			So(len(stmtArgs), ShouldEqual, 2)
		})
		Convey("Where - 2rd normal more arg And", func() {
			// normal And
			w := Where(Eq("id", 0), And(Eq("id", 0), Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=? and `id`=? and `id`=?")
			So(len(stmtArgs), ShouldEqual, 3)
		})
		Convey("Where - 2rd empty Or", func() {
			// Or empty
			w := Where(Eq("id", 0), Or())
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=?")
			So(len(stmtArgs), ShouldEqual, 1)
		})
		Convey("Where - 2rd normal 1 arg Or", func() {
			// normal Or
			w := Where(Eq("id", 0), Or(Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=? and `id`=?")
			So(len(stmtArgs), ShouldEqual, 2)
		})
		Convey("Where - 2rd normal more arg Or", func() {
			// normal Or
			w := Where(Eq("id", 0), Or(Eq("id", 0), Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=? and (`id`=? or `id`=?)")
			So(len(stmtArgs), ShouldEqual, 3)
		})
		Convey("Where - And with Or", func() {
			w := Where(And(Eq("id", 0), Or(Eq("id", 0), Eq("id", 0))))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=? and (`id`=? or `id`=?)")
			So(len(stmtArgs), ShouldEqual, 3)
		})
		Convey("Where - Or with And", func() {
			w := Where(Or(Eq("id", 0), And(Eq("id", 0), Eq("id", 0))))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " where `id`=? or (`id`=? and `id`=?)")
			So(len(stmtArgs), ShouldEqual, 3)
		})
	})

	Convey("Having", t, func() {
		Convey("Having panic", func() {
			So(PanicCheck(func() {
				Having()
			}), ShouldNotBeNil)
		})
		Convey("Having - 1st empty And", func() {
			// And empty
			w := Having(And())
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, "")
			So(len(stmtArgs), ShouldEqual, 0)
		})
		Convey("Having - 1st normal 1 arg And", func() {
			// normal And
			w := Having(And(Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=?")
			So(len(stmtArgs), ShouldEqual, 1)
		})
		Convey("Having - 1st normal more arg And", func() {
			// normal And
			w := Having(And(Eq("id", 0), Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=? and `id`=?")
			So(len(stmtArgs), ShouldEqual, 2)
		})
		Convey("Having - 1st empty Or", func() {
			// Or empty
			w := Having(Or())
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, "")
			So(len(stmtArgs), ShouldEqual, 0)
		})
		Convey("Having - 1st normal 1 arg Or", func() {
			// normal Or
			w := Having(Or(Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=?")
			So(len(stmtArgs), ShouldEqual, 1)
		})
		Convey("Having - 1st normal more arg Or", func() {
			// normal Or
			w := Having(Or(Eq("id", 0), Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=? or `id`=?")
			So(len(stmtArgs), ShouldEqual, 2)
		})
		Convey("Having - 2rd empty And", func() {
			// And empty
			w := Having(Eq("id", 0), And())
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=?")
			So(len(stmtArgs), ShouldEqual, 1)
		})
		Convey("Having - 2rd normal 1 arg And", func() {
			// normal And
			w := Having(Eq("id", 0), And(Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=? and `id`=?")
			So(len(stmtArgs), ShouldEqual, 2)
		})
		Convey("Having - 2rd normal more arg And", func() {
			// normal And
			w := Having(Eq("id", 0), And(Eq("id", 0), Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=? and `id`=? and `id`=?")
			So(len(stmtArgs), ShouldEqual, 3)
		})
		Convey("Having - 2rd empty Or", func() {
			// Or empty
			w := Having(Eq("id", 0), Or())
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=?")
			So(len(stmtArgs), ShouldEqual, 1)
		})
		Convey("Having - 2rd normal 1 arg Or", func() {
			// normal Or
			w := Having(Eq("id", 0), Or(Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=? and `id`=?")
			So(len(stmtArgs), ShouldEqual, 2)
		})
		Convey("Having - 2rd normal more arg Or", func() {
			// normal Or
			w := Having(Eq("id", 0), Or(Eq("id", 0), Eq("id", 0)))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=? and (`id`=? or `id`=?)")
			So(len(stmtArgs), ShouldEqual, 3)
		})
		Convey("Having - And with Or", func() {
			w := Having(And(Eq("id", 0), Or(Eq("id", 0), Eq("id", 0))))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=? and (`id`=? or `id`=?)")
			So(len(stmtArgs), ShouldEqual, 3)
		})
		Convey("Having - Or with And", func() {
			w := Having(Or(Eq("id", 0), And(Eq("id", 0), Eq("id", 0))))
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, " having `id`=? or (`id`=? and `id`=?)")
			So(len(stmtArgs), ShouldEqual, 3)
		})
	})

	Convey("Embedded And and Or", t, func() {
		Convey("Embedded And with Or", func() {
			w := Or(Eq("id1", 0),
				And(Eq("id2", 0),
					Eq("id3", 0),
					Or(Eq("id4", 0),
						Eq("id5", 0),
					),
				),
				Or(Eq("id6", 0),
					Eq("id7", 0),
				),
			)
			var sb strings.Builder
			var stmtArgs []interface{}
			w.BuildSQL(&sb)
			w.BuildArgs(&stmtArgs)

			So(sb.String(), ShouldEqual, "`id1`=? or (`id2`=? and `id3`=? and (`id4`=? or `id5`=?)) or (`id6`=? or `id7`=?)")
			So(len(stmtArgs), ShouldEqual, 7)
		})
	})

	Convey("Limit", t, func() {
		Convey("Limit panic", func() {
			So(PanicCheck(func() {
				Limit()
			}), ShouldNotBeNil)
		})
		Convey("Limit panic 2", func() {
			So(PanicCheck(func() {
				Limit(1, 2, 3)
			}), ShouldNotBeNil)
		})
	})

	Convey("Select", t, func() {
		Convey("Select - arg len err", func() {
			t := Table(db, "test", context.TODO())

			var o x
			_, err := t.Select(&o)
			So(err, ShouldNotBeNil)
		})

		Convey("Select - arg type err", func() {
			t := Table(db, "test", context.TODO())

			var o x
			_, err := t.Select(o, Where("`id` >= ?", 1))
			So(err, ShouldNotBeNil)
		})

		Convey("Select - Reuse", func() {
			ReuseTest()
			ReuseTest()
		})

		Convey("Select - UseNameWhenTagEmpty", func() {
			t := Table(db, "test", context.TODO())

			var o x1
			_, err := t.Select(&o, Where("`id` >= ?", 1))
			So(err, ShouldBeNil)
			So(o.CTime(), ShouldEqual, 0)

			_, err = t.UseNameWhenTagEmpty().Select(&o, Where("`id` >= ?", 1))
			So(err, ShouldBeNil)
			So(o.CTime(), ShouldNotEqual, 0)

			_, err = t.UseNameWhenTagEmpty().Select(&o, Fields("ctime"), Where("`id` >= ?", 1))
			So(err, ShouldBeNil)
			So(o.CTime(), ShouldNotEqual, 0)
		})

		Convey("Select - other type with Fields", func() {
			t := Table(db, "test", context.TODO())

			var cnt int64
			_, err := t.Select(&cnt, Where("`id` >= ?", 1))
			So(err, ShouldNotBeNil)

			_, err = t.Select(&cnt, Fields())
			So(err, ShouldNotBeNil)
		})

		Convey("Select - empty single result", func() {
			t := Table(db, "test", context.TODO())

			var o x
			n, err := t.Select(&o, Where("`id` >= ?", 1011))
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 0)
		})

		Convey("Select - sql error", func() {
			t := Table(db, "test", context.TODO())

			var o x
			n, err := t.Select(&o, Where("xxxx"))
			So(err, ShouldNotBeNil)
			So(n, ShouldEqual, 0)

			var o1 []x
			n, err = t.Select(&o1, Where("xxxx"))
			So(err, ShouldNotBeNil)
			So(n, ShouldEqual, 0)
		})

		Convey("Select - scan error", func() {
			t := Table(db, "test", context.TODO())

			var o []struct {
				Name struct {
					I int64
				} `borm:"name"`
			}
			n, err := t.Debug().Select(&o, Where(Lt("id", 100), Like("name", "Or%")), Limit(1))
			So(err, ShouldNotBeNil)
			So(n, ShouldEqual, 0)
		})
	})

	Convey("Insert", t, func() {
		Convey("Insert - arg type err", func() {
			t := Table(db, "test", context.TODO())

			var o x
			_, err := t.Insert(o, Where("`id` >= ?", 1))
			So(err, ShouldNotBeNil)

			var i int64
			_, err = t.Insert(&i, Where("`id` >= ?", 1))
			So(err, ShouldNotBeNil)
		})

		Convey("Insert - sql error", func() {
			t := Table(db, "test", context.TODO())

			var o x
			n, err := t.Insert(&o, Where("xxxx"))
			So(err, ShouldNotBeNil)
			So(n, ShouldEqual, 0)
		})

		Convey("Insert - UseNameWhenTagEmpty", func() {
			t := Table(db, "test", context.TODO())

			o := x1{
				X: "xxx",
			}
			n, err := t.Insert(&o)
			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)

			n, err = t.UseNameWhenTagEmpty().Insert(&o)
			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)

			n, err = t.UseNameWhenTagEmpty().Insert(&o, Fields("ctime"))
			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Update", t, func() {
		Convey("Update - arg len err", func() {
			t := Table(db, "test", context.TODO())

			var o x
			_, err := t.Update(&o)
			So(err, ShouldNotBeNil)
		})

		Convey("Update - arg type err", func() {
			t := Table(db, "test", context.TODO())

			var o x
			_, err := t.Update(o, Where("`id` >= ?", 1))
			So(err, ShouldNotBeNil)

			var i int64
			_, err = t.Update(&i, Where("`id` >= ?", 1))
			So(err, ShouldNotBeNil)
		})

		Convey("Update - sql error", func() {
			t := Table(db, "test", context.TODO())

			var o x
			n, err := t.Update(&o, Where("xxxx"))
			So(err, ShouldNotBeNil)
			So(n, ShouldEqual, 0)
		})

		Convey("Update - UseNameWhenTagEmpty", func() {
			t := Table(db, "test", context.TODO())

			o := x1{
				X:     "xxx2",
				ctime: 1,
			}
			n, err := t.Update(&o, Where("id>=0"), Limit(1))
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)

			o.X += "1"
			n, err = t.UseNameWhenTagEmpty().Update(&o, Where("id>=0"), Limit(1))
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)

			o.X += "1"
			n, err = t.UseNameWhenTagEmpty().Update(&o, Fields("name"), Where("id>=0"), Limit(1))
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
		})
	})

	Convey("Delete", t, func() {
		Convey("Delete - arg len err", func() {
			t := Table(db, "test", context.TODO())

			_, err := t.Delete()
			So(err, ShouldNotBeNil)
		})

		Convey("Delete - sql error", func() {
			t := Table(db, "test", context.TODO())

			n, err := t.Delete(Where("xxxx"))
			So(err, ShouldNotBeNil)
			So(n, ShouldEqual, 0)
		})
	})

	Convey("toUnix - leap year", t, func() {
		So(toUnix(2020, 3, 1, 0, 0, 0), ShouldEqual, 1583020800)
	})

	Convey("fieldsItem - BuildArgs", t, func() {
		var stmtArgs []interface{}
		var f fieldsItem
		f.BuildArgs(&stmtArgs)
		So(len(stmtArgs), ShouldEqual, 0)
	})

	Convey("onDuplicateKeyUpdateItem - Type", t, func() {
		var odku onDuplicateKeyUpdateItem
		So(odku.Type(), ShouldEqual, _onDuplicateKeyUpdate)
	})

	Convey("OnDuplicateKeyUpdate - Empty", t, func() {
		w := OnDuplicateKeyUpdate(V{})
		var sb strings.Builder
		var stmtArgs []interface{}
		w.BuildSQL(&sb)
		w.BuildArgs(&stmtArgs)

		So(sb.String(), ShouldEqual, "")
		So(len(stmtArgs), ShouldEqual, 0)
	})

	Convey("havingItem - Type", t, func() {
		var having havingItem
		So(having.Type(), ShouldEqual, _having)
	})

	Convey("orderByItem - Type", t, func() {
		var orderBy orderByItem
		So(orderBy.Type(), ShouldEqual, _orderBy)
	})

	Convey("limitItem - Type", t, func() {
		var limit limitItem
		So(limit.Type(), ShouldEqual, _limit)
	})

	Convey("ormCondEx - Type", t, func() {
		condEx := &ormCondEx{Ty: _andCondEx}
		So(condEx.Type(), ShouldEqual, _andCondEx)
	})

	Convey("checkInTestFile", t, func() {
		Convey("checkInTestFile normal", func() {
			So(PanicCheck(func() {
				checkInTestFile("aaa_test.go")
			}), ShouldBeNil)
		})
		Convey("checkInTestFile panic", func() {
			So(PanicCheck(func() {
				checkInTestFile("aaa.go")
			}), ShouldNotBeNil)
		})
	})

	Convey("numberToString", t, func() {
		Convey("default", func() {
			var i int8
			t := reflect2.TypeOf(i)
			So(numberToString(t.Kind(), i), ShouldEqual, "")
		})
	})

	Convey("strconvErr", t, func() {
		Convey("strconv.NumError", func() {
			err := &strconv.NumError{
				Func: "fn",
				Num:  "str",
				Err:  strconv.ErrSyntax,
			}
			So(strconvErr(err), ShouldEqual, err.Err)
		})
		Convey("normal err", func() {
			err := errors.New("xxx")
			So(strconvErr(err), ShouldEqual, err)
		})
	})

	Convey("numberToString", t, func() {
		Convey("default", func() {
			var i int8
			t := reflect2.TypeOf(i)
			So(numberToString(t.Kind(), i), ShouldEqual, "")
		})
	})
}
