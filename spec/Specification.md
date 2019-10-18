# Specification

This section attempts to codify the [architecture](./Architecture) with a number of concrete 
implementation details, function signatures, naming choices, etc. 

## Definitions

**Contract** is as some wasm code uploaded to the system, *possibly* with some "global" state, initialized at the creation of the contract.

**Instance** is one instantiation of the contract. This contains a reference to the contract, as well as some "local" state to this instance, initialized at the creation of the instance.

Example: we could upload a generic "ERC20 mintable" contract, and many people could create independent instances of the same bytecode, where the local data defines the token name, the issuer, the max issuance, etc.

* First you **create** a *contract*
* Then you **instantiate** an *instance*
* Finally users **invoke** the *instance*

*Contracts* are immutible (code/logic is fixed), but *instances* are mutible (state changes)

## Serialization

There are two pieces of data that must be considered here. **Message Data**, which is arbitrary binary data passed in the transaction by the end user signing it, and **Context Data**, which is passed in by the cosmos sdk runtime, providing some guaranteed context. Context data may include the signer's address, the instance's address, number of tokens sent, block height, and any other information a contract may need to control the internal logic.

**Message Data** comes from a binary transaction and must be serialized. The most standard and flexible codec is (unfortunately) JSON. This allows the contract to define any schema it wants, and the client can easily provide the proper data. We recommend using a `string` field in the `InvokeMsg`, to contain the user-defined *message data*.

**Contact Data** comes from the go runtime and can either be serialized by sdk and deserialized by the contract, or we can try to do some ffi magic and use the same memory layout for the struct in Go and Wasm and avoid any serialization overhead. Note that the context data struct will be well-defined at compile time and guaranteed not to change between invocations (the same cannot be said for *message data*).

In spite of possible performance gains or compiler guarantees with C-types, I would recommend using JSON for this as well. Or another well-defined binary format, like protobuf. However, I will document some links below for those who would like to research the shared struct approach.

* [repr( c )](https://doc.rust-lang.org/nomicon/other-reprs.html) is a rust directive to produce cannonical C-style memory layouts. This is typically used in FFI (which wasm calls are).
* [wasm-ffi](https://github.com/DeMille/wasm-ffi) demos how to pass structs between wasm/rust and javascript painlessly. Not into golang, but it provides a nice explanation and design overview.
* [wasm-bindgen](https://github.com/rustwasm/wasm-bindgen/) also tries to convert types and you can [read some success and limitations of the approach](https://github.com/rustwasm/wasm-bindgen/issues/111)
* [cgo](https://golang.org/cmd/cgo/#hdr-Go_references_to_C) has some documentation about accessing C structs from Go (which is what we get with the repr( c ) directive)

## State Access

**Contract State** is globally accessible to all instances of  the contract. This is optional and I would consider a read-only singleton as sufficient for most customizations of the contract (using a contract with different configurations), and more secure than other access modes. This should be discussed in the light of actual use cases. Maybe it is unneeded, maybe write access is needed.

We can set contract state upon *creation*. This is a unique db key that cannot be accessed excpet by this contract code. I would conceive of this as a data section baked into the binary, which is actually probably the better approach. 

**Instance State** is accessible only by one instance of the contract, with full read-write access. This can contain either a singleton (one key - simple contract or config) or a kvstore (subspace with many keys that can be accessed - like erc20 instance holding balances for many accounts). Sometimes the contract may want one or the other or even both (config + much data) access modes. 

We can set the instance state upon *instantiation*. We can read and modify it upon *invocation*. This is a unique "prefixed db"  subspace that can only be accessed by this instance. The read-only contract state should suffice for shared data between all instances. (Discuss this design in light of all use cases)

## Function Definitions

As discussed above, all data structures passed between web assembly and the cosmos-sdk will be sent in their JSON representation. For simplicity, I will show them as Go structs in this section, and we can assume a cannonical snake_case conversion of the field name. This allows us to be explicit about types.

Function: `Process(request Request) Response`

This Read-Only info is available to every contract:

```go
type Request struct {
    Block BlockInfo
    Message MessageInfo
    Contract ContractInfo
}

type BlockInfo struct {
    Height int64
    Time int64 // ??? seconds (nanoseconds) since unix epoch???
    ChainID string
}

type MessageInfo struct {
    Signer string // bech32 encoding of authorizing address
    TokensSent CoinInfo  // money transfered by sdk as part of this message (before execution)
    UserData string // arbitrary data set in transaction
}

type ContractInfo struct {
    Address string // bech32 encoding of address this contract controls
    Balance []CoinInfo
}

type CoinInfo struct {
    Amount string // encoing of decimal value, eg. "12.3456"
    Denom string // type, eg. "ATOM"
}
```

This is the information the contract can return:

```go
type Response struct {
    Result ABCIResult
    // Msg struct is an "interface" discussed in a later section
    Messages []Msg
}

type ABCIResult struct {
    Data string // hex-encoded
    Log string
    Code int32 // non-zero on error
    CodeSpace string // non-empty on error
    Events []Event
}

// do we give all this power to wasm, or provide a simpler interface (eg. fixing type)
type Event struct {
    Type string
    Attributes []struct{
        Key string
        Value string
    }
}
```

Note that I intentionally redefine a number of core types, rather than importing them from sdk/types. This is to guarantee immutibility. These types will be passed to and from the contract, and the contract adapter code (in go) can convert them to the go types used in the rest of the app. But these are decoupled, so they can remain constant while other parts of the sdk evolve.

## Exposed imports

**TODO**

Precise imports it can expect to be able to call (eg. query modules, modify local state)


## Define Query and Message types

**TODO**

After upgrade discussion above, some fixed subset of types here