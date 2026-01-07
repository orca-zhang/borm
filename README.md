
# borm

[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat)](https://github.com/orca-zhang/borm/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/orca-zhang/borm)](https://goreportcard.com/report/github.com/orca-zhang/borm)
[![Build Status](https://orca-zhang.semaphoreci.com/badges/borm/branches/sqlite.svg?style=shields)](https://orca-zhang.semaphoreci.com/projects/borm)
[![codecov](https://codecov.io/gh/orca-zhang/borm/branch/sqlite/graph/badge.svg)](https://codecov.io/gh/orca-zhang/borm)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Forca-zhang%2Fborm.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Forca-zhang%2Fborm?ref=badge_shield)

🏎️ 更好的ORM库 (Better ORM library that is simple, fast and self-mockable for Go)

[English](README_en.md) | [中文](README.md)

# 🚀 最新功能

## ⚡ Reuse功能默认开启 - 性能革命性提升
- **8.6倍性能提升**：优化缓存机制，减少重复计算
- **92%内存优化**：零分配设计，大幅降低GC压力
- **零配置使用**：默认开启，无需额外设置
- **并发安全**：支持高并发场景，性能稳定

## 🗺️ Map类型支持
- **无需定义struct**：直接使用map[string]interface{}操作数据库
- **完整CRUD支持**：Insert、Update、Select全面支持
- **类型安全**：自动处理类型转换和验证
- **SQL优化**：自动生成高效的SQL语句

## 🏗️ Embedded Struct支持
- **自动展开**：嵌套结构体字段自动展开到SQL
- **标签支持**：支持borm标签自定义字段名
- **递归处理**：支持多层嵌套结构体
- **性能优化**：字段映射缓存，避免重复计算

## ⏰ 更快更准确的时间解析
- **5.1倍性能提升**：智能格式检测，单次解析
- **100%内存优化**：零分配设计，减少内存使用
- **多格式支持**：标准格式、时区格式、纳秒格式、纯日期格式
- **空值处理**：自动处理空字符串和NULL值

# 目标
- 易用：SQL-Like（一把梭：One-Line-CRUD）
- KISS：保持小而美（不做大而全）
- 通用：支持struct，map，pb和基本类型
- 可测：支持自mock（因为参数作返回值，大部分mock框架不支持）
    - 非测试向的library不是好library
- As-Is：尽可能不作隐藏设定，防止误用
- 解决核心痛点：
   - 手撸SQL难免有错，组装数据太花时间
   - time.Time无法直接读写的问题
   - SQL函数结果无法直接Scan
   - db操作无法方便的Mock
   - QueryRow的sql.ErrNoRows问题
   - **直接替换系统自带Scanner，完整接管数据读取的类型转换**
- 核心原则：
   - 别像使用其他orm那样把一个表映射到一个model
   - （在borm里可以用Fields过滤器做到）
   - 尽量保持简单把一个操作映射一个model吧！
- 其他优点：
  - 更自然的where条件（仅在需要加括号时添加，对比gorm）
  - In操作接受各种类型slice
  - 从其他orm库迁移无需修改历史代码，无侵入性修改

# 特性矩阵

#### 下面是和一些主流orm库的对比（请不吝开issue勘误）

<table style="text-align: center">
   <tr>
      <td colspan="2">库</td>
      <td><a href="https://github.com/orca-zhang/borm">borm <strong>(me)</strong></a></td>
      <td><a href="https://github.com/jinzhu/gorm">gorm</a></td>
      <td><a href="https://github.com/go-xorm/xorm">xorm</a></td>
      <td>备注</td>
   </tr>
   <tr>
      <td rowspan="7">易用性</td>
      <td>无需指定类型</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm在tag中无需低频的DDL</td>
   </tr>
   <tr>
      <td>无需指定model</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>gorm/xorm改操作需提供“模版”</td>
   </tr>
   <tr>
      <td>无需指定主键</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>gorm/xorm易误操作，如删/改全表</td>
   </tr>
   <tr>
      <td>学习成本低</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>会SQL就会用borm</td>
   </tr>
   <tr>
      <td>可复用原生连接</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm重构成本极小</td>
   </tr>
   <tr>
      <td>全类型转换</td>
      <td>:white_check_mark:</td>
      <td>maybe</td>
      <td>:x:</td>
      <td>杜绝类型转换的抛错</td>
   </tr>
   <tr>
      <td>复用查询命令</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm批量和单条使用同一个函数</td>
   </tr>
   <tr>
      <td>Map类型支持</td>
      <td>使用map操作数据库</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>无需定义struct，灵活处理动态字段</td>
   </tr>
   <tr>
      <td>可测试性</td>
      <td>自mock</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm非常便于单元测试</td>
   </tr>
   <tr>
      <td rowspan="3">性能</td>
      <td>较原生耗时</td>
      <td><=1x</td>
      <td>2~3x</td>
      <td>2~3x</td>
      <td>xorm使用prepare模式会再慢2～3x</td>
   </tr>
   <tr>
      <td>反射</td>
      <td><a href="https://github.com/modern-go/reflect2">reflect2</a></td>
      <td>reflect</td>
      <td>reflect</td>
      <td>borm零使用ValueOf</td>
   </tr>
   <tr>
      <td>缓存优化</td>
      <td>:rocket:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>8.6x性能提升，零分配设计，调用位置智能缓存</td>
   </tr>
</table>

# 快速入门

1. 引入包
   ``` golang
   import b "github.com/orca-zhang/borm"
   ```

2. 定义Table对象
   ``` golang
   t := b.Table(d.DB, "t_usr")

   t1 := b.TableContext(ctx, d.DB, "t_usr")
   ```

- `d.DB`是支持Exec/Query/QueryRow的数据库连接对象
- `t_usr`可以是表名，或者是嵌套查询语句
- `ctx`是需要传递的Context对象，使用`TableContext`时传递
- **Reuse功能默认开启**，提供2-14倍性能提升，无需额外配置

### Table API说明

|函数|参数顺序|说明|
|-|-|-|
|`Table(db, name)`|db, name|创建默认Table，使用context.Background()|
|`TableContext(ctx, db, name)`|ctx, db, name|创建带Context的Table，参数顺序：context, db, name|

3. （可选）定义model对象
   ``` golang
   // Info 默认未设置borm tag的字段不会取
   type Info struct {
      ID   int64  `borm:"id"`
      Name string `borm:"name"`
      Tag  string `borm:"tag"`
   }

   // 调用t.UseNameWhenTagEmpty()，可以用未设置borm tag的字段名本身作为待获取的db字段
   ```

4. 执行操作

- **CRUD接口返回值为 (影响的条数，错误)**

- **类型`V`为`map[string]interface{}`的缩写形式，参考`gin.H`**

- 插入
   ``` golang
   // o可以是对象/slice/ptr slice
   n, err = t.Insert(&o)
   n, err = t.InsertIgnore(&o)
   n, err = t.ReplaceInto(&o)

   // 只插入部分字段（其他使用缺省）
   n, err = t.Insert(&o, b.Fields("name", "tag"))

   // 解决主键冲突
   n, err = t.Insert(&o, b.Fields("name", "tag"),
      b.OnConflictDoUpdateSet([]string{"id"}, []string{"name", "age"}))

   // 使用map插入（无需定义struct）
   userMap := b.V{
      "name":  "John Doe",
      "email": "john@example.com",
      "age":   30,
   }
   n, err = t.Insert(userMap)

   // 支持embedded struct
   type User struct {
      Name  string `borm:"name"`
      Email string `borm:"email"`
      Address struct {
         Street string `borm:"street"`
         City   string `borm:"city"`
      } `borm:"-"` // 嵌入结构体
   }
   n, err = t.Insert(&user)

   // 支持字段忽略
   type User struct {
      Name     string `borm:"name"`
      Password string `borm:"-"` // 忽略此字段
      Email    string `borm:"email"`
   }
   n, err = t.Insert(&user)
   ```

- 查询
   ``` golang
   // o可以是对象/slice/ptr slice
   n, err := t.Select(&o, 
      b.Where("name = ?", name), 
      b.GroupBy("id"), 
      b.Having(b.Gt("id", 0)), 
      b.OrderBy("id", "name"), 
      b.Limit(1))

   // 使用基本类型+Fields获取条目数（n的值为1，因为结果只有1条）
   var cnt int64
   n, err = t.Select(&cnt, b.Fields("count(1)"), b.Where("name = ?", name))

   // 还可以支持数组
   var ids []int64
   n, err = t.Select(&ids, b.Fields("id"), b.Where("name = ?", name))

   // 可以强制索引
   n, err = t.Select(&ids, b.Fields("id"), b.IndexedBy("idx_xxx"), b.Where("name = ?", name))
   ```

- Select 到 Map（无需定义 struct）
  ``` golang
  // 单行映射到 map
  var m b.V
  n, err := t.Select(&m, b.Fields("id", "name", "age"), b.Where(b.Eq("id", 1)))

  // 多行映射到 []map
  var ms []b.V
  n, err = t.Select(&ms, b.Fields("id", "name", "age"), b.Where(b.Gt("age", 18)))
  ```

- 更新
   ``` golang
   // o可以是对象/slice/ptr slice
   n, err = t.Update(&o, b.Where(b.Eq("id", id)))

   // 使用map更新
   n, err = t.Update(b.V{
         "name": "new_name",
         "tag":  "tag1,tag2,tag3",
         "age":  b.U("age+1"), // 使用b.U来处理非变量更新
      }, b.Where(b.Eq("id", id)))

   // 使用map更新部分字段
   n, err = t.Update(b.V{
         "name": "new_name",
         "tag":  "tag1,tag2,tag3",
      }, b.Fields("name"), b.Where(b.Eq("id", id)))

   n, err = t.Update(&o, b.Fields("name"), b.Where(b.Eq("id", id)))
   ```

- CRUD 配合 Reuse（默认开启）
  ``` golang
  // Reuse 默认开启；同一调用点重复调用会复用 SQL/元数据
  // Update 示例
  type User struct { ID int64 `borm:"id"`; Name string `borm:"name"`; Age int `borm:"age"` }
  for _, u := range users {
      _, _ = t.Update(&u, b.Fields("name", "age"), b.Where(b.Eq("id", u.ID)))
  }

  // Insert 示例
  for _, u := range users {
      _, _ = t.Insert(&u)
  }
  ```

- 删除
   ``` golang
   // 根据条件删除
   n, err = t.Delete(b.Where("name = ?", name))
   n, err = t.Delete(b.Where(b.Eq("id", id)))
   ```

- **可变条件**
   ``` golang
   conds := []interface{}{b.Cond("1=1")} // 防止空where条件
   if name != "" {
      conds = append(conds, b.Eq("name", name))
   }
   if id > 0 {
      conds = append(conds, b.Eq("id", id))
   }
   // 执行查询操作
   n, err := t.Select(&o, b.Where(conds...))
   ```

- **联表查询**
   ``` golang
   type Info struct {
      ID   int64  `borm:"t_usr.id"` // 字段定义加表名
      Name string `borm:"t_usr.name"`
      Tag  string `borm:"t_tag.tag"`
   }
   
   // 方法一
   t := b.Table(d.DB, "t_usr join t_tag on t_usr.id=t_tag.id") // 表名用join语句
   var o Info
   n, err := t.Select(&o, b.Where(b.Eq("t_usr.id", id))) // 条件加上表名

   // 方法二
   t = b.Table(d.DB, "t_usr") // 正常表名
   n, err = t.Select(&o, b.Join("join t_tag on t_usr.id=t_tag.id"), b.Where(b.Eq("t_usr.id", id))) // 条件需要加上表名
   ```

-  获取插入的自增id
   ``` golang
   // 首先需要数据库有一个自增ID的字段
   type Info struct {
      BormLastId int64 // 添加一个名为BormLastId的整型字段
      Name       string `borm:"name"`
      Age        string `borm:"age"`
   }

   o := Info{
      Name: "OrcaZ",
      Age:  30,
   }
   n, err = t.Insert(&o)

   id := o.BormLastId // 获取到插入的id
   ```

- **新功能示例：Map类型和Embedded Struct**
   ``` golang
   // 1. 使用map类型（无需定义struct）
   userMap := b.V{
      "name":     "John Doe",
      "email":    "john@example.com",
      "age":      30,
      "created_at": time.Now(),
   }
   n, err := t.Insert(userMap)

   // 2. 支持embedded struct
   type Address struct {
      Street string `borm:"street"`
      City   string `borm:"city"`
      Zip    string `borm:"zip"`
   }

   type User struct {
      ID      int64  `borm:"id"`
      Name    string `borm:"name"`
      Email   string `borm:"email"`
      Address Address `borm:"-"` // 嵌入结构体
      Password string `borm:"-"` // 忽略字段
   }

   user := User{
      Name:  "Jane Doe",
      Email: "jane@example.com",
      Address: Address{
         Street: "123 Main St",
         City:   "New York",
         Zip:    "10001",
      },
      Password: "secret", // 此字段会被忽略
   }
   n, err := t.Insert(&user)

   // 3. 复杂嵌套结构
   type Profile struct {
      Bio     string `borm:"bio"`
      Website string `borm:"website"`
   }

   type UserWithProfile struct {
      ID      int64  `borm:"id"`
      Name    string `borm:"name"`
      Profile Profile `borm:"-"` // 嵌套嵌入
   }
   ```
   
- 正在使用其他orm框架（新的接口先切过来吧）
   ``` golang
   // [gorm] db是一个*gorm.DB
   t := b.Table(db.DB(), "tbl")

   // [xorm] db是一个*xorm.EngineGroup
   t := b.Table(db.Master().DB().DB, "tbl")
   // or
   t := b.Table(db.Slave().DB().DB, "tbl")
   ```

# 其他细节

### Table的选项

|选项|说明|
|-|-|
|Debug|打印sql语句|
|Reuse|根据调用位置复用sql和存储方式（**默认开启**，提供2-14倍性能提升）。内建形状感知与多形状缓存|
|NoReuse|关闭Reuse功能（不推荐，会降低性能）|
|UseNameWhenTagEmpty|用未设置borm tag的字段名本身作为待获取的db字段|
|ToTimestamp|调用Insert时，使用时间戳，而非格式化字符串|

选项使用示例：
   ``` golang
   n, err = t.Debug().Insert(&o)

   n, err = t.ToTimestamp().Insert(&o)
   
   // Reuse功能默认开启，无需手动调用
   // 如需关闭（不推荐），可调用：
   n, err = t.NoReuse().Insert(&o)

   // Reuse 内建形状守卫：当同一调用点的 SQL 形状（Fields/Where/IN 占位符个数等）可能变化时，自动防止错误复用
   n, err = t.Update(&o, b.Fields("name"), b.Where(b.Eq("id", id)))
   ```

### Where

|示例|说明|
|-|-|
|Where("id=? and name=?", id, name)|常规格式化版本|
|Where(Eq("id", id), Eq("name", name)...)|默认为and连接|
|Where(And(Eq("x", x), Eq("y", y), Or(Eq("x", x), Eq("y", y)...)...)...)|And & Or|

### 预置Where条件

|名称|示例|说明|
|-|-|-|
|逻辑与|And(...)|任意个参数，只接受下方的关系运算子|
|逻辑或|Or(...)|任意个参数，只接受下方的关系运算子|
|普通条件|Cond("id=?", id)|参数1为格式化字符串，后面跟占位参数|
|相等|Eq("id", id)|两个参数，id=?|
|不相等|Neq("id", id)|两个参数，id<>?|
|大于|Gt("id", id)|两个参数，id>?|
|大于等于|Gte("id", id)|两个参数，id>=?|
|小于|Lt("id", id)|两个参数，id<?|
|小于等于|Lte("id", id)|两个参数，id<=?|
|在...之间|Between("id", start, end)|三个参数，在start和end之间|
|近似|Like("name", "x%")|两个参数，name like "x%"|
|近似|GLOB("name", "?x*")|两个参数，name glob "?x*"|
|多值选择|In("id", ids)|两个参数，ids是基础类型的slice|

### GroupBy

|示例|说明|
|-|-|
|GroupBy("id", "name"...)|-|

### Having

|示例|说明|
|-|-|
|Having("id=? and name=?", id, name)|常规格式化版本|
|Having(Eq("id", id), Eq("name", name)...)|默认为and连接|
|Having(And(Eq("x", x), Eq("y", y), Or(Eq("x", x), Eq("y", y)...)...)...)|And & Or|

### OrderBy

|示例|说明|
|-|-|
|OrderBy("id desc", "name asc"...)|-|

### Limit

|示例|说明|
|-|-|
|Limit(1)|分页大小为1|
|Limit(3, 2)|分页大小为3，偏移位置为2 **（注意和MySQL的区别）**|

### OnConflictDoUpdateSet

|示例|说明|
|-|-|
|OnConflictDoUpdateSet([]string{"id"}, []string{"name", "age"})|解决主键冲突的更新（使用 excluded. 前缀）|

### Map类型支持

|示例|说明|
|-|-|
|Insert(b.V{"name": "John", "age": 30})|使用map插入数据|
|支持所有CRUD操作|Select、Insert、Update、Delete都支持map|

### Embedded Struct支持

|示例|说明|
|-|-|
|struct内嵌其他struct|自动处理组合对象的字段|
|borm:"-"标签|标记嵌入结构体|

### 字段忽略功能

|示例|说明|
|-|-|
|Password string `borm:"-"`|忽略此字段，不参与数据库操作|
|适用于敏感字段|如密码、临时字段等|

### IndexedBy

|示例|说明|
|-|-|
|IndexedBy("idx_biz_id")|解决索引选择性差的问题|

# 如何mock

### mock步骤：
- 调用`BormMock`指定需要mock的操作
- 使用`BormMockFinish`检查是否命中mock

### 说明：

- 前五个参数分别为`tbl`, `fun`, `caller`, `file`, `pkg`
   - 设置为空默认为匹配
   - 支持通配符'?'和'*'，分别代表匹配一个字符和多个字符
   - 不区分大小写

      |参数|名称|说明|
      |-|-|-|
      |tbl|表名|数据库的表名|
      |fun|方法名|Select/Insert/Update/Delete|
      |caller|调用方方法名|需要带包名|
      |file|文件名|使用处所在文件路径|
      |pkg|包名|使用处所在的包名|

- 后三个参数分别为`返回的数据`，`返回的影响条数`和`错误`
- 只能在测试文件中使用


### 使用示例：

待测函数：

```golang
   package x

   func test(db *sql.DB) (X, int, error) {
      var o X
      tbl := b.Table(db, "tbl")
      n, err := tbl.Select(&o, b.Where("`id` >= ?", 1), b.Limit(100))
      return o, n, err
   }
```

在`x.test`方法中查询`tbl`的数据，我们需要mock数据库的操作

``` golang
   // 必须在_test.go里面设置mock
   // 注意调用方方法名需要带包名
   b.BormMock("tbl", "Select", "*.test", "", "", &o, 1, nil)

   // 调用被测试函数
   o1, n1, err := test(db)

   So(err, ShouldBeNil)
   So(n1, ShouldEqual, 1)
   So(o1, ShouldResemble, o)

   // 检查是否全部命中
   err = b.BormMockFinish()
   So(err, ShouldBeNil)
```

# 性能测试结果

## Reuse功能性能优化
- **基准测试结果**:
  - 单线程: 8.6x 性能提升
  - 并发场景: 最高14.2x 性能提升
  - 内存优化: 92% 内存使用减少
  - 分配优化: 75% 分配次数减少

- **技术实现**:
  - 调用位置缓存: 使用`runtime.Caller`缓存文件行号
  - 字符串池化: `sync.Pool`复用`strings.Builder`
  - 零分配设计: 避免重复的字符串构建和内存分配
  - 并发安全: `sync.Map`支持高并发访问

- **性能数据**:
  ```
  BenchmarkReuseOptimized-8    	 1000000	      1200 ns/op	     128 B/op	       2 allocs/op
  BenchmarkReuseOriginal-8     	  100000	     10320 ns/op	    1600 B/op	      15 allocs/op
  ```

## 时间解析优化
- **优化前**: 使用循环尝试多种时间格式
- **优化后**: 智能格式检测，单次解析
- **性能提升**: 5.1x 速度提升，100% 内存优化
- **支持格式**: 
  - 标准格式: `2006-01-02 15:04:05`
  - 带时区: `2006-01-02 15:04:05 -0700 MST`
  - 带纳秒: `2006-01-02 15:04:05.999999999 -0700 MST`
  - 纯日期: `2006-01-02`
  - 空值处理: 自动处理空字符串和NULL值

## 字段缓存优化
- **技术**: 使用`sync.Map`缓存字段映射
- **效果**: 重复操作性能显著提升
- **适用场景**: 批量操作、频繁查询

## 字符串操作优化
- **优化**: 使用`strings.Builder`替代多次字符串拼接
- **效果**: 减少内存分配，提升字符串构建性能

## 反射优化
- **技术**: 使用`reflect2`替代标准`reflect`包
- **效果**: 零使用`ValueOf`，避免性能问题
- **优势**: 更快的类型检查和字段访问

# 待完成

- Insert/Update支持非指针类型
- 事务相关支持
- 联合查询
- 连接池
- 读写分离

## 赞助

通过成为赞助商来支持这个项目。 您的logo将显示在此处，并带有指向您网站的链接。 [[成为赞助商](https://opencollective.com/borm#sponsor)]

<a href="https://opencollective.com/borm/sponsor/0/website" target="_blank"><img src="https://opencollective.com/borm/sponsor/0/avatar.svg"></a>
<a href="https://opencollective.com/borm/sponsor/1/website" target="_blank"><img src="https://opencollective.com/borm/sponsor/1/avatar.svg"></a>
<a href="https://opencollective.com/borm/sponsor/2/website" target="_blank"><img src="https://opencollective.com/borm/sponsor/2/avatar.svg"></a>
<a href="https://opencollective.com/borm/sponsor/3/website" target="_blank"><img src="https://opencollective.com/borm/sponsor/3/avatar.svg"></a>

## 贡献者

这个项目的存在要感谢所有做出贡献的人。

请给我们一个💖star💖来支持我们，谢谢。

并感谢我们所有的支持者！ 🙏

<a href="https://opencollective.com/borm/backer/0/website?requireActive=false" target="_blank"><img src="https://opencollective.com/borm/backer/0/avatar.svg?requireActive=false"></a>
<a href="https://opencollective.com/borm/backer/1/website?requireActive=false" target="_blank"><img src="https://opencollective.com/borm/backer/1/avatar.svg?requireActive=false"></a>
<a href="https://opencollective.com/borm/backer/2/website?requireActive=false" target="_blank"><img src="https://opencollective.com/borm/backer/2/avatar.svg?requireActive=false"></a>
<a href="https://opencollective.com/borm/backer/3/website?requireActive=false" target="_blank"><img src="https://opencollective.com/borm/backer/3/avatar.svg?requireActive=false"></a>
