
### BORM

[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat)](https://github.com/orca-zhang/borm/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/orca-zhang/borm)](https://goreportcard.com/report/github.com/orca-zhang/borm)
[![Build Status](https://semaphoreci.com/api/v1/orca-zhang/borm/branches/master/shields_badge.svg)](https://semaphoreci.com/orca-zhang/borm)
[![codecov](https://codecov.io/gh/orca-zhang/borm/branch/master/graph/badge.svg)](https://codecov.io/gh/orca-zhang/borm)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Forca-zhang%2Fborm.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Forca-zhang%2Fborm?ref=badge_shield)

- 🏎️ 小而美的ORM库 (A Beautiful ORM library that is simple, fast and self-mock for Go)

### 背景

- golang写SQL太耗时间了，花了几天写了一个golang版的ORM库，基本参照cpp版borm进行复刻（暂未开源）

- 解决核心痛点：
    1. 手工组装数据太花时间
    2. 手撸SQL难免有语法错误
    3. time.Time无法直接读写的问题
    4. SQL函数结果无法直接Scan
    5. db操作无法方便的Mock
    6. QueryRow的sql.ErrNoRows问题
    7. 直接替换系统自带Scanner，完整接管数据读取的类型转换
    8. 目前暂未开始优化，benchmark显示性能和原生database/sql接近

- 横向对比：
    1. 其他orm库：需要指定数据库字段类型，需要显示指定Model，链式调用
      - borm：all-in-one，单函数调用，参数直接传递你喜欢的“对象/map/对象数组/对象指针数组/任意数据类型”（同时便于mock）
    2. 使用reflect2，零使用ValueOf，并尽量少使用临时对象保证尽可能少的性能损耗和额外内存使用
    3. SQL-Like，无学习成本，不暴露SQL语句，尽最大可能避免语法问题的心智负担
    4. **支持自mock，内建低成本支持mock，无需外部库支持**

- TODO：
    1. 自动规整sql语法（条件可以无序传入）
    2. 支持复用sql和存储方式，根据代码位置复用（参考json-iterator的binding实现）
    3. Select存储到map
    4. Insert从map读
    5. Insert/Update支持非指针类型
    6. 自动处理where条件优先级（Or的处理）
    7. Benchmark报告

- 大致的demo（更多可以看ut用例）：
    ``` golang
    count, err := t.Select(&mtime,
    Where(Gt(`mtime`, m)),
    OrderBy("`mtime` desc"),
    Limit(1)))

    count, err := t.Select(&mtime,
    Fields("mtime"),
    Where(Gt("mtime", m)),
    GroupBy("foo", "bar"),
    OrderBy("`mtime` desc"),
    Limit(0, 100))
    ```
### 目标：
- 易用：SQL-Like（一把梭：One-Line-CRUD）
- 通用：支持struct，pb，map和基本类型
- slice用于表达批量，每个元素是row，而不是column
- KISS：保持小而美（不做大而全）
- 可测：支持自mock（因为参数作返回值，大部分mock框架不支持）
    - 非测试向的library不是好library

### ROADMAP

1. ✔【易用】支持不同类型自动转换，数值&字符串&byte数组&时间

2. ✔【易用】支持slice & map直接In操作
- map需要指定path和field

3. ✔【易用】sql.ErrNoRows处理成count为0

4. ✔【安全】自动防注入

5. 【语法】自动处理表达式优先级
- 默认是逻辑`与`，逻辑`或`需要显式使用Or
``` golang
Where(Gt("mtime", m), Lte("id", 0), Or(Nz("age")))
> "where `mtime` > ? and `id` >= 0 or `age` != 0"

Where(And(c, c, c)) euqals to Where(c, c, c)
> "where c and c and c"

Where(Or(c, c, c))
> "where c or c or c"
```

6. ✔【语法】自动转义字段

7. 【语法】自动规整sql语法
- 条件可以无序传入

8. 【性能】支持复用sql，根据代码位置复用
 ``` golang
 pc, file, line, ok = runtime.Caller(1)  
 log.Println(runtime.FuncForPC(pc).Name())
 ```

9. 【性能】支持复用存储方式
- column和struct&pb中字段index对应关系
- 以及需要类型转换的转换函数

10. ✔【性能】使用reflect2 & strings.Builder保证性能

11. 【测试】支持自mock
- 强制检查是否在*_test.go文件内开启

12. ✔【增强】全局和表级开关配置：
- 是否复用sql
- 是否调试模式，输出日志
- 是否加下划线（去除驼峰开启后生效）
- 是否去除驼峰
- 是否开启mock

13. ✔【增强】关于Fields

|struct|map|pb|基本类型|
|-|-|-|-|
|用tag来表达|用key来表达|用tag来表达|用Fields来表达|

- Fields与声明的类型都存在，以Fields为准
- 数据库和struct字段不一致的情况：
  - 用tag/key来处理alias
  - Fields里面用as或者空格来处理alias，前面是数据库字段，后面是输出字段

14. 【增强】支持嵌套查询或者sql函数
- 利用表名或者字段

15. 【增强】支持事务

16. 【增强】支持存储成map
