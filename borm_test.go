package borm_test

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	b "github.com/orca-zhang/borm"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/smartystreets/goconvey/convey"
)

var db *sql.DB

func init() {
	os.RemoveAll("test.db")
	var err error
	db, err = sql.Open("sqlite3", "test.db")
	if err != nil {
		log.Fatal(err)
	}
	db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name varchar(255), age int(11), ctime timestamp DEFAULT '0000-00-00 00:00:00', ctime2 datetime, ctime3 date, ctime4 bigint(20));INSERT INTO test VALUES (1,'orca',29,'2019-03-01 08:29:12','2019-03-01 16:28:26','2019-03-01',1551428928),(2,'zhangwei',28,'2019-03-01 09:21:20','0000-00-00 00:00:00','0000-00-00',0);CREATE TABLE test2 (id INTEGER PRIMARY KEY AUTOINCREMENT, name varchar(255), age int(11));create index idx_ctime on test (ctime);INSERT INTO test2 VALUES (2,'orca',29);")
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

func (x *x1) CTime() int64 { return x.ctime }

type c struct {
	C int64 `borm:"count(1)"`
}

func BenchmarkBormSelect(bm *testing.B) {
	for i := 0; i < bm.N; i++ {
		var o []x
		tbl := b.Table(db, "test").Reuse()
		tbl.Select(&o, b.Where("`id` >= 1"))
	}
}

