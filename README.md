
# borm

[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat)](https://github.com/orca-zhang/borm/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/orca-zhang/borm)](https://goreportcard.com/report/github.com/orca-zhang/borm)
[![Build Status](https://semaphoreci.com/api/v1/orca-zhang/borm/branches/master/shields_badge.svg)](https://semaphoreci.com/orca-zhang/borm)
[![codecov](https://codecov.io/gh/orca-zhang/borm/branch/master/graph/badge.svg)](https://codecov.io/gh/orca-zhang/borm)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Forca-zhang%2Fborm.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Forca-zhang%2Fborm?ref=badge_shield)

🏎️ 更好的ORM库 (Better ORM library that is simple, fast and self-mock for Go)

# 目标：
- 易用：SQL-Like（一把梭：One-Line-CRUD）
- 通用：支持struct，pb，map和基本类型
- slice用于表达批量，每个元素是row，而不是column
- KISS：保持小而美（不做大而全）
- 可测：支持自mock（因为参数作返回值，大部分mock框架不支持）
    - 非测试向的library不是好library

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
      <td rowspan="8">易用性</td>
      <td>无需指定类型</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>tag中类型定义，用于DDL，低频操作</td>
   </tr>
   <tr>
      <td>无需指定model</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>xorm/gorm需要提供一个“模版”</td>
   </tr>
   <tr>
      <td>无需指定主键</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>容易误操作，比如删/改全表</td>
   </tr>
   <tr>
      <td>学习成本低</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm是SQL-Like，会SQL就会用</td>
   </tr>
   <tr>
      <td>非链式调用</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm是单函数调用</td>
   </tr>
   <tr>
      <td>可复用原生连接</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>重构成本极小</td>
   </tr>
   <tr>
      <td>全类型转换</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>杜绝使用库函数等场景的抛错</td>
   </tr>
   <tr>
      <td>查询命令复用</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>borm批量和单条使用同一个函数</td>
   </tr>
   <tr>
      <td>可测试性</td>
      <td>自mock</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>非常便于单元测试</td>
   </tr>
   <tr>
      <td rowspan="2">性能</td>
      <td>较原生耗时</td>
      <td>1x</td>
      <td>2~3x</td>
      <td>2~3x</td>
      <td>xorm使用prepare模式会比gorm慢</td>
   </tr>
   <tr>
      <td>反射</td>
      <td>reflect2</td>
      <td>reflect</td>
      <td>reflect</td>
      <td>borm零使用ValueOf</td>
   </tr>
</table>

## 背景

- 手写SQL太耗时，花几天写了一个Go版的ORM库，基本参照cpp版borm（暂未开源）进行复刻

- 解决核心痛点：
    1. 手工组装数据太花时间
    2. 手撸SQL难免有语法错误
    3. time.Time无法直接读写的问题
    4. SQL函数结果无法直接Scan
    5. db操作无法方便的Mock
    6. QueryRow的sql.ErrNoRows问题
    7. 直接替换系统自带Scanner，完整接管数据读取的类型转换

- 横向对比：
    1. 其他orm库需要指定数据库字段类型，需要显示指定Model，链式调用；而borm是all-in-one-stmt，单函数调用，参数直接传递你喜欢的“对象/map/对象数组/对象指针数组/任意数据类型”（同时便于mock）
    2. 使用reflect2，零使用ValueOf，并尽量少使用临时对象保证尽可能少的性能损耗和额外内存使用
    3. SQL-Like，无学习成本，不暴露SQL语句，尽最大可能避免语法问题的心智负担
    4. 支持自mock，内建低成本支持mock，无需外部库支持
    5. 目前暂未开始优化，benchmark显示性能和原生database/sql接近，其他orm库2-3倍

- TODO：
    1. 支持复用sql和存储方式，根据代码位置复用（参考json-iterator的binding实现）
    2. Select存储到map
    3. Insert从map读
    4. Insert/Update支持非指针类型
    5. 自动处理where条件优先级（Or的处理）
    6. Benchmark报告
    7. 事务相关支持
