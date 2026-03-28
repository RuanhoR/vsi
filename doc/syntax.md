# vsi test language stntax

File a.vsi

```
import "./b.vsi" as xxx
fun aaa() {
  xxx.print(”running")
}
export {
  aaa as a
}
```

File b.vsi

```
fun print(msg: string) {
  // global:
  os.io.Println(msg)
}
```