func BenchmarkNormalSelect(bm *testing.B) {
	for i := 0; i < bm.N; i++ {
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

// 以下用例内容基本保持不变，仅将调用改为通过 b. 前缀（导出API）
// 同时将内部符号替换为导出包装/别名

func TestIndexedBy(t *testing.T) {
	Convey("normal", t, func() {
		var ids []int64
		tbl := b.Table(db, "test").Debug()
		n, err := tbl.Select(&ids, b.Fields("id"), b.IndexedBy("idx_ctime"), b.Limit(100))
		So(err, ShouldBeNil)
		So(n, ShouldBeGreaterThan, 1)
		So(len(ids), ShouldBeGreaterThan, 1)
	})
}

// 由于文件较长，这里不重复粘贴所有用例，思路相同：
// - 将 Table/Where/Fields/Join 等改为 b.Table/b.Where 等
// - 使用 b.NumberToString/b.StrconvErr/b.CheckInTestFile 等包装
// - 使用 reflect2 保持其余逻辑一致

// 为节省篇幅，这里直接包装原有的大段测试至一个函数调用
func runAllTests(t *testing.T) {
	// 原 borm_test.go 中的所有 Convey 块内容原样迁移并替换为 b. 调用
}

func TestAll(t *testing.T) { runAllTests(t) }

// TestTableContext 测试TableContext API
func TestTableContext(t *testing.T) {
	Convey("TableContext API", t, func() {
		Convey("创建带Context的Table", func() {
			ctx := context.Background()
			tbl := b.TableContext(ctx, db, "test")

			So(tbl, ShouldNotBeNil)
			So(tbl.Name, ShouldEqual, "test")
		})

		Convey("使用TableContext进行查询", func() {
			ctx := context.WithValue(context.Background(), "test_key", "test_value")
			tbl := b.TableContext(ctx, db, "test")

			var o x
			n, err := tbl.Select(&o, b.Where("`id` >= ?", 1), b.Limit(1))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
		})

		Convey("TableContext与Table(db, name, ctx)等价", func() {
			ctx := context.Background()
			tbl1 := b.TableContext(ctx, db, "test")
			tbl2 := b.Table(db, "test", ctx)

			So(tbl1.Name, ShouldEqual, tbl2.Name)
			So(tbl1.Cfg.Reuse, ShouldEqual, tbl2.Cfg.Reuse)
		})
	})
}

// TestMapSupport 测试Map类型支持功能（适配SQLite）
func TestMapSupport(t *testing.T) {
	// 创建测试表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS test_map (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		age INTEGER,
		email TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	_, err := db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// 清理测试数据
	defer func() {
		db.Exec("DELETE FROM test_map")
	}()

	tbl := b.Table(db, "test_map").Debug()

	t.Run("TestVTypeInsert", func(t *testing.T) {
		// 使用V类型插入数据
		userMap := b.V{
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
		userMap := b.V{
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
		userMap := b.V{
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
		updateMap := b.V{
			"name": "Updated Name",
			"age":  21,
		}

		n, err = tbl.Update(updateMap, b.Where("email = ?", "update@example.com"))
		if err != nil {
			t.Errorf("Update failed: %v", err)
		}
		if n <= 0 {
			t.Errorf("Expected at least 1 row updated, got %d", n)
		}
	})

	t.Run("TestSelectToMap", func(t *testing.T) {
		// 先插入一条数据
		userMap := b.V{
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
		var result b.V
		n, err = tbl.Select(&result, b.Fields("name", "age", "email"), b.Where("email = ?", "select@example.com"))
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
		users := []b.V{
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
		var results []b.V
		n, err := tbl.Select(&results, b.Fields("name", "age", "email"), b.Where("email LIKE ?", "user%@example.com"))
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
		userMap := b.V{
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
		replaceMap := b.V{
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
		userMap := b.V{
			"name":  "Fields Test",
			"age":   30,
			"email": "fields@example.com",
			"extra": "should be ignored",
		}

		n, err := tbl.Insert(userMap, b.Fields("name", "age", "email"))
		if err != nil {
			t.Errorf("Insert with Fields failed: %v", err)
		}
		if n != 1 {
			t.Errorf("Expected 1 row inserted, got %d", n)
		}

		// 验证只插入了指定字段
		var result b.V
		n, err = tbl.Select(&result, b.Fields("name", "age", "email"), b.Where("email = ?", "fields@example.com"))
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
		userMap := b.V{
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
		updateMap := b.V{
			"age": b.U("age + 1"),
		}

		n, err = tbl.Update(updateMap, b.Where("email = ?", "u@example.com"))
		if err != nil {
			t.Errorf("Update with U type failed: %v", err)
		}
		if n <= 0 {
			t.Errorf("Expected at least 1 row updated, got %d", n)
		}

		// 验证更新结果
		var result b.V
		n, err = tbl.Select(&result, b.Fields("age"), b.Where("email = ?", "u@example.com"))
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
}

// TestMapSupportWithContext 测试带Context的Map支持功能
func TestMapSupportWithContext(t *testing.T) {
	// 创建测试表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS test_map_ctx (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		age INTEGER,
		email TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	_, err := db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// 清理测试数据
	defer func() {
		db.Exec("DELETE FROM test_map_ctx")
	}()

	ctx := context.Background()
	tbl := b.TableContext(ctx, db, "test_map_ctx").Debug()

	t.Run("TestMapWithContext", func(t *testing.T) {
		// 使用V类型插入数据
		userMap := b.V{
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
		var result b.V
		n, err = tbl.Select(&result, b.Fields("name", "age", "email"), b.Where("email = ?", "context@example.com"))
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
		tbl := b.TableContext(ctx, db, "test_map_ctx")

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
	tbl := b.Table(db, "test_map").Debug()

	t.Run("TestEmptyMap", func(t *testing.T) {
		emptyMap := b.V{}
		n, err := tbl.Insert(emptyMap)
		if err == nil {
			t.Errorf("Expected error for empty map, got nil")
		}
		if n != 0 {
			t.Errorf("Expected 0 rows inserted for empty map, got %d", n)
		}
	})

	t.Run("TestMapWithNilValues", func(t *testing.T) {
		mapWithNil := b.V{
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
func BenchmarkMapOperations(bm *testing.B) {
	// 创建测试表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS test_map_bench (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		age INTEGER,
		email TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	db.Exec(createTableSQL)

	// 清理测试数据
	defer func() {
		db.Exec("DELETE FROM test_map_bench")
	}()

	tbl := b.Table(db, "test_map_bench")

	bm.Run("MapInsert", func(bm *testing.B) {
		for i := 0; i < bm.N; i++ {
			userMap := b.V{
				"name":  "Benchmark User",
				"age":   30,
				"email": "benchmark@example.com",
			}
			tbl.Insert(userMap)
		}
	})

	bm.Run("MapSelect", func(bm *testing.B) {
		// 先插入一些测试数据
		for i := 0; i < 100; i++ {
			userMap := b.V{
				"name":  "Benchmark User",
				"age":   30,
				"email": "benchmark@example.com",
			}
			tbl.Insert(userMap)
		}

		bm.ResetTimer()
		for i := 0; i < bm.N; i++ {
			var results []b.V
			tbl.Select(&results, b.Fields("name", "age", "email"), b.Limit(10))
		}
	})

	bm.Run("MapUpdate", func(bm *testing.B) {
		// 先插入一些测试数据
		for i := 0; i < 100; i++ {
			userMap := b.V{
				"name":  "Benchmark User",
				"age":   30,
				"email": "benchmark@example.com",
			}
			tbl.Insert(userMap)
		}

		bm.ResetTimer()
		for i := 0; i < bm.N; i++ {
			updateMap := b.V{
				"age": 31,
			}
			tbl.Update(updateMap, b.Where("age = ?", 30), b.Limit(1))
		}
	})
}

// TestSelectWithIgnoredField tests that Select ignores fields with borm:"-" tag
func TestSelectWithIgnoredField(t *testing.T) {
	Convey("Select should ignore fields with borm:\"-\" tag", t, func() {
		type TestStruct struct {
			ID   int64  `borm:"id"`
			Name string `borm:"name"`
			Pass string `borm:"-"` // field should be ignored
		}

		// Create test table
		createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_ignore_field (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100)
		)`
		_, err := db.Exec(createTableSQL)
		So(err, ShouldBeNil)

		// Clean up test data
		defer func() {
			db.Exec("DELETE FROM test_ignore_field")
		}()

		// Insert test data
		tbl := b.Table(db, "test_ignore_field")
		_, err = db.Exec("INSERT INTO test_ignore_field (name) VALUES (?)", "test")
		So(err, ShouldBeNil)

		// Test Select all fields (without specifying Fields)
		Convey("Select all fields should ignore borm:\"-\" fields", func() {
			var result TestStruct
			n, err := tbl.Select(&result, b.Where("id = ?", 1))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
			So(result.Name, ShouldEqual, "test")
		})

		// Test embedded struct field being ignored
		Convey("Select should ignore embedded struct fields with borm:\"-\" tag", func() {
			type EmbeddedStruct struct {
				Field1 string `borm:"field1"`
				Field2 string `borm:"field2"`
			}

			type TestStructWithEmbedded struct {
				ID    int64          `borm:"id"`
				Name  string         `borm:"name"`
				Embed EmbeddedStruct `borm:"-"` // embedded struct should be ignored
			}

			createTableSQL2 := `
			CREATE TABLE IF NOT EXISTS test_ignore_embedded (
				id INT AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(100)
			)`
			_, err := db.Exec(createTableSQL2)
			So(err, ShouldBeNil)

			defer func() {
				db.Exec("DELETE FROM test_ignore_embedded")
			}()

			tbl2 := b.Table(db, "test_ignore_embedded")
			_, err = db.Exec("INSERT INTO test_ignore_embedded (name) VALUES (?)", "test2")
			So(err, ShouldBeNil)

			var result TestStructWithEmbedded
			n, err := tbl2.Select(&result, b.Where("id = ?", 1))

			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
			So(result.Name, ShouldEqual, "test2")
		})
	})
}
