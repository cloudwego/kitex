# Combine Service 的使用
## 使用场景
有些服务提供了几十个方法，而对于调用方可能只请求其中 1、2 个方法，为了避免这种巨大的 service 带来的庞大的生成代码，Combine Service 可以让用户将原来一个 Service 的几十个方法拆分成多个 Service。比如原来的 Service 是：
```thrift
service ExampleService {
    ExampleResponse Method0(3: ExampleRequest req)
    ExampleResponse Method1(3: ExampleRequest req)
    ExampleResponse Method2(3: ExampleRequest req)
}
```

用户 IDL 定义可以拆分为三个 Service：
```thrift
service ExampleService0 {
    ExampleResponse Method0(3: ExampleRequest req)
}

service ExampleService1 {
    ExampleResponse Method1(3: ExampleRequest req)
}

service ExampleService2 {
    ExampleResponse Method2(3: ExampleRequest req)
}
```

调用方可以只保留其中一个 Service 生成代码，方法名和参数保持一致不影响 rpc 调用。
## 具体描述
当 root thrift 文件中存在形如下述定义时：
```thrift
service ExampleService0 {
    ExampleResponse Method0(3: ExampleRequest req)
}

service ExampleService1 {
    ExampleResponse Method1(3: ExampleRequest req)
}

service ExampleService2 {
    ExampleResponse Method2(3: ExampleRequest req)
}
```

带上`--combine-service` 参数后，会生成一个名为 CombineService 的新 service 及其对应的 client/server 代码。
其定义为：
```thrift
service CombineService {
    ExampleResponse Method0(3: ExampleRequest req)
    ExampleResponse Method1(3: ExampleRequest req)
    ExampleResponse Method2(3: ExampleRequest req)
}
```

当同时使用了`-service` 参数时，会使用 CombineService 作为 main package 中 server 对应的 service 。
注意： CombineService 只是 method 的聚合，因此当 method 名冲突时将无法生成 CombineService 。

Tips：
配合 extends 关键字，可以实现跨文件的 CombineService
如：
```
service ExampleService0 extends thriftA.Service0 {
}

service ExampleService1 extends thriftB.Service1 {
}

service ExampleService2 extends thriftC.Service2 {
}
```
