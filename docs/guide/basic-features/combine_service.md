# Combine Service Usage
## Usage Scenarios
There cloud be decades method defined in service, but only one or two methods is needed by client.
Combine Service provides a way to split one service into several services.
For Example, there is a `ExampleService`
```thrift
service ExampleService {
    ExampleResponse Method0(3: ExampleRequest req)
    ExampleResponse Method1(3: ExampleRequest req)
    ExampleResponse Method2(3: ExampleRequest req)
}
```

We can split it into three services:
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

Client can use one of them to generate code.

## Practice
root thrift:
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

with `--combine-service` parameter, it will generate a new service named CombineService and also client/server code.
It's definition:
```thrift
service CombineService {
    ExampleResponse Method0(3: ExampleRequest req)
    ExampleResponse Method1(3: ExampleRequest req)
    ExampleResponse Method2(3: ExampleRequest req)
}
```

When used with `-service` at the same time, it will use CombineService to generate main package.
Attention: CombineService just combine methods defined in services, it won't generate code when method method name conflicts.

Tips:
You can use extends to combine services defined in several thrift files.
like:
```
service ExampleService0 extends thriftA.Service0 {
}

service ExampleService1 extends thriftB.Service1 {
}

service ExampleService2 extends thriftC.Service2 {
}
```
