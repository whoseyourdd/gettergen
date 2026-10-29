# gettergen

`gettergen` generates getter methods for private fields in Go structs.

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

## Options

```sh
//go:generate gettergen -pointers=true
```

`-pointers=true` uses pointer receivers for all generated methods. Receiver names default to the lowercase initial of each struct name. To select types or customize receivers:

```sh
//go:generate gettergen -types=Quantity,UnitOfMeasure -receivers=Quantity=q,UnitOfMeasure=u
```

An explicit pointer receiver can be selected with `-receivers=Quantity=*q`. Set `-output=filename.go` to choose a different output file.

Imports used by field types are copied from the source file that declares each struct, including explicit import aliases.
