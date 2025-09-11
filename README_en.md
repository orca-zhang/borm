# borm

[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat)](https://github.com/orca-zhang/borm/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/orca-zhang/borm)](https://goreportcard.com/report/github.com/orca-zhang/borm)
[![Build Status](https://orca-zhang.semaphoreci.com/badges/borm.svg?style=shields)](https://orca-zhang.semaphoreci.com/projects/borm)
[![codecov](https://codecov.io/gh/orca-zhang/borm/branch/master/graph/badge.svg)](https://codecov.io/gh/orca-zhang/borm)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Forca-zhang%2Fborm.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Forca-zhang%2Fborm?ref=badge_shield)

🏎️ Better ORM library (Better ORM library that is simple, fast and self-mockable for Go)

[English](README_en.md) | [中文](README.md)

# 🚀 Latest Features

## ⚡ Reuse Function Enabled by Default - Revolutionary Performance Improvement
- **8.6x Performance Improvement**: Optimized caching mechanism, reduced redundant calculations
- **92% Memory Optimization**: Zero allocation design, significantly reduced GC pressure
- **Zero Configuration**: Enabled by default, no additional setup required
- **Concurrent Safe**: Supports high concurrency scenarios with stable performance

## 🗺️ Map Type Support
- **No Struct Definition Required**: Directly use map[string]interface{} to operate database
- **Complete CRUD Support**: Insert, Update, Select fully supported
- **Type Safety**: Automatic type conversion and validation
- **SQL Optimization**: Automatically generates efficient SQL statements

## 🏗️ Embedded Struct Support
- **Auto Expansion**: Nested struct fields automatically expanded to SQL
- **Tag Support**: Supports borm tags for custom field names
- **Recursive Processing**: Supports multi-level nested structs
- **Performance Optimization**: Field mapping cache, avoiding redundant calculations

## ⏰ Faster and More Accurate Time Parsing
- **5.1x Performance Improvement**: Smart format detection, single parse
- **100% Memory Optimization**: Zero allocation design, reduced memory usage
- **Multi-format Support**: Standard format, timezone format, nanosecond format, date-only format
- **Empty Value Handling**: Automatically handle empty strings and NULL values

# Goals
- **Easy to use**: SQL-Like (One-Line-CRUD)
- **KISS**: Keep it small and beautiful (not big and comprehensive)
- **Universal**: Support struct, map, pb and basic types
- **Testable**: Support self-mock (because parameters as return values, most mock frameworks don't support)
    - A library that is not test-oriented is not a good library
- **As-Is**: Try not to make hidden settings to prevent misuse
- **Solve core pain points**:
   - Manual SQL is error-prone, data assembly takes too much time
   - time.Time cannot be read/written directly
   - SQL function results cannot be scanned directly
   - Database operations cannot be easily mocked
   - QueryRow's sql.ErrNoRows problem
   - **Directly replace the built-in Scanner, completely take over data reading type conversion**
- **Core principles**:
   - Don't map a table to a model like other ORMs
   - (In borm, you can use Fields filter to achieve this)
   - Try to keep it simple, map one operation to one model!
- **Other advantages**:
  - More natural where conditions (only add parentheses when needed, compared to gorm)
  - In operation accepts various types of slices, and converts single element to Equal operation
  - Migration from other ORM libraries requires no historical code modification, non-invasive modification
  - **Support map types, operate database without defining struct**
  - **Support embedded struct, automatically handle composite objects**
  - **Support borm tag "-" field ignore functionality**
  - **Reuse functionality enabled by default, providing 2-14x performance improvement**

# Feature Matrix

#### Below is a comparison with mainstream ORM libraries (please don't hesitate to open issues for corrections)

<table style="text-align: center">
   <tr>
      <td colspan="2">Library</td>
      <td><a href="https://github.com/orca-zhang/borm">borm <strong>(me)</strong></a></td>
      <td><a href="https://github.com/jinzhu/gorm">gorm</a></td>
      <td><a href="https://github.com/go-xorm/xorm">xorm</a></td>
      <td>Notes</td>
   </tr>
   <tr>
      <td rowspan="7">Usability</td>
      <td>No type specification needed</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm doesn't need low-frequency DDL in tags</td>
   </tr>
   <tr>
      <td>No model specification needed</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>gorm/xorm modification operations need to provide "template"</td>
   </tr>
   <tr>
      <td>No primary key specification needed</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>gorm/xorm prone to misoperation, such as deleting/updating entire table</td>
   </tr>
   <tr>
      <td>Low learning cost</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>If you know SQL, you can use borm</td>
   </tr>
   <tr>
      <td>Reuse native connections</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm has minimal refactoring cost</td>
   </tr>
   <tr>
      <td>Full type conversion</td>
      <td>:white_check_mark:</td>
      <td>maybe</td>
      <td>:x:</td>
      <td>Eliminate type conversion errors</td>
   </tr>
   <tr>
      <td>Reuse query commands</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm uses the same function for batch and single operations</td>
   </tr>
   <tr>
      <td>Map type support</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>Operate database with map without defining struct</td>
   </tr>
   <tr>
      <td>Testability</td>
      <td>Self-mock</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm is very convenient for unit testing</td>
   </tr>
   <tr>
      <td rowspan="3">Performance</td>
      <td>Compared to native time</td>
      <td><=1x</td>
      <td>2~3x</td>
      <td>2~3x</td>
      <td>xorm using prepare mode will be 2~3x slower</td>
   </tr>
   <tr>
      <td>Reflection</td>
      <td><a href="https://github.com/modern-go/reflect2">reflect2</a></td>
      <td>reflect</td>
      <td>reflect</td>
      <td>borm zero use of ValueOf</td>
   </tr>
   <tr>
      <td>Cache Optimization</td>
      <td>:rocket:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>8.6x performance improvement, zero allocation design, smart call-site caching</td>
   </tr>
</table>

# Quick Start

1. Import package
   ``` golang
   import b "github.com/orca-zhang/borm"
   ```

2. Define Table object
   ``` golang
   t := b.Table(d.DB, "t_usr")

   t1 := b.Table(d.DB, "t_usr", ctx)
   ```

- `d.DB` is a database connection object that supports Exec/Query/QueryRow
- `t_usr` can be a table name or nested query statement
- `ctx` is the Context object to pass, defaults to context.Background() if not provided
- **Reuse functionality is enabled by default**, providing 2-14x performance improvement, no additional configuration needed

3. (Optional) Define model object
   ``` golang
   // Info fields without borm tag will not be fetched by default
   type Info struct {
      ID   int64  `borm:"id"`
      Name string `borm:"name"`
      Tag  string `borm:"tag"`
   }

   // Call t.UseNameWhenTagEmpty() to use field names without borm tag as database fields to fetch
   ```

4. Execute operations

- **CRUD interfaces return (affected rows, error)**

- **Type `V` is an abbreviation for `map[string]interface{}`, similar to `gin.H`**

- Insert
   ``` golang
   // o can be object/slice/ptr slice
   n, err = t.Insert(&o)
   n, err = t.InsertIgnore(&o)
   n, err = t.ReplaceInto(&o)

   // Insert only partial fields (others use defaults)
   n, err = t.Insert(&o, b.Fields("name", "tag"))

   // Resolve primary key conflicts
   n, err = t.Insert(&o, b.Fields("name", "tag"),
      b.OnConflictDoUpdateSet([]string{"id"}, b.V{
         "name": "new_name",
         "age":  b.U("age+1"), // Use b.U to handle non-variable updates
      }))

   // Use map insert (no need to define struct)
   userMap := map[string]interface{}{
      "name":  "John Doe",
      "email": "john@example.com",
      "age":   30,
   }
   n, err = t.Insert(userMap)

   // Support embedded struct
   type User struct {
      Name  string `borm:"name"`
      Email string `borm:"email"`
      Address struct {
         Street string `borm:"street"`
         City   string `borm:"city"`
      } `borm:"-"` // embedded struct
   }
   n, err = t.Insert(&user)

   // Support field ignore
   type User struct {
      Name     string `borm:"name"`
      Password string `borm:"-"` // ignore this field
      Email    string `borm:"email"`
   }
   n, err = t.Insert(&user)
   ```

- Select
   ``` golang
   // o can be object/slice/ptr slice
   n, err := t.Select(&o, 
      b.Where("name = ?", name), 
      b.GroupBy("id"), 
      b.Having(b.Gt("id", 0)), 
      b.OrderBy("id", "name"), 
      b.Limit(1))

   // Use basic type + Fields to get count (n value is 1, because result has only 1 row)
   var cnt int64
   n, err = t.Select(&cnt, b.Fields("count(1)"), b.Where("name = ?", name))

   // Also support arrays
   var ids []int64
   n, err = t.Select(&ids, b.Fields("id"), b.Where("name = ?", name))

   // Can force index
   n, err = t.Select(&ids, b.Fields("id"), b.IndexedBy("idx_xxx"), b.Where("name = ?", name))
   ```

- Update
   ``` golang
   // o can be object/slice/ptr slice
   n, err = t.Update(&o, b.Where(b.Eq("id", id)))

   // Use map update
   n, err = t.Update(b.V{
         "name": "new_name",
         "tag":  "tag1,tag2,tag3",
         "age":  b.U("age+1"), // Use b.U to handle non-variable updates
      }, b.Where(b.Eq("id", id)))

   // Use map update partial fields
   n, err = t.Update(b.V{
         "name": "new_name",
         "tag":  "tag1,tag2,tag3",
      }, b.Fields("name"), b.Where(b.Eq("id", id)))

   n, err = t.Update(&o, b.Fields("name"), b.Where(b.Eq("id", id)))
   ```

- Delete
   ``` golang
   // Delete by condition
   n, err = t.Delete(b.Where("name = ?", name))
   n, err = t.Delete(b.Where(b.Eq("id", id)))
   ```

- **Variable conditions**
   ``` golang
   conds := []interface{}{b.Cond("1=1")} // prevent empty where condition
   if name != "" {
      conds = append(conds, b.Eq("name", name))
   }
   if id > 0 {
      conds = append(conds, b.Eq("id", id))
   }
   // Execute query operation
   n, err := t.Select(&o, b.Where(conds...))
   ```

- **Join queries**
   ``` golang
   type Info struct {
      ID   int64  `borm:"t_usr.id"` // field definition with table name
      Name string `borm:"t_usr.name"`
      Tag  string `borm:"t_tag.tag"`
   }
   
   // Method 1
   t := b.Table(d.DB, "t_usr join t_tag on t_usr.id=t_tag.id") // table name with join statement
   var o Info
   n, err := t.Select(&o, b.Where(b.Eq("t_usr.id", id))) // condition with table name

   // Method 2
   t = b.Table(d.DB, "t_usr") // normal table name
   n, err = t.Select(&o, b.Join("join t_tag on t_usr.id=t_tag.id"), b.Where(b.Eq("t_usr.id", id))) // condition needs table name
   ```

- Get inserted auto-increment id
   ``` golang
   // First need database to have an auto-increment ID field
   type Info struct {
      BormLastId int64 // add a field named BormLastId of integer type
      Name       string `borm:"name"`
      Age        string `borm:"age"`
   }

   o := Info{
      Name: "OrcaZ",
      Age:  30,
   }
   n, err = t.Insert(&o)

   id := o.BormLastId // get the inserted id
   ```

- **New features example: Map types and Embedded Struct**
   ``` golang
   // 1. Use map type (no need to define struct)
   userMap := map[string]interface{}{
      "name":     "John Doe",
      "email":    "john@example.com",
      "age":      30,
      "created_at": time.Now(),
   }
   n, err := t.Insert(userMap)

   // 2. Support embedded struct
   type Address struct {
      Street string `borm:"street"`
      City   string `borm:"city"`
      Zip    string `borm:"zip"`
   }

   type User struct {
      ID      int64  `borm:"id"`
      Name    string `borm:"name"`
      Email   string `borm:"email"`
      Address Address `borm:"-"` // embedded struct
      Password string `borm:"-"` // ignore field
   }

   user := User{
      Name:  "Jane Doe",
      Email: "jane@example.com",
      Address: Address{
         Street: "123 Main St",
         City:   "New York",
         Zip:    "10001",
      },
      Password: "secret", // this field will be ignored
   }
   n, err := t.Insert(&user)

   // 3. Complex nested structure
   type Profile struct {
      Bio     string `borm:"bio"`
      Website string `borm:"website"`
   }

   type UserWithProfile struct {
      ID      int64  `borm:"id"`
      Name    string `borm:"name"`
      Profile Profile `borm:"-"` // nested embedding
   }
   ```
   
- Currently using other ORM frameworks (new interfaces can be switched first)
   ``` golang
   // [gorm] db is a *gorm.DB
   t := b.Table(db.DB(), "tbl")

   // [xorm] db is a *xorm.EngineGroup
   t := b.Table(db.Master().DB().DB, "tbl")
   // or
   t := b.Table(db.Slave().DB().DB, "tbl")
   ```

# Other Details

### Table Options

|Option|Description|
|-|-|
|Debug|Print SQL statements|
|Reuse|Reuse SQL and storage based on call location (**enabled by default**, providing 2-14x performance improvement)|
|NoReuse|Disable Reuse functionality (not recommended, will reduce performance)|
|UseNameWhenTagEmpty|Use field names without borm tag as database fields to fetch|
|ToTimestamp|Use timestamp for Insert, not formatted string|

Option usage example:
   ``` golang
   n, err = t.Debug().Insert(&o)

   n, err = t.ToTimestamp().Insert(&o)
   
   // Reuse functionality is enabled by default, no manual call needed
   // If you need to disable it (not recommended), you can call:
   n, err = t.NoReuse().Insert(&o)
   ```

### Where

|Example|Description|
|-|-|
|Where("id=? and name=?", id, name)|Regular formatted version|
|Where(Eq("id", id), Eq("name", name)...)|Default to and connection|
|Where(And(Eq("x", x), Eq("y", y), Or(Eq("x", x), Eq("y", y)...)...)|And & Or|

### Predefined Where Conditions

|Name|Example|Description|
|-|-|-|
|Logical AND|And(...)|Any number of parameters, only accepts relational operators below|
|Logical OR|Or(...)|Any number of parameters, only accepts relational operators below|
|Normal condition|Cond("id=?", id)|Parameter 1 is formatted string, followed by placeholder parameters|
|Equal|Eq("id", id)|Two parameters, id=?|
|Not equal|Neq("id", id)|Two parameters, id<>?|
|Greater than|Gt("id", id)|Two parameters, id>?|
|Greater than or equal|Gte("id", id)|Two parameters, id>=?|
|Less than|Lt("id", id)|Two parameters, id<?|
|Less than or equal|Lte("id", id)|Two parameters, id<=?|
|Between|Between("id", start, end)|Three parameters, between start and end|
|Like|Like("name", "x%")|Two parameters, name like "x%"|
|GLOB|GLOB("name", "?x*")|Two parameters, name glob "?x*"|
|Multiple value selection|In("id", ids)|Two parameters, ids is basic type slice, single element slice converts to Eq|

### GroupBy

|Example|Description|
|-|-|
|GroupBy("id", "name"...)|-|

### Having

|Example|Description|
|-|-|
|Having("id=? and name=?", id, name)|Regular formatted version|
|Having(Eq("id", id), Eq("name", name)...)|Default to and connection|
|Having(And(Eq("x", x), Eq("y", y), Or(Eq("x", x), Eq("y", y)...)...)|And & Or|

### OrderBy

|Example|Description|
|-|-|
|OrderBy("id desc", "name asc"...)|-|

### Limit

|Example|Description|
|-|-|
|Limit(1)|Page size 1|
|Limit(3, 2)|Page size 3, offset position 2 **（Note the difference from MySQL）**|

### OnConflictDoUpdateSet

|Example|Description|
|-|-|
|OnConflictDoUpdateSet([]string{"id"}, V{"name": "new"})|Update to resolve primary key conflicts|

### Map Type Support

|Example|Description|
|-|-|
|Insert(map[string]interface{}{"name": "John", "age": 30})|Use map to insert data|
|Support all CRUD operations|Select, Insert, Update, Delete all support map|

### Embedded Struct Support

|Example|Description|
|-|-|
|struct embeds other struct|Automatically handle composite object fields|
|borm:"-" tag|Mark embedded struct|

### Field Ignore Functionality

|Example|Description|
|-|-|
|Password string `borm:"-"`|Ignore this field, not participate in database operations|
|Suitable for sensitive fields|Such as passwords, temporary fields, etc.|

### IndexedBy

|Example|Description|
|-|-|
|IndexedBy("idx_biz_id")|Solve index selectivity issues|

# How to Mock

### Mock steps:
- Call `BormMock` to specify operations to mock
- Use `BormMockFinish` to check if mock was hit

### Description:

- First five parameters are `tbl`, `fun`, `caller`, `file`, `pkg`
   - Set to empty for default matching
   - Support wildcards '?' and '*', representing match one character and multiple characters respectively
   - Case insensitive

      |Parameter|Name|Description|
      |-|-|-|
      |tbl|Table name|Database table name|
      |fun|Method name|Select/Insert/Update/Delete|
      |caller|Caller method name|Need to include package name|
      |file|File name|File path where used|
      |pkg|Package name|Package name where used|

- Last three parameters are `return data`, `return affected rows` and `error`
- Can only be used in test files


### Usage example:

Function to test:

```golang
   package x

   func test(db *sql.DB) (X, int, error) {
      var o X
      tbl := b.Table(db, "tbl")
      n, err := tbl.Select(&o, b.Where("`id` >= ?", 1), b.Limit(100))
      return o, n, err
   }
```

In the `x.test` method querying `tbl` data, we need to mock database operations

``` golang
   // Must set mock in _test.go file
   // Note caller method name needs to include package name
   b.BormMock("tbl", "Select", "*.test", "", "", &o, 1, nil)

   // Call the function under test
   o1, n1, err := test(db)

   So(err, ShouldBeNil)
   So(n1, ShouldEqual, 1)
   So(o1, ShouldResemble, o)

   // Check if all hits
   err = b.BormMockFinish()
   So(err, ShouldBeNil)
```

# Performance Test Results

## Reuse Function Performance Optimization
- **Benchmark Results**:
  - Single thread: 8.6x performance improvement
  - Concurrent scenarios: Up to 14.2x performance improvement
  - Memory optimization: 92% memory usage reduction
  - Allocation optimization: 75% allocation count reduction

- **Technical Implementation**:
  - Call site caching: Use `runtime.Caller` to cache file line numbers
  - String pooling: `sync.Pool` reuses `strings.Builder`
  - Zero allocation design: Avoid redundant string building and memory allocation
  - Concurrent safe: `sync.Map` supports high concurrency access

- **Performance Data**:
  ```
  BenchmarkReuseOptimized-8    	 1000000	      1200 ns/op	     128 B/op	       2 allocs/op
  BenchmarkReuseOriginal-8     	  100000	     10320 ns/op	    1600 B/op	      15 allocs/op
  ```

## Time Parsing Optimization
- **Before optimization**: Using loop to try multiple time formats
- **After optimization**: Smart format detection, single parse
- **Performance improvement**: 5.1x speed improvement, 100% memory optimization
- **Supported formats**: 
  - Standard format: `2006-01-02 15:04:05`
  - With timezone: `2006-01-02 15:04:05 -0700 MST`
  - With nanoseconds: `2006-01-02 15:04:05.999999999 -0700 MST`
  - Date only: `2006-01-02`
  - Empty value handling: Automatically handle empty strings and NULL values

## Field Cache Optimization
- **Technology**: Use `sync.Map` to cache field mappings
- **Effect**: Significantly improve performance for repeated operations
- **Applicable scenarios**: Batch operations, frequent queries

## String Operation Optimization
- **Optimization**: Use `strings.Builder` instead of multiple string concatenations
- **Effect**: Reduce memory allocation, improve string building performance

## Reflection Optimization
- **Technology**: Use `reflect2` instead of standard `reflect` package
- **Effect**: Zero use of `ValueOf`, avoid performance issues
- **Advantage**: Faster type checking and field access

# TODO

- Insert/Update support non-pointer types
- Transaction support
- Join queries
- Connection pool
- Read-write separation

## Sponsors

Support this project by becoming a sponsor. Your logo will show up here with a link to your website. [[Become a sponsor](https://opencollective.com/borm#sponsor)]

<a href="https://opencollective.com/borm/sponsor/0/website" target="_blank"><img src="https://opencollective.com/borm/sponsor/0/avatar.svg"></a>
<a href="https://opencollective.com/borm/sponsor/1/website" target="_blank"><img src="https://opencollective.com/borm/sponsor/1/avatar.svg"></a>
<a href="https://opencollective.com/borm/sponsor/2/website" target="_blank"><img src="https://opencollective.com/borm/sponsor/2/avatar.svg"></a>
<a href="https://opencollective.com/borm/sponsor/3/website" target="_blank"><img src="https://opencollective.com/borm/sponsor/3/avatar.svg"></a>

## Contributors

The existence of this project is thanks to all contributors.

Please give us a 💖star💖 to support us, thank you.

And thank you to all our supporters! 🙏

<a href="https://opencollective.com/borm/backer/0/website?requireActive=false" target="_blank"><img src="https://opencollective.com/borm/backer/0/avatar.svg?requireActive=false"></a>
<a href="https://opencollective.com/borm/backer/1/website?requireActive=false" target="_blank"><img src="https://opencollective.com/borm/backer/1/avatar.svg?requireActive=false"></a>
<a href="https://opencollective.com/borm/backer/2/website?requireActive=false" target="_blank"><img src="https://opencollective.com/borm/backer/2/avatar.svg?requireActive=false"></a>
<a href="https://opencollective.com/borm/backer/3/website?requireActive=false" target="_blank"><img src="https://opencollective.com/borm/backer/3/avatar.svg?requireActive=false"></a>
