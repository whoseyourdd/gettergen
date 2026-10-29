package main

import (
	"go/ast"
	"strconv"
	"testing"
)

func TestExportedFieldName(t *testing.T) {
	tests := map[string]string{
		"id":              "ID",
		"uid":             "UID",
		"companyId":       "CompanyID",
		"uuidValue":       "UUIDValue",
		"httpServerURL":   "HTTPServerURL",
		"purchaseOrderId": "PurchaseOrderID",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := exportedFieldName(input); got != want {
				t.Fatalf("exportedFieldName(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestGetterNameTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		want    string
		include bool
		wantErr bool
	}{
		{name: "override", tag: `getter:"PORevision"`, want: "PORevision", include: true},
		{name: "skip", tag: `getter:"-"`, include: false},
		{name: "default", tag: `json:"id"`, want: "ID", include: true},
		{name: "unexported override", tag: `getter:"revision"`, wantErr: true},
		{name: "invalid override", tag: `getter:"bad-name"`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			literal := &ast.BasicLit{Value: strconv.Quote(test.tag)}
			got, include, err := getterName("id", &ast.Ident{Name: "string"}, literal)
			if (err != nil) != test.wantErr {
				t.Fatalf("getterName() error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && (got != test.want || include != test.include) {
				t.Fatalf("getterName() = (%q, %v), want (%q, %v)", got, include, test.want, test.include)
			}
		})
	}
}

func TestBoolGetterNames(t *testing.T) {
	for field, want := range map[string]string{"deleted": "IsDeleted", "isDeleted": "IsDeleted", "uid": "IsUID"} {
		t.Run(field, func(t *testing.T) {
			got, include, err := getterName(field, &ast.Ident{Name: "bool"}, nil)
			if err != nil || !include || got != want {
				t.Fatalf("getterName() = (%q, %v, %v), want (%q, true, nil)", got, include, err, want)
			}
		})
	}
}

func TestSliceGetterCopiesBackingArray(t *testing.T) {
	got := getterValueExpression("m", "ids", &ast.ArrayType{Elt: &ast.Ident{Name: "ID"}}, "[]ID")
	if got != "append([]ID(nil), m.ids...)" {
		t.Fatalf("getterValueExpression() = %q", got)
	}
}

func TestNilableTypes(t *testing.T) {
	tests := []struct {
		typ  ast.Expr
		want bool
	}{
		{typ: &ast.StarExpr{X: &ast.Ident{Name: "string"}}, want: true},
		{typ: &ast.ArrayType{Elt: &ast.Ident{Name: "string"}}, want: true},
		{typ: &ast.MapType{Key: &ast.Ident{Name: "string"}, Value: &ast.Ident{Name: "int"}}, want: true},
		{typ: &ast.ChanType{Value: &ast.Ident{Name: "int"}}, want: true},
		{typ: &ast.FuncType{}, want: true},
		{typ: &ast.InterfaceType{Methods: &ast.FieldList{}}, want: true},
		{typ: &ast.Ident{Name: "string"}, want: false},
	}
	for _, test := range tests {
		if got := isNilableType(test.typ); got != test.want {
			t.Errorf("isNilableType(%T) = %v, want %v", test.typ, got, test.want)
		}
	}
}

func TestPointerFieldIsSkippedUnlessTagged(t *testing.T) {
	if _, include, err := getterName("approval", &ast.StarExpr{X: &ast.Ident{Name: "Approval"}}, nil); err != nil || include {
		t.Fatalf("untagged pointer field include = %v, err = %v; want false, nil", include, err)
	}
	got, include, err := getterName("approval", &ast.StarExpr{X: &ast.Ident{Name: "Approval"}}, &ast.BasicLit{Value: strconv.Quote(`getter:"Approval"`)})
	if err != nil || !include || got != "Approval" {
		t.Fatalf("tagged pointer getter = (%q, %v, %v)", got, include, err)
	}
}
