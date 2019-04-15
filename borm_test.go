package borm

import (
	"context"
	"database/sql"
	"testing"

	"fmt"
	"log"
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

type x1 struct {
	X     string    `borm:"name"`
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

func TestSelect(t *testing.T) {

	Convey("normal", t, func() {

		Convey("single select", func() {
			var o x
			tbl := Table(db, "test").Reuse()

			for i := 0; i < 10; i++ {
				n, err := tbl.Select(&o, Where("`id` >= ?", 1), Limit(100))

				So(err, ShouldBeNil)
				So(n, ShouldEqual, 1)
				fmt.Printf("%+v\n", o)
			}
		})

		Convey("multiple select", func() {
			var o []x
			tbl := Table(db, "test").Debug()

			n, err := tbl.Select(&o, Where(Gte("id", 0)), OrderBy("id", "name"), Limit(0, 100))

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

			n, err := tbl.Select(&o, GroupBy("id", `name`), Limit(100))

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

			n, err := tbl.Select(&cnt, Fields("count(1)"), Where("`id` >= ?", 1), Limit(100))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
			fmt.Printf("%+v\n", cnt)
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

		Convey("single replace", func() {
			o := x{
				X:  "Orca1",
				Y:  20,
				Z1: 1551405784,
			}
			tbl := Table(db, "test").ReplaceInto().Debug()

			n, err := tbl.Insert(&o)

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
			tbl := Table(db, "test").InsertIgnore().Debug()

			n, err := tbl.Insert(&o)

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

			n, err := tbl.Insert(&o, Fields("name", "ctime", "age"), OnDuplicateKeyUpdate(map[string]interface{}{
				"name": "OnDuplicateKeyUpdate",
			}))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
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

			n, err := tbl.Update(map[string]interface{}{
				"name": "OrcaUpdated",
				"age":  88,
			}, Where("id = ?", 0))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})

		Convey("update with map & Fields", func() {
			tbl := Table(db, "test").Debug()

			n, err := tbl.Update(map[string]interface{}{
				"name": "OrcaUpdatedFields",
				"age":  88,
			}, Fields("name", "age"), Where("id = ?", 0))

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

			n, err := tbl.Delete(Where("`id`=0"))

			So(err, ShouldBeNil)
			So(n, ShouldBeGreaterThan, 0)
		})

		Convey("bulk delete", func() {
			tbl := Table(db, "test").Debug()

			n, err := tbl.Delete(Where("`id`=0"), Limit(100))

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
			So(i, ShouldEqual, time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix())
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

func TestMock(t *testing.T) {
	Convey("Mock one func", t, func() {
		Convey("tests", func() {
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
	})
}

func PanicCheck(f func ()) (err interface{}) {
	defer func() {
		if t := recover(); t != nil {
			err = t
		}
	}()
	f()
	return
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
			So(PanicCheck(func () {
				Where()
			}), ShouldNotBeNil)
		})
	})

	Convey("Limit", t, func() {
		Convey("Limit panic", func() {
			So(PanicCheck(func () {
				Limit()
			}), ShouldNotBeNil)
		})
		Convey("Limit panic 2", func() {
			So(PanicCheck(func () {
				Limit(1, 2, 3)
			}), ShouldNotBeNil)
		})
	})

	Convey("Select", t, func() {
		Convey("Select arg len err", func() {
			t := Table(db, "test", context.TODO())

			var o x
			_, err := t.Select(&o)
			So(err, ShouldNotBeNil)
		})
		Convey("Select arg type err", func() {
			t := Table(db, "test", context.TODO())

			var o x
			_, err := t.Select(o)
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Select - Reuse", t, func() {
		// TODO
	})

	Convey("Select - UseNameWhenTagEmpty", t, func() {
		t := Table(db, "test", context.TODO())

		var o x1
		_, err := t.Select(&o)
		So(err, ShouldNotBeNil)
		So(o.CTime(), ShouldEqual, 0)

		_, err = t.UseNameWhenTagEmpty().Select(&o)
		So(err, ShouldNotBeNil)
		So(o.CTime(), ShouldNotEqual, 0)
	})

	Convey("Select - other type with Fields", t, func() {
		t := Table(db, "test", context.TODO())

		var cnt int64
		_, err := t.Select(&cnt, Where("`id` >= ?", 1))
		So(err, ShouldNotBeNil)

		_, err = t.Select(&cnt, Fields())
		So(err, ShouldNotBeNil)
	})

	Convey("Select - empty single result", t, func() {
		t := Table(db, "test", context.TODO())

		var o x
		n, err := t.Select(&o, Where("`id` >= ?", 1011))
		So(err, ShouldBeNil)
		So(n, ShouldEqual, 0)
	})

	Convey("Select - sql error", t, func() {
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
}
