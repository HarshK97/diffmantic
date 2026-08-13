package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
)

func TestGetDeclarationName(t *testing.T) {
	t.Run("function declaration", func(t *testing.T) {
		n := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
		)
		if got := getDeclarationName(n); got != "foo" {
			t.Errorf("getDeclarationName = %q, want %q", got, "foo")
		}
	})

	t.Run("method declaration", func(t *testing.T) {
		n := testutil.Node("method_declaration", "",
			testutil.Leaf("field_identifier", "Bar"),
		)
		if got := getDeclarationName(n); got != "Bar" {
			t.Errorf("getDeclarationName = %q, want %q", got, "Bar")
		}
	})

	t.Run("nil node", func(t *testing.T) {
		if got := getDeclarationName(nil); got != "" {
			t.Errorf("getDeclarationName(nil) = %q, want empty", got)
		}
	})

	t.Run("no identifier child", func(t *testing.T) {
		n := testutil.Node("function_declaration", "",
			testutil.Leaf("block", "{}"),
		)
		if got := getDeclarationName(n); got != "" {
			t.Errorf("getDeclarationName = %q, want empty", got)
		}
	})
}

func TestGetReceiverTypeName(t *testing.T) {
	ptrReceiver := testutil.Node("pointer_type", "",
		testutil.Leaf("type_identifier", "Type"),
	)
	receiver := testutil.Node("parameter_declaration", "",
		ptrReceiver,
	)
	paramList := testutil.Node("parameter_list", "",
		receiver,
	)
	method := testutil.Node("method_declaration", "",
		testutil.Leaf("field_identifier", "Foo"),
		paramList,
	)

	t.Run("method with pointer receiver", func(t *testing.T) {
		if got := getReceiverTypeName(method); got != "Type" {
			t.Errorf("getReceiverTypeName = %q, want %q", got, "Type")
		}
	})

	t.Run("non-method returns empty", func(t *testing.T) {
		fn := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
		)
		if got := getReceiverTypeName(fn); got != "" {
			t.Errorf("getReceiverTypeName for function = %q, want empty", got)
		}
	})

	t.Run("nil node", func(t *testing.T) {
		if got := getReceiverTypeName(nil); got != "" {
			t.Errorf("getReceiverTypeName(nil) = %q, want empty", got)
		}
	})
}

func TestMatchDeclarations(t *testing.T) {
	t.Run("matches functions with same name", func(t *testing.T) {
		src := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
				testutil.Leaf("call", "a()"),
			),
		)
		src.Language = "go"
		dst := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
				testutil.Leaf("call", "b()"),
			),
		)
		dst.Language = "go"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcFn := src.Children[0]
		dstFn := dst.Children[0]
		if !m.Has(srcFn) {
			t.Error("src function_declaration should be mapped by name")
		}
		if m.Src()[srcFn] != dstFn {
			t.Error("functions with same name should map to each other")
		}
	})

	t.Run("skips ambiguous declarations", func(t *testing.T) {
		src := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
				testutil.Leaf("call", "a()"),
			),
		)
		src.Language = "go"
		dst := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
				testutil.Leaf("call", "b()"),
			),
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
				testutil.Leaf("call", "c()"),
			),
		)
		dst.Language = "go"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcFn := src.Children[0]
		if m.Has(srcFn) {
			t.Error("should not map when multiple dst candidates exist")
		}
	})

	t.Run("does not match different names", func(t *testing.T) {
		src := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
			),
		)
		src.Language = "go"
		dst := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "bar"),
			),
		)
		dst.Language = "go"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcFn := src.Children[0]
		if m.Has(srcFn) {
			t.Error("foo and bar should not be matched")
		}
	})

	t.Run("matches class declarations by name", func(t *testing.T) {
		src := testutil.Node("program", "",
			testutil.Node("class_declaration", "",
				testutil.Leaf("identifier", "User"),
				testutil.Leaf("body", "{...}"),
			),
		)
		src.Language = "java"
		dst := testutil.Node("program", "",
			testutil.Node("class_declaration", "",
				testutil.Leaf("identifier", "User"),
				testutil.Leaf("body", "{...different...}"),
			),
		)
		dst.Language = "java"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcCls := src.Children[0]
		if !m.Has(srcCls) {
			t.Error("class declarations with same name should be matched")
		}
	})

	t.Run("matches methods with same receiver type", func(t *testing.T) {
		srcParam := testutil.Node("parameter_list", "",
			testutil.Node("parameter_declaration", "",
				testutil.Node("pointer_type", "",
					testutil.Leaf("type_identifier", "Type"),
				),
			),
		)
		dstParam := testutil.Node("parameter_list", "",
			testutil.Node("parameter_declaration", "",
				testutil.Node("pointer_type", "",
					testutil.Leaf("type_identifier", "Type"),
				),
			),
		)

		src := testutil.Node("program", "",
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				srcParam,
			),
		)
		src.Language = "go"
		dst := testutil.Node("program", "",
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				dstParam,
			),
		)
		dst.Language = "go"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcMeth := src.Children[0]
		if !m.Has(srcMeth) {
			t.Error("methods with same name and receiver should be matched")
		}
	})

	t.Run("does not match methods with different receiver", func(t *testing.T) {
		srcParam := testutil.Node("parameter_list", "",
			testutil.Node("parameter_declaration", "",
				testutil.Leaf("type_identifier", "TypeA"),
			),
		)
		dstParam := testutil.Node("parameter_list", "",
			testutil.Node("parameter_declaration", "",
				testutil.Leaf("type_identifier", "TypeB"),
			),
		)

		src := testutil.Node("program", "",
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				srcParam,
			),
		)
		src.Language = "go"
		dst := testutil.Node("program", "",
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				dstParam,
			),
		)
		dst.Language = "go"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcMeth := src.Children[0]
		if m.Has(srcMeth) {
			t.Error("methods with different receivers should not be matched")
		}
	})
}

func TestMatchDeclarationIntegration(t *testing.T) {
	// Functions with same name but completely different bodies
	// should still match via the declaration pre-pass.
	src := testutil.Node("program", "",
		testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
			testutil.Node("block", "",
				testutil.Leaf("return_statement", "return 1"),
			),
		),
	)
	dst := testutil.Node("program", "",
		testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
			testutil.Node("block", "",
				testutil.Leaf("call", "other()"),
				testutil.Leaf("return_statement", "return 2"),
			),
		),
	)

	r := Match(src, dst, nil, nil)
	srcFn := src.Children[0]
	dstFn := dst.Children[0]
	if !r.Mappings.Has(srcFn) {
		t.Error("Match should map declarations with same name")
	}
	if r.Mappings.Src()[srcFn] != dstFn {
		t.Error("declaration pre-match should map foo -> foo")
	}
}
