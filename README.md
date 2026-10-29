# gettergen

`gettergen` generates getter methods for private fields in Go structs.

For instructions to share with AI coding agents in projects that use gettergen, see [Using gettergen with AI coding agents](docs/ai-agents.md).

## Install

```sh
go install github.com/whoseyourdd/gettergen/cmd/gettergen@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`. Then add a directive to a package source file:

```go
//go:generate gettergen
```

Run generation from the module or package:

```sh
go generate ./...
```

By default, gettergen scans all non-test Go files in the current package, generates getters for private named fields, uses value receivers, and writes `getters_gen.go`. The generated file is excluded from later scans. Field names use Go-style initialisms by default, so `id`, `uid`, and `companyId` become `ID()`, `UID()`, and `CompanyID()`.

Use a `getter` struct tag to override a field's getter name or skip it:

```go
type PurchaseReturn struct {
	id              string `getter:"ID"`
	poRevision      uint64 `getter:"PORevision"`
	lines           []Line `getter:"-"`
}
```

An explicit getter name must be an exported Go identifier. `getter:"-"` omits the field. Fields without a `getter` tag use the default naming rules.

Boolean fields use `Is` getters by default: `deleted bool` generates `IsDeleted()`. If the field already starts with the `is` word, it is not repeated: `isDeleted bool` also generates `IsDeleted()`.

Slice getters return a shallow copy of the slice. Struct values are returned by value. Pointer fields are skipped by default to avoid exposing mutable nested state; add an explicit `getter:"Name"` tag to opt in.

## Options

```sh
//go:generate gettergen -pointers=true
```

`-pointers=true` uses pointer receivers for all generated methods. Receiver names default to the lowercase initial of each struct name. To select types or customize receivers:

```sh
//go:generate gettergen -types=Quantity,UnitOfMeasure -receivers=Quantity=q,UnitOfMeasure=u
```

An explicit pointer receiver can be selected with `-receivers=Quantity=*q`. Set `-output=filename.go` to choose a different output file.

Use `-pointer` to select pointer receivers for specific structs while keeping value receivers for the rest:

```sh
//go:generate gettergen -pointer=StockCount
```

Multiple types can be comma-separated, for example `-pointer=StockCount,PurchaseReturn`. Pointer receiver getters return the field type's zero value when called on a nil receiver. `-pointer` takes precedence over a matching `-receivers` entry; use `-receivers=StockCount=*c` to customize the receiver name.

Imports used by field types are copied from the source file that declares each struct, including explicit import aliases.
