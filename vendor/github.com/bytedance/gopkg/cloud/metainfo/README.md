
metainfo
========

This library provides a set of methods to store and manipulate meta information in golang's `context.Context` for inter-service data transmission.

**Meta information** is designed as key-value pairs of strings and keys are case-sensitive.

Meta information is divided into two kinds: **transient** and **persistent** -- the former is passed from a client to a server and disappears from the network while the latter should be passed to the other servers (if the current server plays a client role, too) and go on until some server discards it.

Because we uses golang's `context.Context` to carry meta information, we need to distinguish the *transient* information comes from a client and those set by the current service since they will be stored in the same context object. So we introduce a third kind of meta information **transient-upstream** to denote the transient information received from a client. The **transient-upstream** kind is designed to achieve the semantic of *transient* meta information and is indistinguishable from the transient kind with the APIs from a user perspective.

This library is designed as a set of methods operating on `context.Context`. The way data is transmitted between services should be defined and implemented by frameworks that support this library. Usually, end users should not be concerned about this but rely on the APIs only. 

Framework Support Guide
-----------------------

To introduce **metainfo** into a certain framework, to following conditions should be satisfied:

1. Protocol used by the framework for data transmission should support meta information (HTTP headers, header transport of the apache thrift framework, and so on).
2. When the framework receives meta information as a server role, it needs to add those information to the `context.Context` (by using APIs in this library). Then, before going into the codes of end user (or other codes that requires meta information), the framework should use `metainfo.TransferForward` to convert transient information from the client into transient-upstream information.
3. When the framework establishes a request towards a server as a client role, before sending out the meta information, it should call `metainfo.TransferForward` to discard transient-upstream information in the `context.Context` to make them "alive only one hop".

API Reference
-------------

**Notice**

1. Meta information is designed as key-value pairs as strings.
2. Empty string is invalid when used as a key or value.
3. Due to the characteristics of `context.Context`, any modification on a context object is only visible to the codes that holds the context or its sub contexts.

**Constants**

Package metainfo provides serveral string prefixes to denote the kinds of meta information where context is not available (such as when transmiting data through network).

Typical codes for bussiness logic should never use prefixes. And frameworks that support metainfo may choose other approaches to achieve the same goal.

- `PrefixPersistent`
- `PrefixTransient`
- `PrefixTransientUpstream`

