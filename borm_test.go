package borm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	_ "github.com/go-sql-driver/mysql"
	"github.com/modern-go/reflect2"
	. "github.com/smartystreets/goconvey/convey"
)

var db *sql.DB

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

		Convey("join select", func() {
			var ids []int64
			tbl := Table(db, "test").Debug()

			n, err := tbl.Select(&ids, Fields("test.id"), Join("join test2 on test.id=test2.id"), Limit(100))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
			So(len(ids), ShouldEqual, 1)
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

// TestMapSupport 测试Map类型支持功能
func TestMapSupport(t *testing.T) {
	// 初始化数据库连接
	db, err := sql.Open("mysql", "root:semaphoredb@tcp(localhost:3306)/borm_test?charset=utf8mb4")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 创建测试表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS test_map (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100),
		age INT,
		email VARCHAR(100),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// 清理测试数据
	defer func() {
		db.Exec("DELETE FROM test_map")
	}()

	tbl := Table(db, "test_map").Debug()

	t.Run("TestVTypeInsert", func(t *testing.T) {
		// 使用V类型插入数据
		userMap := V{
			"name":  "John Doe",
			"age":   30,
			"email": "john@example.com",
		}

		n, err := tbl.Insert(userMap)
		if err != nil {
			t.Errorf("Insert failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}
	})

	t.Run("TestGenericMapInsert", func(t *testing.T) {
		// 使用通用map类型插入数据
		userMap := map[string]interface{}{
			"name":  "Jane Doe",
			"age":   25,
			"email": "jane@example.com",
		}

		n, err := tbl.Insert(userMap)
		if err != nil {
			t.Errorf("Insert failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}
	})

	t.Run("TestVTypeUpdate", func(t *testing.T) {
		// 先插入一条数据
		userMap := V{
			"name":  "Update Test",
			"age":   20,
			"email": "update@example.com",
		}
		n, err := tbl.Insert(userMap)
		if err != nil {
			t.Errorf("Insert failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}

		// 更新数据
		updateMap := V{
			"name": "Updated Name",
			"age":  21,
		}

		n, err = tbl.Update(updateMap, Where("email = ?", "update@example.com"))
		if err != nil {
			t.Errorf("Update failed: %v", err)
		}
		if n <= 0 {
			t.Errorf("Expected at least 1 row updated, got %d", n)
		}
	})

	t.Run("TestGenericMapUpdate", func(t *testing.T) {
		// 先插入一条数据
		userMap := V{
			"name":  "Generic Update Test",
			"age":   20,
			"email": "generic@example.com",
		}
		n, err := tbl.Insert(userMap)
		if err != nil {
			t.Errorf("Insert failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}

		// 更新数据
		updateMap := V{
			"name": "Generic Updated Name",
			"age":  21,
		}

		n, err = tbl.Update(updateMap, Where("email = ?", "generic@example.com"))
		if err != nil {
			t.Errorf("Update failed: %v", err)
		}
		if n <= 0 {
			t.Errorf("Expected at least 1 row updated, got %d", n)
		}
	})

	t.Run("TestSelectToMap", func(t *testing.T) {
		// 先插入一条数据
		userMap := V{
			"name":  "Select Test",
			"age":   30,
			"email": "select@example.com",
		}
		n, err := tbl.Insert(userMap)
		if err != nil {
			t.Errorf("Insert failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}

		// 查询单条记录到map
		var result V
		n, err = tbl.Select(&result, Fields("name", "age", "email"), Where("email = ?", "select@example.com"))
		if err != nil {
			t.Errorf("Select failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row selected, got %d", n)
		}
		if result["name"] != "Select Test" {
			t.Errorf("Expected name 'Select Test', got %v", result["name"])
		}
		if result["age"] != int64(30) {
			t.Errorf("Expected age 30, got %v", result["age"])
		}
		if result["email"] != "select@example.com" {
			t.Errorf("Expected email 'select@example.com', got %v", result["email"])
		}
	})

	t.Run("TestSelectToMapSlice", func(t *testing.T) {
		// 先插入多条数据
		users := []V{
			{"name": "User1", "age": 25, "email": "user1@example.com"},
			{"name": "User2", "age": 26, "email": "user2@example.com"},
		}

		for _, user := range users {
			n, err := tbl.Insert(user)
			if err != nil {
				t.Errorf("Insert failed: %v", err)
			}
			if n != 1 {
				t.Errorf("Expected 1 row inserted, got %d", n)
			}
		}

		// 查询多条记录到map切片
		var results []V
		n, err := tbl.Select(&results, Fields("name", "age", "email"), Where("email LIKE ?", "user%@example.com"))
		if err != nil {
			t.Errorf("Select failed: %v", err)
		}
		if n <= 0 {
			t.Errorf("Expected at least 1 row selected, got %d", n)
		}
		if len(results) == 0 {
			t.Errorf("Expected non-empty results slice")
		}

		// 验证结果
		for _, result := range results {
			if result["name"] == nil {
				t.Errorf("Expected non-nil name")
			}
			if result["age"] == nil {
				t.Errorf("Expected non-nil age")
			}
			if result["email"] == nil {
				t.Errorf("Expected non-nil email")
			}
		}
	})

	t.Run("TestInsertIgnoreAndReplaceInto", func(t *testing.T) {
		// 测试InsertIgnore
		userMap := V{
			"name":  "Ignore Test",
			"age":   30,
			"email": "ignore@example.com",
		}

		n, err := tbl.InsertIgnore(userMap)
		if err != nil {
			t.Errorf("InsertIgnore failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}

		// 再次插入相同数据，应该被忽略
		n, err = tbl.InsertIgnore(userMap)
		if err != nil {
			t.Errorf("InsertIgnore failed: %v", err)
		}
		if n != 0 {
			t.Errorf("Expected 0 rows inserted (ignored), got %d", n)
		}

		// 测试ReplaceInto
		replaceMap := V{
			"name":  "Replace Test",
			"age":   35,
			"email": "replace@example.com",
		}

		n, err = tbl.ReplaceInto(replaceMap)
		if err != nil {
			t.Errorf("ReplaceInto failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}
	})

	t.Run("TestMapFieldsSupport", func(t *testing.T) {
		// 使用Fields参数插入部分字段
		userMap := V{
			"name":  "Fields Test",
			"age":   30,
			"email": "fields@example.com",
			"extra": "should be ignored",
		}

		n, err := tbl.Insert(userMap, Fields("name", "age", "email"))
		if err != nil {
			t.Errorf("Insert with Fields failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}

		// 验证只插入了指定字段
		var result V
		n, err = tbl.Select(&result, Fields("name", "age", "email"), Where("email = ?", "fields@example.com"))
		if err != nil {
			t.Errorf("Select failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row selected, got %d", n)
		}
		if result["name"] != "Fields Test" {
			t.Errorf("Expected name 'Fields Test', got %v", result["name"])
		}
		if result["age"] != int64(30) {
			t.Errorf("Expected age 30, got %v", result["age"])
		}
		if result["email"] != "fields@example.com" {
			t.Errorf("Expected email 'fields@example.com', got %v", result["email"])
		}
	})

	t.Run("TestMapUTypeSupport", func(t *testing.T) {
		// 先插入一条数据
		userMap := V{
			"name":  "U Test",
			"age":   30,
			"email": "u@example.com",
		}
		n, err := tbl.Insert(userMap)
		if err != nil {
			t.Errorf("Insert failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}

		// 使用U类型更新
		updateMap := V{
			"age": U("age + 1"),
		}

		n, err = tbl.Update(updateMap, Where("email = ?", "u@example.com"))
		if err != nil {
			t.Errorf("Update with U type failed: %v", err)
		}
		if n <= 0 {
			t.Errorf("Expected at least 1 row updated, got %d", n)
		}

		// 验证更新结果
		var result V
		n, err = tbl.Select(&result, Fields("age"), Where("email = ?", "u@example.com"))
		if err != nil {
			t.Errorf("Select failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row selected, got %d", n)
		}
		if result["age"] != int64(31) {
			t.Errorf("Expected age 31, got %v", result["age"])
		}
	})

	t.Run("TestMapComplexQuery", func(t *testing.T) {
		// 插入测试数据
		users := []V{
			{"name": "Complex1", "age": 25, "email": "complex1@example.com"},
			{"name": "Complex2", "age": 30, "email": "complex2@example.com"},
			{"name": "Complex3", "age": 35, "email": "complex3@example.com"},
		}

		for _, user := range users {
			n, err := tbl.Insert(user)
			if err != nil {
				t.Errorf("Insert failed: %v", err)
			}
			if n != 1 {
				t.Errorf("Expected 1 row inserted, got %d", n)
			}
		}

		// 复杂查询：年龄在25-35之间，按年龄排序
		var results []V
		n, err := tbl.Select(&results,
			Fields("name", "age", "email"),
			Where(And(
				Gte("age", 25),
				Lte("age", 35),
			)),
			OrderBy("age"),
			Limit(10),
		)
		if err != nil {
			t.Errorf("Complex query failed: %v", err)
		}
		if n <= 0 {
			t.Errorf("Expected at least 1 row selected, got %d", n)
		}
		if len(results) == 0 {
			t.Errorf("Expected non-empty results slice")
		}

		// 验证结果按年龄排序
		for i := 1; i < len(results); i++ {
			prevAge := results[i-1]["age"].(int64)
			currAge := results[i]["age"].(int64)
			if prevAge > currAge {
				t.Errorf("Results not sorted by age: %d > %d", prevAge, currAge)
			}
		}
	})
}

// TestMapSupportWithContext 测试带Context的Map支持功能
func TestMapSupportWithContext(t *testing.T) {
	// 初始化数据库连接
	db, err := sql.Open("mysql", "root:semaphoredb@tcp(localhost:3306)/borm_test?charset=utf8mb4")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 创建测试表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS test_map_ctx (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100),
		age INT,
		email VARCHAR(100),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// 清理测试数据
	defer func() {
		db.Exec("DELETE FROM test_map_ctx")
	}()

	ctx := context.Background()
	tbl := TableContext(ctx, db, "test_map_ctx").Debug()

	t.Run("TestMapWithContext", func(t *testing.T) {
		// 使用V类型插入数据
		userMap := V{
			"name":  "Context Test",
			"age":   30,
			"email": "context@example.com",
		}

		n, err := tbl.Insert(userMap)
		if err != nil {
			t.Errorf("Insert failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}

		// 查询数据
		var result V
		n, err = tbl.Select(&result, Fields("name", "age", "email"), Where("email = ?", "context@example.com"))
		if err != nil {
			t.Errorf("Select failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row selected, got %d", n)
		}
		if result["name"] != "Context Test" {
			t.Errorf("Expected name 'Context Test', got %v", result["name"])
		}
	})

	t.Run("TestTableContextAPI", func(t *testing.T) {
		// 测试TableContext API
		ctx := context.WithValue(context.Background(), "test_key", "test_value")
		tbl := TableContext(ctx, db, "test_map_ctx")

		// 验证TableContext创建成功
		if tbl == nil {
			t.Errorf("TableContext should not be nil")
		}
		if tbl.Name != "test_map_ctx" {
			t.Errorf("Expected table name 'test_map_ctx', got %s", tbl.Name)
		}
	})
}

// TestMapSupportErrorHandling 测试Map支持的错误处理
func TestMapSupportErrorHandling(t *testing.T) {
	// 初始化数据库连接
	db, err := sql.Open("mysql", "root:semaphoredb@tcp(localhost:3306)/borm_test?charset=utf8mb4")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	tbl := Table(db, "test_map").Debug()

	t.Run("TestEmptyMap", func(t *testing.T) {
		emptyMap := V{}
		n, err := tbl.Insert(emptyMap)
		if err == nil {
			t.Errorf("Expected error for empty map, got nil")
		}
		if n != 0 {
			t.Errorf("Expected 0 rows inserted for empty map, got %d", n)
		}
	})

	t.Run("TestMapWithNilValues", func(t *testing.T) {
		mapWithNil := V{
			"name":  "Nil Test",
			"age":   nil,
			"email": "nil@example.com",
		}
		n, err := tbl.Insert(mapWithNil)
		if err != nil {
			t.Errorf("Insert with nil values failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}
	})
}

// BenchmarkMapOperations Map操作的基准测试
func BenchmarkMapOperations(b *testing.B) {
	// 初始化数据库连接
	db, err := sql.Open("mysql", "root:semaphoredb@tcp(localhost:3306)/borm_test?charset=utf8mb4")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// 创建测试表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS test_map_bench (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100),
		age INT,
		email VARCHAR(100),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	db.Exec(createTableSQL)

	// 清理测试数据
	defer func() {
		db.Exec("DELETE FROM test_map_bench")
	}()

	tbl := Table(db, "test_map_bench")

	b.Run("MapInsert", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			userMap := V{
				"name":  "Benchmark User",
				"age":   30,
				"email": "benchmark@example.com",
			}
			tbl.Insert(userMap)
		}
	})

	b.Run("MapSelect", func(b *testing.B) {
		// 先插入一些测试数据
		for i := 0; i < 100; i++ {
			userMap := V{
				"name":  "Benchmark User",
				"age":   30,
				"email": "benchmark@example.com",
			}
			tbl.Insert(userMap)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var results []V
			tbl.Select(&results, Fields("name", "age", "email"), Limit(10))
		}
	})

	b.Run("MapUpdate", func(b *testing.B) {
		// 先插入一些测试数据
		for i := 0; i < 100; i++ {
			userMap := V{
				"name":  "Benchmark User",
				"age":   30,
				"email": "benchmark@example.com",
			}
			tbl.Insert(userMap)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			updateMap := V{
				"age": 31,
			}
			tbl.Update(updateMap, Where("age = ?", 30), Limit(1))
		}
	})
}

// TestReuseFunctionality 测试Reuse功能
func TestReuseFunctionality(t *testing.T) {
	Convey("测试Reuse功能", t, func() {
		// 测试Reuse方法
		table := &BormTable{
			Cfg: Config{},
		}

		// 验证初始状态
		So(table.Cfg.Reuse, ShouldBeFalse)

		// 调用Reuse方法
		result := table.Reuse()

		// 验证Reuse方法返回自身
		So(result, ShouldEqual, table)
		So(table.Cfg.Reuse, ShouldBeTrue)
	})
}

// TestFieldMapCache 测试字段映射缓存功能
func TestFieldMapCache(t *testing.T) {
	Convey("测试字段映射缓存功能", t, func() {
		type TestStruct struct {
			ID   int64  `borm:"id"`
			Name string `borm:"name"`
			Age  int    `borm:"age"`
		}

		table := &BormTable{
			Cfg: Config{UseNameWhenTagEmpty: true},
		}

		// 第一次调用，应该从缓存中获取
		rt := reflect2.TypeOf(TestStruct{})
		structType := rt.(reflect2.StructType)

		// 清空缓存
		table.fieldMapCache = sync.Map{}

		// 第一次调用，应该构建并缓存
		fieldMap1 := table.getStructFieldMap(structType)
		So(fieldMap1, ShouldNotBeNil)
		So(len(fieldMap1), ShouldEqual, 3)
		So(fieldMap1["id"], ShouldNotBeNil)
		So(fieldMap1["name"], ShouldNotBeNil)
		So(fieldMap1["age"], ShouldNotBeNil)

		// 第二次调用，应该从缓存中获取
		fieldMap2 := table.getStructFieldMap(structType)
		So(fieldMap2, ShouldNotBeNil)
		So(len(fieldMap2), ShouldEqual, 3)

		// 验证是同一个map（缓存生效）
		So(fieldMap1, ShouldEqual, fieldMap2)
	})
}

// TestFieldMapCacheWithDifferentStructs 测试不同结构体的字段缓存
func TestFieldMapCacheWithDifferentStructs(t *testing.T) {
	Convey("测试不同结构体的字段缓存", t, func() {
		type Struct1 struct {
			ID   int64  `borm:"id"`
			Name string `borm:"name"`
		}

		type Struct2 struct {
			ID    int64  `borm:"id"`
			Email string `borm:"email"`
		}

		table := &BormTable{
			Cfg: Config{UseNameWhenTagEmpty: true},
		}

		// 清空缓存
		table.fieldMapCache = sync.Map{}

		// 测试Struct1
		rt1 := reflect2.TypeOf(Struct1{})
		structType1 := rt1.(reflect2.StructType)
		fieldMap1 := table.getStructFieldMap(structType1)
		So(fieldMap1, ShouldNotBeNil)
		So(len(fieldMap1), ShouldEqual, 2)
		So(fieldMap1["id"], ShouldNotBeNil)
		So(fieldMap1["name"], ShouldNotBeNil)
		So(fieldMap1["email"], ShouldBeNil)

		// 测试Struct2
		rt2 := reflect2.TypeOf(Struct2{})
		structType2 := rt2.(reflect2.StructType)
		fieldMap2 := table.getStructFieldMap(structType2)
		So(fieldMap2, ShouldNotBeNil)
		So(len(fieldMap2), ShouldEqual, 2)
		So(fieldMap2["id"], ShouldNotBeNil)
		So(fieldMap2["email"], ShouldNotBeNil)
		So(fieldMap2["name"], ShouldBeNil)

		// 验证两个结构体的字段映射不同
		So(fieldMap1, ShouldNotEqual, fieldMap2)
	})
}

// TestFieldMapCacheWithEmbeddedStruct 测试embedded struct的字段缓存
func TestFieldMapCacheWithEmbeddedStruct(t *testing.T) {
	Convey("测试embedded struct的字段缓存", t, func() {
		type Address struct {
			Street string `borm:"street"`
			City   string `borm:"city"`
		}

		type User struct {
			ID      int64   `borm:"id"`
			Name    string  `borm:"name"`
			Address Address `borm:"-"` // embedded struct
		}

		table := &BormTable{
			Cfg: Config{UseNameWhenTagEmpty: true},
		}

		// 清空缓存
		table.fieldMapCache = sync.Map{}

		rt := reflect2.TypeOf(User{})
		structType := rt.(reflect2.StructType)
		fieldMap := table.getStructFieldMap(structType)

		So(fieldMap, ShouldNotBeNil)
		So(len(fieldMap), ShouldEqual, 2) // 只有id和name，Address有borm:"-"标签
		So(fieldMap["id"], ShouldNotBeNil)
		So(fieldMap["name"], ShouldNotBeNil)
		So(fieldMap["street"], ShouldBeNil) // embedded struct字段不会被收集
		So(fieldMap["city"], ShouldBeNil)
	})
}

// TestDataBindingCache 测试数据绑定缓存
func TestDataBindingCache(t *testing.T) {
	Convey("测试数据绑定缓存", t, func() {
		// 测试storeToCache
		item := &DataBindingItem{
			SQL:  "SELECT * FROM test",
			Cols: []interface{}{"id", "name"},
			Type: reflect2.TypeOf(""),
			Elem: "test",
		}

		storeToCache("test.go", 123, item)

		// 测试loadFromCache
		loadedItem := loadFromCache("test.go", 123)
		So(loadedItem, ShouldNotBeNil)
		So(loadedItem.SQL, ShouldEqual, "SELECT * FROM test")
		So(len(loadedItem.Cols), ShouldEqual, 2)
		So(loadedItem.Cols[0], ShouldEqual, "id")
		So(loadedItem.Cols[1], ShouldEqual, "name")

		// 测试不存在的缓存
		notFoundItem := loadFromCache("test.go", 456)
		So(notFoundItem, ShouldBeNil)
	})
}

// TestReuseCacheKeyGeneration 测试Reuse缓存键生成
func TestReuseCacheKeyGeneration(t *testing.T) {
	Convey("测试Reuse缓存键生成", t, func() {
		// 测试缓存键格式
		key1 := fmt.Sprintf("%s:%d", "test.go", 123)
		key2 := fmt.Sprintf("%s:%d", "test.go", 456)
		key3 := fmt.Sprintf("%s:%d", "other.go", 123)

		So(key1, ShouldEqual, "test.go:123")
		So(key2, ShouldEqual, "test.go:456")
		So(key3, ShouldEqual, "other.go:123")

		// 验证不同的文件或行号生成不同的键
		So(key1, ShouldNotEqual, key2)
		So(key1, ShouldNotEqual, key3)
		So(key2, ShouldNotEqual, key3)
	})
}

// TestConcurrentFieldMapCache 测试并发字段缓存
func TestConcurrentFieldMapCache(t *testing.T) {
	Convey("测试并发字段缓存", t, func() {
		type TestStruct struct {
			ID   int64  `borm:"id"`
			Name string `borm:"name"`
		}

		table := &BormTable{
			Cfg: Config{UseNameWhenTagEmpty: true},
		}

		// 清空缓存
		table.fieldMapCache = sync.Map{}

		rt := reflect2.TypeOf(TestStruct{})
		structType := rt.(reflect2.StructType)

		// 并发调用getStructFieldMap
		done := make(chan bool, 10)
		results := make([]map[string]reflect2.StructField, 10)

		for i := 0; i < 10; i++ {
			go func(index int) {
				results[index] = table.getStructFieldMap(structType)
				done <- true
			}(i)
		}

		// 等待所有goroutine完成
		for i := 0; i < 10; i++ {
			<-done
		}

		// 验证所有结果都相同（缓存一致性）
		for i := 1; i < 10; i++ {
			So(results[i], ShouldEqual, results[0])
		}
	})
}

// TestReusePotentialBugs 测试Reuse功能可能的问题
func TestReusePotentialBugs(t *testing.T) {
	Convey("测试Reuse功能可能的问题", t, func() {
		// 测试1: 不同调用位置使用相同的缓存键
		Convey("测试不同调用位置使用相同的缓存键", func() {
			// 模拟相同的文件名和行号
			item1 := &DataBindingItem{
				SQL:  "SELECT * FROM table1",
				Cols: []interface{}{"id", "name"},
			}

			item2 := &DataBindingItem{
				SQL:  "SELECT * FROM table2",
				Cols: []interface{}{"id", "email"},
			}

			// 使用相同的文件位置存储不同的数据
			storeToCache("test.go", 100, item1)
			storeToCache("test.go", 100, item2) // 覆盖了item1

			// 加载缓存
			loadedItem := loadFromCache("test.go", 100)
			So(loadedItem, ShouldNotBeNil)
			So(loadedItem.SQL, ShouldEqual, "SELECT * FROM table2") // 应该是item2
			So(loadedItem.Cols[1], ShouldEqual, "email")            // 应该是email，不是name
		})

		// 测试2: 缓存键冲突
		Convey("测试缓存键冲突", func() {
			// 清空缓存
			_dataBindingCache = sync.Map{}

			item1 := &DataBindingItem{SQL: "SELECT * FROM users"}
			item2 := &DataBindingItem{SQL: "SELECT * FROM orders"}

			// 使用可能冲突的键
			storeToCache("file.go", 1, item1)
			storeToCache("file.go", 1, item2) // 相同键，会覆盖

			loadedItem := loadFromCache("file.go", 1)
			So(loadedItem.SQL, ShouldEqual, "SELECT * FROM orders")
		})

		// 测试3: 字段缓存与数据绑定缓存的独立性
		Convey("测试字段缓存与数据绑定缓存的独立性", func() {
			type TestStruct struct {
				ID   int64  `borm:"id"`
				Name string `borm:"name"`
			}

			table := &BormTable{
				Cfg: Config{UseNameWhenTagEmpty: true},
			}

			// 清空字段缓存
			table.fieldMapCache = sync.Map{}

			rt := reflect2.TypeOf(TestStruct{})
			structType := rt.(reflect2.StructType)

			// 获取字段映射
			fieldMap := table.getStructFieldMap(structType)
			So(fieldMap, ShouldNotBeNil)

			// 字段缓存应该独立于数据绑定缓存
			// 数据绑定缓存存储的是SQL和参数
			// 字段缓存存储的是结构体字段映射
			So(len(fieldMap), ShouldEqual, 2)
		})
	})
}

// TestReuseWithDifferentStructs 测试Reuse功能与不同结构体
func TestReuseWithDifferentStructs(t *testing.T) {
	Convey("测试Reuse功能与不同结构体", t, func() {
		type Struct1 struct {
			ID   int64  `borm:"id"`
			Name string `borm:"name"`
		}

		type Struct2 struct {
			ID    int64  `borm:"id"`
			Email string `borm:"email"`
		}

		table := &BormTable{
			Cfg: Config{UseNameWhenTagEmpty: true},
		}

		// 清空字段缓存
		table.fieldMapCache = sync.Map{}

		// 测试不同结构体的字段缓存
		rt1 := reflect2.TypeOf(Struct1{})
		structType1 := rt1.(reflect2.StructType)
		fieldMap1 := table.getStructFieldMap(structType1)

		rt2 := reflect2.TypeOf(Struct2{})
		structType2 := rt2.(reflect2.StructType)
		fieldMap2 := table.getStructFieldMap(structType2)

		// 验证不同结构体有不同的字段映射
		So(fieldMap1, ShouldNotEqual, fieldMap2)
		So(len(fieldMap1), ShouldEqual, 2)
		So(len(fieldMap2), ShouldEqual, 2)
		So(fieldMap1["name"], ShouldNotBeNil)
		So(fieldMap2["email"], ShouldNotBeNil)
		So(fieldMap1["email"], ShouldBeNil)
		So(fieldMap2["name"], ShouldBeNil)
	})
}

// TestReuseCacheMemoryLeak 测试Reuse缓存内存泄漏
func TestReuseCacheMemoryLeak(t *testing.T) {
	Convey("测试Reuse缓存内存泄漏", t, func() {
		// 清空缓存
		_dataBindingCache = sync.Map{}

		// 存储大量缓存项
		for i := 0; i < 1000; i++ {
			item := &DataBindingItem{
				SQL:  fmt.Sprintf("SELECT * FROM table%d", i),
				Cols: []interface{}{"id", "name"},
			}
			storeToCache(fmt.Sprintf("file%d.go", i), i, item)
		}

		// 验证缓存项数量
		count := 0
		_dataBindingCache.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		So(count, ShouldEqual, 1000)

		// 测试缓存项访问
		item := loadFromCache("file500.go", 500)
		So(item, ShouldNotBeNil)
		So(item.SQL, ShouldEqual, "SELECT * FROM table500")
	})
}

// TestReuseWithNilValues 测试Reuse功能与nil值
func TestReuseWithNilValues(t *testing.T) {
	Convey("测试Reuse功能与nil值", t, func() {
		// 测试存储nil值
		storeToCache("test.go", 1, nil)

		// 加载nil值
		item := loadFromCache("test.go", 1)
		So(item, ShouldBeNil)

		// 测试不存在的键
		notFound := loadFromCache("nonexistent.go", 999)
		So(notFound, ShouldBeNil)
	})
}

// TestSelectWithIgnoredField 测试Select时忽略borm:"-"字段
func TestSelectWithIgnoredField(t *testing.T) {
	Convey("测试Select时忽略borm:\"-\"字段", t, func() {
		type TestStruct struct {
			ID   int64  `borm:"id"`
			Name string `borm:"name"`
			Pass string `borm:"-"` // 应该被忽略的字段
		}

		// 创建测试表
		createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_ignore_field (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100)
		)`
		_, err := db.Exec(createTableSQL)
		So(err, ShouldBeNil)

		// 清理测试数据
		defer func() {
			db.Exec("DELETE FROM test_ignore_field")
		}()

		// 插入测试数据
		tbl := Table(db, "test_ignore_field")
		_, err = db.Exec("INSERT INTO test_ignore_field (name) VALUES (?)", "test")
		So(err, ShouldBeNil)

		// 测试Select所有字段（不指定Fields）
		Convey("Select所有字段时应该忽略borm:\"-\"字段", func() {
			var result TestStruct
			n, err := tbl.Select(&result, Where("id = ?", 1))
			
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
			So(result.Name, ShouldEqual, "test")
		})
	})
}
