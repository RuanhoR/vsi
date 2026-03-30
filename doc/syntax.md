# VSI 语言语法文档

## 概述

VSI 是一种简洁的脚本语言，支持函数、类、模块化、错误处理等特性。

## 基本语法

### 注释

```vsi
// 单行注释

/*
  多行注释
*/
```

### 变量声明

```vsi
// 基本声明
var name = "VSI"
var count = 42
```

### 数据类型

| 类型     | 说明   | 示例            |
| -------- | ------ | --------------- |
| `int`    | 整数   | `42`, `-10`     |
| `string` | 字符串 | `"hello"`       |
| `bool`   | 布尔值 | `true`, `false` |
| `array`  | 数组   | `[1, 2, 3]`     |
| `object` | 对象   | `new Person()`  |
| `nil`    | 空值   | `nil`           |

### 运算符

```vsi
// 算术运算
var sum = 1 + 2      // 加法
var diff = 5 - 3     // 减法
var product = 4 * 2  // 乘法
var quotient = 8 / 2 // 除法

// 比较运算
var eq = a == b      // 等于
var neq = a != b     // 不等于
var lt = a < b       // 小于
var gt = a > b       // 大于
var le = a <= b      // 小于等于
var ge = a >= b      // 大于等于

// 逻辑运算
var and = a && b     // 逻辑与
var or = a || b      // 逻辑或
var not = !a         // 逻辑非
```

## 函数

### 函数声明

```vsi
// 无返回值
fun greet(name) {
  process.stdout.write("Hello, " + name)
}

// 带返回值
fun add(a, b) {
  return a + b
}

// 带类型标注
fun multiply(a: int, b: int): int {
  return a * b
}

// void 返回类型
fun log(msg: string): void {
  process.stdout.write(msg)
}
```

### 函数调用

```vsi
var result = add(10, 20)
greet("World")
```

## 控制流

### 条件语句

```vsi
// if-else
if (x > 0) {
  process.stdout.write("positive")
} else if (x < 0) {
  process.stdout.write("negative")
} else {
  process.stdout.write("zero")
}
```

### 循环语句

```vsi
// for 循环
for (var i = 0; i < 10; i = i + 1) {
  process.stdout.write(i)
}

// while 循环
var j = 0
while (j < 5) {
  process.stdout.write(j)
  j = j + 1
}
```

## 数组

### 数组字面量

```vsi
var arr = [1, 2, 3, 4, 5]
var names = ["Alice", "Bob", "Charlie"]
```

### 数组操作

```vsi
// 访问元素
var first = arr[0]
var last = arr[4]

// 修改元素
arr[0] = 100

// 数组长度
var len = arr.length

// 带类型标注的数组
var nums: int[] = [1, 2, 3]
```

## 类

### 类定义

```vsi
class Person {
  // 属性
  name
  age

  // 构造函数
  public fun constructor(name: string, age: int) {
    this.name = name
    this.age = age
  }

  // 方法
  public fun greet(): void {
    process.stdout.write("Hello, I'm " + this.name)
  }

  // getter
  public fun getName(): string {
    return this.name
  }
}
```

### 实例化

```vsi
var person = new Person("Alice", 30)
person.greet()
var name = person.getName()
```

### 继承

```vsi
class Student extends Person {
  school

  public fun constructor(name: string, age: int, school: string) {
    super(name, age)
    this.school = school
  }
}
```

## 模块系统

### 导入模块

```vsi
// 导入整个模块
import "./utils.vsi" as utils

// 使用导入的函数
utils.log("Hello")
```

### 导出

```vsi
// 定义函数
fun add(a, b) {
  return a + b
}

fun multiply(a, b) {
  return a * b
}

// 导出
export {
  add,
  multiply as mul
}
```

### 完整示例

文件 `math.vsi`:

```vsi
fun add(a: int, b: int): int {
  return a + b
}

export {
  add
}
```

文件 `main.vsi`:

```vsi
import "./math.vsi" as math

var result = math.add(10, 20)
process.stdout.write(result)
```

## 错误处理

### try-catch-finally

```vsi
try {
  // 可能出错的代码
  throw Error.new("Something went wrong")
} catch (err) {
  // 处理错误
  process.stdout.write(err.Message)
} finally {
  // 无论是否出错都会执行
  process.stdout.write("Cleanup")
}
```

### 抛出错误

```vsi
// 创建并抛出错误
var err = Error.new("Error message")
throw err

// 直接抛出
throw Error.new("Immediate error")
```

## 展开运算符

```vsi
fun sum(a, b, c) {
  return a + b + c
}

var arr = [1, 2, 3]
var result = sum(...arr)  // 相当于 sum(1, 2, 3)
```

## 内置对象

### process

```vsi
// 当前工作目录
var cwd = process.cwd

// 标准输出
process.stdout.write("Hello")
process.stdout.write(123)

// 文件操作
var content = process.file.readFile("./data.txt")  // 返回 int[]
process.file.writeFile("./output.txt", content)

// 路径操作
var path = process.path.join("/home", "user", "file.txt")
var dir = process.path.dirname("/home/user/file.txt")
var base = process.path.basename("/home/user/file.txt")
```

### String

```vsi
// 从 Unicode 数组创建字符串
var codes = [72, 101, 108, 108, 111]  // "Hello"
var str = String.new(codes)

// 带长度限制
var str2 = String.new(codes, nil, 3)  // "Hel"

// 从字符编码创建
var str3 = String.fromCharCode(72, 101, 108, 108, 111)  // "Hello"
```

### Error

```vsi
// 创建错误
var err = Error.new("Error message")

// 访问错误信息
var msg = err.Message
```

## 命令行工具

### 运行脚本

```bash
vsi run script.vsi
```

### 编译为字节码

```bash
vsi build script.vsi -o script.vsic
```

### 运行字节码

```bash
vsi vsic script.vsic
```

### 启动 REPL

```bash
vsi repl
```

## VSIC 字节码

VSI 代码可以编译为 VSIC 字节码（.vsic 文件），具有以下特性：

- 二进制格式，体积小
- 快速加载和执行
- 支持优化（死代码消除、函数内联）

### 编译选项

```bash
# 不优化编译
vsi build script.vsi

# 启用优化
vsi build script.vsi --optimize
```

## 类型系统

VSI 支持可选的类型标注，用于编译时检查：

```vsi
// 基本类型
var n: int = 42
var s: string = "hello"
var b: bool = true

// 数组类型
var arr: int[] = [1, 2, 3]

// 函数参数和返回类型
fun add(a: int, b: int): int {
  return a + b
}

// void 返回类型
fun log(msg: string): void {
  process.stdout.write(msg)
}
```

## 完整示例

```vsi
// 导入模块
import "./math.vsi" as math

// 类定义
class Calculator {
  result

  public fun constructor() {
    this.result = 0
  }

  public fun add(n: int): void {
    this.result = this.result + n
  }

  public fun getResult(): int {
    return this.result
  }
}

// 主程序
fun main() {
  var calc = new Calculator()
  calc.add(10)
  calc.add(20)

  process.stdout.write("Result: ")
  process.stdout.write(calc.getResult())
  process.stdout.write("\n")

  // 使用导入的模块
  var sum = math.add(100, 200)
  process.stdout.write("Math sum: ")
  process.stdout.write(sum)
  process.stdout.write("\n")
}

main()
```
