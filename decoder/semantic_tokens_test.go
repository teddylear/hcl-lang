// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl-lang/lang"
	"github.com/hashicorp/hcl-lang/reference"
	"github.com/hashicorp/hcl-lang/schema"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/json"
	"github.com/zclconf/go-cty-debug/ctydebug"
	"github.com/zclconf/go-cty/cty"
)

func TestDecoder_SemanticTokensInFile_emptyBody(t *testing.T) {
	f := &hcl.File{
		Body: hcl.EmptyBody(),
	}
	d := testPathDecoder(t, &PathContext{
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
	})

	ctx := context.Background()

	_, err := d.SemanticTokensInFile(ctx, "test.tf")
	unknownFormatErr := &UnknownFileFormatError{}
	if !errors.As(err, &unknownFormatErr) {
		t.Fatal("expected UnknownFileFormatError for empty body")
	}
}

func TestDecoder_SemanticTokensInFile_json(t *testing.T) {
	f, pDiags := json.Parse([]byte(`{
	"customblock": {
		"label1": {}
	}
}`), "test.tf.json")
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Files: map[string]*hcl.File{
			"test.tf.json": f,
		},
	})

	ctx := context.Background()

	_, err := d.SemanticTokensInFile(ctx, "test.tf.json")
	unknownFormatErr := &UnknownFileFormatError{}
	if !errors.As(err, &unknownFormatErr) {
		t.Fatal("expected UnknownFileFormatError for JSON body")
	}
}

func TestDecoder_SemanticTokensInFile_zeroByteContent(t *testing.T) {
	f, pDiags := hclsyntax.ParseConfig([]byte{}, "test.tf", hcl.InitialPos)
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
	})

	ctx := context.Background()

	tokens, err := d.SemanticTokensInFile(ctx, "test.tf")
	if err != nil {
		t.Fatal(err)
	}
	expectedTokens := []lang.SemanticToken{}
	if diff := cmp.Diff(expectedTokens, tokens); diff != "" {
		t.Fatalf("unexpected tokens: %s", diff)
	}
}

func TestDecoder_SemanticTokensInFile_fileNotFound(t *testing.T) {
	f, pDiags := hclsyntax.ParseConfig([]byte{}, "test.tf", hcl.InitialPos)
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
	})

	ctx := context.Background()

	_, err := d.SemanticTokensInFile(ctx, "foobar.tf")
	notFoundErr := &FileNotFoundError{}
	if !errors.As(err, &notFoundErr) {
		t.Fatal("expected FileNotFoundError for non-existent file")
	}
}

func TestDecoder_SemanticTokensInFile_basic(t *testing.T) {
	bodySchema := &schema.BodySchema{
		Blocks: map[string]*schema.BlockSchema{
			"module": {
				Body: &schema.BodySchema{
					Attributes: map[string]*schema.AttributeSchema{
						"count": {
							Constraint: schema.LiteralType{Type: cty.Number},
						},
						"source": {
							Constraint:   schema.LiteralType{Type: cty.String},
							IsDeprecated: true,
							SemanticTokenModifiers: lang.SemanticTokenModifiers{
								lang.TokenModifierDependent,
							},
						},
					},
				},
			},
			"resource": {
				Labels: []*schema.LabelSchema{
					{
						Name:     "type",
						IsDepKey: true,
						SemanticTokenModifiers: lang.SemanticTokenModifiers{
							lang.TokenModifierDependent,
						},
					},
					{Name: "name"},
				},
			},
		},
	}

	testCfg := []byte(`module "ref" {
  source = "./sub"
  count  = 1
}
resource "vault_auth_backend" "blah" {
  default_lease_ttl_seconds = 1
}
`)

	f, pDiags := hclsyntax.ParseConfig(testCfg, "test.tf", hcl.InitialPos)
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Schema: bodySchema,
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
	})

	ctx := context.Background()

	tokens, err := d.SemanticTokensInFile(ctx, "test.tf")
	if err != nil {
		t.Fatal(err)
	}

	expectedTokens := []lang.SemanticToken{
		{ // module
			Type:      lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   1,
					Column: 1,
					Byte:   0,
				},
				End: hcl.Pos{
					Line:   1,
					Column: 7,
					Byte:   6,
				},
			},
		},
		{ // source
			Type: lang.TokenAttrName,
			Modifiers: []lang.SemanticTokenModifier{
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 3,
					Byte:   17,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 9,
					Byte:   23,
				},
			},
		},
		{ // "./sub"
			Type:      lang.TokenString,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 12,
					Byte:   26,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 19,
					Byte:   33,
				},
			},
		},
		{ // count
			Type:      lang.TokenAttrName,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 3,
					Byte:   36,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 8,
					Byte:   41,
				},
			},
		},
		{ // 1
			Type:      lang.TokenNumber,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 12,
					Byte:   45,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 13,
					Byte:   46,
				},
			},
		},
		{ // resource
			Type:      lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 1,
					Byte:   49,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 9,
					Byte:   57,
				},
			},
		},
		{ // vault_auth_backend
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 10,
					Byte:   58,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 30,
					Byte:   78,
				},
			},
		},
		{ // blah
			Type:      lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 31,
					Byte:   79,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 37,
					Byte:   85,
				},
			},
		},
	}

	diff := cmp.Diff(expectedTokens, tokens)
	if diff != "" {
		t.Fatalf("unexpected tokens: %s", diff)
	}
}

func TestDecoder_SemanticTokensInFile_dependentSchema(t *testing.T) {
	bodySchema := &schema.BodySchema{
		Blocks: map[string]*schema.BlockSchema{
			"resource": {
				Labels: []*schema.LabelSchema{
					{
						Name:     "type",
						IsDepKey: true,
						SemanticTokenModifiers: lang.SemanticTokenModifiers{
							lang.TokenModifierDependent,
						},
					},
					{Name: "name"},
				},
				DependentBody: map[schema.SchemaKey]*schema.BodySchema{
					schema.NewSchemaKey(schema.DependencyKeys{
						Labels: []schema.LabelDependent{
							{
								Index: 0,
								Value: "aws_instance",
							},
						},
					}): {
						Attributes: map[string]*schema.AttributeSchema{
							"instance_type": {
								Constraint: schema.LiteralType{Type: cty.String},
							},
							"deprecated": {
								Constraint: schema.LiteralType{Type: cty.Bool},
							},
						},
					},
				},
			},
		},
	}

	testCfg := []byte(`resource "vault_auth_backend" "alpha" {
  default_lease_ttl_seconds = 1
}
resource "aws_instance" "beta" {
  instance_type = "t2.micro"
  deprecated = true
}
`)

	f, pDiags := hclsyntax.ParseConfig(testCfg, "test.tf", hcl.InitialPos)
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Schema: bodySchema,
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
	})

	ctx := context.Background()

	tokens, err := d.SemanticTokensInFile(ctx, "test.tf")
	if err != nil {
		t.Fatal(err)
	}

	expectedTokens := []lang.SemanticToken{
		{ // resource
			Type:      lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   1,
					Column: 1,
					Byte:   0,
				},
				End: hcl.Pos{
					Line:   1,
					Column: 9,
					Byte:   8,
				},
			},
		},
		{ // "vault_auth_backend"
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   1,
					Column: 10,
					Byte:   9,
				},
				End: hcl.Pos{
					Line:   1,
					Column: 30,
					Byte:   29,
				},
			},
		},
		{ // "alpha"
			Type:      lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   1,
					Column: 31,
					Byte:   30,
				},
				End: hcl.Pos{
					Line:   1,
					Column: 38,
					Byte:   37,
				},
			},
		},
		{ // resource
			Type:      lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 1,
					Byte:   74,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 9,
					Byte:   82,
				},
			},
		},
		{ // "aws_instance"
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 10,
					Byte:   83,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 24,
					Byte:   97,
				},
			},
		},
		{ // "beta"
			Type:      lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 25,
					Byte:   98,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 31,
					Byte:   104,
				},
			},
		},
		{ // instance_type
			Type:      lang.TokenAttrName,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 3,
					Byte:   109,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 16,
					Byte:   122,
				},
			},
		},
		{ // "t2.micro"
			Type:      lang.TokenString,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 19,
					Byte:   125,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 29,
					Byte:   135,
				},
			},
		},
		{ // deprecated
			Type:      lang.TokenAttrName,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   6,
					Column: 3,
					Byte:   138,
				},
				End: hcl.Pos{
					Line:   6,
					Column: 13,
					Byte:   148,
				},
			},
		},
		{ // true
			Type:      lang.TokenBool,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   6,
					Column: 16,
					Byte:   151,
				},
				End: hcl.Pos{
					Line:   6,
					Column: 20,
					Byte:   155,
				},
			},
		},
	}

	diff := cmp.Diff(expectedTokens, tokens)
	if diff != "" {
		t.Fatalf("unexpected tokens: %s", diff)
	}
}

func TestDecoder_SemanticTokensInFile_customModifiers(t *testing.T) {
	bodySchema := &schema.BodySchema{
		Blocks: map[string]*schema.BlockSchema{
			"module": {
				SemanticTokenModifiers: lang.SemanticTokenModifiers{"module"},
				Labels: []*schema.LabelSchema{
					{
						Name:                   "name",
						SemanticTokenModifiers: lang.SemanticTokenModifiers{"name"},
					},
				},
				Body: &schema.BodySchema{
					Attributes: map[string]*schema.AttributeSchema{
						"count": {
							Constraint: schema.LiteralType{Type: cty.Number},
						},
						"source": {
							Constraint:             schema.LiteralType{Type: cty.String},
							IsDeprecated:           true,
							SemanticTokenModifiers: lang.SemanticTokenModifiers{lang.TokenModifierDependent},
						},
					},
				},
			},
			"resource": {
				SemanticTokenModifiers: lang.SemanticTokenModifiers{"resource"},
				Labels: []*schema.LabelSchema{
					{
						Name:                   "type",
						IsDepKey:               true,
						SemanticTokenModifiers: lang.SemanticTokenModifiers{"type", lang.TokenModifierDependent},
					},
					{
						Name:                   "name",
						SemanticTokenModifiers: lang.SemanticTokenModifiers{"name"},
					},
				},
				Body: &schema.BodySchema{
					Blocks: map[string]*schema.BlockSchema{
						"provisioner": {
							SemanticTokenModifiers: lang.SemanticTokenModifiers{"provisioner"},
							Labels: []*schema.LabelSchema{
								{
									Name:                   "type",
									SemanticTokenModifiers: lang.SemanticTokenModifiers{"type"},
								},
							},
						},
					},
				},
			},
		},
	}

	testCfg := []byte(`module "ref" {
  source = "./sub"
  count  = 1
}
resource "vault_auth_backend" "blah" {
  provisioner "inner" {
  	test = 42
  }
}
`)

	f, pDiags := hclsyntax.ParseConfig(testCfg, "test.tf", hcl.InitialPos)
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Schema: bodySchema,
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
	})

	ctx := context.Background()

	tokens, err := d.SemanticTokensInFile(ctx, "test.tf")
	if err != nil {
		t.Fatal(err)
	}

	expectedTokens := []lang.SemanticToken{
		{ // module
			Type: lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{
				lang.SemanticTokenModifier("module"),
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   1,
					Column: 1,
					Byte:   0,
				},
				End: hcl.Pos{
					Line:   1,
					Column: 7,
					Byte:   6,
				},
			},
		},
		{ // "ref"
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.SemanticTokenModifier("module"),
				lang.SemanticTokenModifier("name"),
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   1,
					Column: 8,
					Byte:   7,
				},
				End: hcl.Pos{
					Line:   1,
					Column: 13,
					Byte:   12,
				},
			},
		},
		{ // source
			Type: lang.TokenAttrName,
			Modifiers: []lang.SemanticTokenModifier{
				lang.SemanticTokenModifier("module"),
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 3,
					Byte:   17,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 9,
					Byte:   23,
				},
			},
		},
		{ // "./sub"
			Type:      lang.TokenString,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 12,
					Byte:   26,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 19,
					Byte:   33,
				},
			},
		},
		{ // count
			Type: lang.TokenAttrName,
			Modifiers: []lang.SemanticTokenModifier{
				lang.SemanticTokenModifier("module"),
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 3,
					Byte:   36,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 8,
					Byte:   41,
				},
			},
		},
		{ // 1
			Type:      lang.TokenNumber,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 12,
					Byte:   45,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 13,
					Byte:   46,
				},
			},
		},
		{ // resource
			Type: lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{
				lang.SemanticTokenModifier("resource"),
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 1,
					Byte:   49,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 9,
					Byte:   57,
				},
			},
		},
		{ // vault_auth_backend
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.SemanticTokenModifier("resource"),
				lang.SemanticTokenModifier("type"),
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 10,
					Byte:   58,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 30,
					Byte:   78,
				},
			},
		},
		{ // blah
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.SemanticTokenModifier("resource"),
				lang.SemanticTokenModifier("name"),
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 31,
					Byte:   79,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 37,
					Byte:   85,
				},
			},
		},
		{ // provisioner
			Type: lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{
				lang.SemanticTokenModifier("resource"),
				lang.SemanticTokenModifier("provisioner"),
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 6, Column: 3, Byte: 90},
				End:      hcl.Pos{Line: 6, Column: 14, Byte: 101},
			},
		},
		{ // "inner"
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.SemanticTokenModifier("resource"),
				lang.SemanticTokenModifier("provisioner"),
				lang.SemanticTokenModifier("type"),
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 6, Column: 15, Byte: 102},
				End:      hcl.Pos{Line: 6, Column: 22, Byte: 109},
			},
		},
	}

	diff := cmp.Diff(expectedTokens, tokens)
	if diff != "" {
		t.Fatalf("unexpected tokens: %s", diff)
	}
}

func TestDecoder_SemanticTokensInFile_extensions_basic(t *testing.T) {
	bodySchema := &schema.BodySchema{
		Blocks: map[string]*schema.BlockSchema{
			"resource": {
				Body: &schema.BodySchema{
					Extensions: &schema.BodyExtensions{
						Count: true,
					},
					Attributes: map[string]*schema.AttributeSchema{
						"cpu_core_count": {
							Constraint: schema.OneOf{
								schema.Reference{OfType: cty.Number},
								schema.LiteralType{Type: cty.Number},
							},
							IsOptional: true,
						},
					},
				},
				Labels: []*schema.LabelSchema{
					{
						Name:     "type",
						IsDepKey: true,
						SemanticTokenModifiers: lang.SemanticTokenModifiers{
							lang.TokenModifierDependent,
						},
					},
					{Name: "name"},
				},
			},
		},
	}

	testCfg := []byte(`
resource "aws_instance" "app_server" {
  count          = 1
  cpu_core_count = count.index
}
`)

	refTargets := reference.Targets{
		{
			LocalAddr: lang.Address{
				lang.RootStep{Name: "count"},
				lang.AttrStep{Name: "index"},
			},
			Type:        cty.Number,
			Description: lang.PlainText("The distinct index number (starting with 0) corresponding to the instance"),
		},
	}
	refOrigins := reference.Origins{
		reference.LocalOrigin{
			Addr: lang.Address{
				lang.RootStep{Name: "count"},
				lang.AttrStep{Name: "index"},
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 20,
					Byte:   80,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 31,
					Byte:   91,
				},
			},
		},
	}

	f, pDiags := hclsyntax.ParseConfig(testCfg, "test.tf", hcl.InitialPos)
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Schema: bodySchema,
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
		ReferenceTargets: refTargets,
		ReferenceOrigins: refOrigins,
	})

	ctx := context.Background()

	tokens, err := d.SemanticTokensInFile(ctx, "test.tf")
	if err != nil {
		t.Fatal(err)
	}

	expectedTokens := []lang.SemanticToken{
		{ // resource
			Type:      lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 1,
					Byte:   1,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 9,
					Byte:   9,
				},
			},
		},
		{ // aws_instance
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 10,
					Byte:   10,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 24,
					Byte:   24,
				},
			},
		},
		{ // app_server
			Type:      lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 25,
					Byte:   25,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 37,
					Byte:   37,
				},
			},
		},
		{ // count
			Type:      lang.TokenAttrName,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 3,
					Byte:   42,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 8,
					Byte:   47,
				},
			},
		},
		{ // 1
			Type:      lang.TokenNumber,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 20,
					Byte:   59,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 21,
					Byte:   60,
				},
			},
		},
		{ // cpu_core_count
			Type:      lang.TokenAttrName,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 3,
					Byte:   63,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 17,
					Byte:   77,
				},
			},
		},
		{ // count
			Type:      lang.TokenReferenceStep,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 20,
					Byte:   80,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 25,
					Byte:   85,
				},
			},
		},
		{ // index
			Type:      lang.TokenReferenceStep,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 26,
					Byte:   86,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 31,
					Byte:   91,
				},
			},
		},
	}

	diff := cmp.Diff(expectedTokens, tokens)
	if diff != "" {
		t.Fatalf("unexpected tokens: %s", diff)
	}
}

func TestDecoder_SemanticTokensInFile_expression_extensions_depSchema(t *testing.T) {
	bodySchema := &schema.BodySchema{
		Blocks: map[string]*schema.BlockSchema{
			"resource": {
				Body: &schema.BodySchema{
					Extensions: &schema.BodyExtensions{
						Count: true,
					},
				},
				DependentBody: map[schema.SchemaKey]*schema.BodySchema{
					schema.NewSchemaKey(schema.DependencyKeys{
						Labels: []schema.LabelDependent{
							{
								Index: 0,
								Value: "aws_instance",
							},
						},
					}): {
						Attributes: map[string]*schema.AttributeSchema{
							"cpu_core_count": {
								Constraint: schema.OneOf{
									schema.Reference{OfType: cty.Number},
									schema.LiteralType{Type: cty.Number},
								},
								IsOptional: true,
							},
						},
					},
				},
				Labels: []*schema.LabelSchema{
					{
						Name:     "type",
						IsDepKey: true,
						SemanticTokenModifiers: lang.SemanticTokenModifiers{
							lang.TokenModifierDependent,
						},
					},
					{Name: "name"},
				},
			},
		},
	}

	testCfg := []byte(`
resource "aws_instance" "app_server" {
  count          = 1
  cpu_core_count = count.index
}
`)

	refTargets := reference.Targets{
		{
			LocalAddr: lang.Address{
				lang.RootStep{Name: "count"},
				lang.AttrStep{Name: "index"},
			},
			Type:        cty.Number,
			Description: lang.PlainText("The distinct index number (starting with 0) corresponding to the instance"),
		},
	}
	refOrigins := reference.Origins{
		reference.LocalOrigin{
			Addr: lang.Address{
				lang.RootStep{Name: "count"},
				lang.AttrStep{Name: "index"},
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 20,
					Byte:   80,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 31,
					Byte:   91,
				},
			},
		},
	}

	f, pDiags := hclsyntax.ParseConfig(testCfg, "test.tf", hcl.InitialPos)
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Schema: bodySchema,
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
		ReferenceTargets: refTargets,
		ReferenceOrigins: refOrigins,
	})

	ctx := context.Background()

	tokens, err := d.SemanticTokensInFile(ctx, "test.tf")
	if err != nil {
		t.Fatal(err)
	}

	expectedTokens := []lang.SemanticToken{
		{ // resource
			Type:      lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 1,
					Byte:   1,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 9,
					Byte:   9,
				},
			},
		},
		{ // aws_instance
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 10,
					Byte:   10,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 24,
					Byte:   24,
				},
			},
		},
		{ // app_server
			Type:      lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 25,
					Byte:   25,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 37,
					Byte:   37,
				},
			},
		},
		{ // count
			Type:      lang.TokenAttrName,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 3,
					Byte:   42,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 8,
					Byte:   47,
				},
			},
		},
		{ // 1
			Type:      lang.TokenNumber,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 20,
					Byte:   59,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 21,
					Byte:   60,
				},
			},
		},
		{ // cpu_core_count
			Type:      lang.TokenAttrName,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 3,
					Byte:   63,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 17,
					Byte:   77,
				},
			},
		},
		{ // count
			Type:      lang.TokenReferenceStep,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 20,
					Byte:   80,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 25,
					Byte:   85,
				},
			},
		},
		{ // index
			Type:      lang.TokenReferenceStep,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 26,
					Byte:   86,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 31,
					Byte:   91,
				},
			},
		},
	}

	diff := cmp.Diff(expectedTokens, tokens)
	if diff != "" {
		t.Fatalf("unexpected tokens: %s", diff)
	}
}

func TestDecoder_SemanticTokensInFile_extensions_countUndeclared(t *testing.T) {
	bodySchema := &schema.BodySchema{
		Blocks: map[string]*schema.BlockSchema{
			"resource": {
				Body: &schema.BodySchema{
					Extensions: &schema.BodyExtensions{
						Count: true,
					},
					Attributes: map[string]*schema.AttributeSchema{
						"cpu_count": {
							Constraint: schema.LiteralType{Type: cty.Number},
						},
					},
				},
				Labels: []*schema.LabelSchema{
					{
						Name:     "type",
						IsDepKey: true,
						SemanticTokenModifiers: lang.SemanticTokenModifiers{
							lang.TokenModifierDependent,
						},
					},
					{Name: "name"},
				},
			},
		},
	}

	testCfg := []byte(`
resource "vault_auth_backend" "blah" {
  cpu_count = count.index
}
`)

	f, pDiags := hclsyntax.ParseConfig(testCfg, "test.tf", hcl.InitialPos)
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Schema: bodySchema,
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
	})

	ctx := context.Background()

	tokens, err := d.SemanticTokensInFile(ctx, "test.tf")
	if err != nil {
		t.Fatal(err)
	}

	expectedTokens := []lang.SemanticToken{
		{ // resource
			Type:      lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 1,
					Byte:   1,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 9,
					Byte:   9,
				},
			},
		},
		{ // vault_auth_backend
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 10,
					Byte:   10,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 30,
					Byte:   30,
				},
			},
		},
		{ // blah
			Type:      lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 31,
					Byte:   31,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 37,
					Byte:   37,
				},
			},
		},
		{ // cpu_count
			Type:      lang.TokenAttrName,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 3,
					Byte:   42,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 12,
					Byte:   51,
				},
			},
		},
	}

	diff := cmp.Diff(expectedTokens, tokens)
	if diff != "" {
		t.Fatalf("unexpected tokens: %s", diff)
	}
}

func TestDecoder_SemanticTokensInFile_extensions_countIndexInSubBlock(t *testing.T) {
	bodySchema := &schema.BodySchema{
		Blocks: map[string]*schema.BlockSchema{
			"resource": {
				Body: &schema.BodySchema{
					Extensions: &schema.BodyExtensions{
						Count: true,
					},
					Attributes: map[string]*schema.AttributeSchema{
						"count": {
							Constraint: schema.LiteralType{Type: cty.Number},
						},
					},
					Blocks: map[string]*schema.BlockSchema{
						"block": {
							Body: &schema.BodySchema{
								Attributes: map[string]*schema.AttributeSchema{
									"attr": {
										Constraint: schema.Reference{OfType: cty.Number},
									},
								},
							},
						},
					},
				},
				Labels: []*schema.LabelSchema{
					{
						Name:     "type",
						IsDepKey: true,
						SemanticTokenModifiers: lang.SemanticTokenModifiers{
							lang.TokenModifierDependent,
						},
					},
					{Name: "name"},
				},
			},
		},
	}

	testCfg := []byte(`
resource "foobar" "name" {
	count = 1
	block {
		attr = count.index
	}
}
`)

	refTargets := reference.Targets{
		{
			LocalAddr: lang.Address{
				lang.RootStep{Name: "count"},
				lang.AttrStep{Name: "index"},
			},
			Type:        cty.Number,
			Description: lang.PlainText("The distinct index number (starting with 0) corresponding to the instance"),
		},
	}
	refOrigins := reference.Origins{
		reference.LocalOrigin{
			Addr: lang.Address{
				lang.RootStep{Name: "count"},
				lang.AttrStep{Name: "index"},
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 10,
					Byte:   57,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 21,
					Byte:   68,
				},
			},
		},
	}

	f, pDiags := hclsyntax.ParseConfig(testCfg, "test.tf", hcl.InitialPos)
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Schema: bodySchema,
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
		ReferenceTargets: refTargets,
		ReferenceOrigins: refOrigins,
	})

	ctx := context.Background()

	tokens, err := d.SemanticTokensInFile(ctx, "test.tf")
	if err != nil {
		t.Fatal(err)
	}

	expectedTokens := []lang.SemanticToken{
		{ // resource
			Type:      lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 1,
					Byte:   1,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 9,
					Byte:   9,
				},
			},
		},
		{ // foobar
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 10,
					Byte:   10,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 18,
					Byte:   18,
				},
			},
		},
		{ // name
			Type:      lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   2,
					Column: 19,
					Byte:   19,
				},
				End: hcl.Pos{
					Line:   2,
					Column: 25,
					Byte:   25,
				},
			},
		},
		{ // count
			Type:      lang.TokenAttrName,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 2,
					Byte:   29,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 7,
					Byte:   34,
				},
			},
		},
		{ // 1 number
			Type:      lang.TokenNumber,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   3,
					Column: 10,
					Byte:   37,
				},
				End: hcl.Pos{
					Line:   3,
					Column: 11,
					Byte:   38,
				},
			},
		},
		{ // block
			Type:      lang.TokenBlockType,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   4,
					Column: 2,
					Byte:   40,
				},
				End: hcl.Pos{
					Line:   4,
					Column: 7,
					Byte:   45,
				},
			},
		},
		{ // attr
			Type:      lang.TokenAttrName,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 3,
					Byte:   50,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 7,
					Byte:   54,
				},
			},
		},
		{ // count
			Type:      lang.TokenReferenceStep,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 10,
					Byte:   57,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 15,
					Byte:   62,
				},
			},
		},
		{ // index
			Type:      lang.TokenReferenceStep,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start: hcl.Pos{
					Line:   5,
					Column: 16,
					Byte:   63,
				},
				End: hcl.Pos{
					Line:   5,
					Column: 21,
					Byte:   68,
				},
			},
		},
	}

	diff := cmp.Diff(expectedTokens, tokens)
	if diff != "" {
		t.Fatalf("unexpected tokens: %s", diff)
	}
}

func TestDecoder_SemanticTokensInFile_extensions_for_each(t *testing.T) {
	bodySchema := &schema.BodySchema{
		Blocks: map[string]*schema.BlockSchema{
			"resource": {
				Body: &schema.BodySchema{
					Extensions: &schema.BodyExtensions{
						ForEach: true,
					},
					Attributes: map[string]*schema.AttributeSchema{
						"thing": {
							Constraint: schema.Reference{OfType: cty.String},
						},
						"thing_other": {
							Constraint: schema.Reference{OfType: cty.DynamicPseudoType},
						},
					},
				},
				Labels: []*schema.LabelSchema{
					{
						Name:     "type",
						IsDepKey: true,
						SemanticTokenModifiers: lang.SemanticTokenModifiers{
							lang.TokenModifierDependent,
						},
					},
					{Name: "name"},
				},
			},
		},
	}

	testCfg := []byte(`
resource "foobar" "name" {
	for_each = {
		a_group = "eastus"
	}
	thing = each.key
	thing_other = each.value
}
`)

	f, pDiags := hclsyntax.ParseConfig(testCfg, "test.tf", hcl.InitialPos)
	if len(pDiags) > 0 {
		t.Fatal(pDiags)
	}

	d := testPathDecoder(t, &PathContext{
		Schema: bodySchema,
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
	})

	targets, err := d.CollectReferenceTargets()
	if err != nil {
		t.Fatal(err)
	}
	origins, err := d.CollectReferenceOrigins()
	if err != nil {
		t.Fatal(err)
	}

	d = testPathDecoder(t, &PathContext{
		Schema: bodySchema,
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
		ReferenceTargets: targets,
		ReferenceOrigins: origins,
	})

	ctx := context.Background()

	tokens, err := d.SemanticTokensInFile(ctx, "test.tf")
	if err != nil {
		t.Fatal(err)
	}

	expectedTokens := []lang.SemanticToken{
		{ // resource
			Type:      lang.TokenBlockType,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 2, Column: 1, Byte: 1},
				End:      hcl.Pos{Line: 2, Column: 9, Byte: 9},
			},
		},
		{ // foobar
			Type: lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{
				lang.TokenModifierDependent,
			},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 2, Column: 10, Byte: 10},
				End:      hcl.Pos{Line: 2, Column: 18, Byte: 18},
			},
		},
		{ // name
			Type:      lang.TokenBlockLabel,
			Modifiers: []lang.SemanticTokenModifier{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 2, Column: 19, Byte: 19},
				End:      hcl.Pos{Line: 2, Column: 25, Byte: 25},
			},
		},
		{ // for_each
			Type:      lang.TokenAttrName,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 3, Column: 2, Byte: 29},
				End:      hcl.Pos{Line: 3, Column: 10, Byte: 37},
			},
		},
		// a_group
		{
			Type:      lang.TokenMapKey,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 4, Column: 3, Byte: 44},
				End:      hcl.Pos{Line: 4, Column: 10, Byte: 51},
			},
		},
		// "eastus"
		{
			Type:      lang.TokenString,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 4, Column: 13, Byte: 54},
				End:      hcl.Pos{Line: 4, Column: 21, Byte: 62},
			},
		},
		{ // thing
			Type:      lang.TokenAttrName,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 6, Column: 2, Byte: 67},
				End:      hcl.Pos{Line: 6, Column: 7, Byte: 72},
			},
		},
		{ // each
			Type:      lang.TokenReferenceStep,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 6, Column: 10, Byte: 75},
				End:      hcl.Pos{Line: 6, Column: 14, Byte: 79},
			},
		},
		{ // key
			Type:      lang.TokenReferenceStep,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 6, Column: 15, Byte: 80},
				End:      hcl.Pos{Line: 6, Column: 18, Byte: 83},
			},
		},
		{ // thing_other
			Type:      lang.TokenAttrName,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 7, Column: 2, Byte: 85},
				End:      hcl.Pos{Line: 7, Column: 13, Byte: 96},
			},
		},
		{ // each
			Type:      lang.TokenReferenceStep,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 7, Column: 16, Byte: 99},
				End:      hcl.Pos{Line: 7, Column: 20, Byte: 103},
			},
		},
		{ // value
			Type:      lang.TokenReferenceStep,
			Modifiers: lang.SemanticTokenModifiers{},
			Range: hcl.Range{
				Filename: "test.tf",
				Start:    hcl.Pos{Line: 7, Column: 21, Byte: 104},
				End:      hcl.Pos{Line: 7, Column: 26, Byte: 109},
			},
		},
	}

	diff := cmp.Diff(expectedTokens, tokens)
	if diff != "" {
		t.Fatalf("unexpected tokens: %s", diff)
	}
}

func TestDecoder_SemanticTokensInFile_extensions_dynamic(t *testing.T) {
	testCases := []struct {
		name           string
		bodySchema     *schema.BodySchema
		config         string
		expectedTokens []lang.SemanticToken
	}{
		{
			"basic dynamic block",
			&schema.BodySchema{
				Blocks: map[string]*schema.BlockSchema{
					"myblock": {
						Labels: []*schema.LabelSchema{
							{
								Name:                   "type",
								IsDepKey:               true,
								Completable:            true,
								SemanticTokenModifiers: lang.SemanticTokenModifiers{lang.TokenModifierDependent},
							},
							{Name: "name"},
						},
						Body: &schema.BodySchema{
							Extensions: &schema.BodyExtensions{
								DynamicBlocks: true,
							},
							Blocks: make(map[string]*schema.BlockSchema, 0),
						},
						DependentBody: map[schema.SchemaKey]*schema.BodySchema{
							schema.NewSchemaKey(schema.DependencyKeys{
								Labels: []schema.LabelDependent{
									{Index: 0, Value: "foo"},
								},
							}): {
								Blocks: map[string]*schema.BlockSchema{
									"setting": {
										Body: schema.NewBodySchema(),
									},
								},
							},
						},
					},
				},
			},
			`myblock "foo" "bar" {
  dynamic "setting" {
    content {}
  }
}
`,
			[]lang.SemanticToken{
				{ // myblock
					Type:      lang.TokenBlockType,
					Modifiers: []lang.SemanticTokenModifier{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
						End:      hcl.Pos{Line: 1, Column: 8, Byte: 7},
					},
				},
				{ // foo
					Type: lang.TokenBlockLabel,
					Modifiers: []lang.SemanticTokenModifier{
						lang.TokenModifierDependent,
					},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 1, Column: 9, Byte: 8},
						End:      hcl.Pos{Line: 1, Column: 14, Byte: 13},
					},
				},
				{ // bar
					Type:      lang.TokenBlockLabel,
					Modifiers: []lang.SemanticTokenModifier{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 1, Column: 15, Byte: 14},
						End:      hcl.Pos{Line: 1, Column: 20, Byte: 19},
					},
				},
				{ // dynamic
					Type:      lang.TokenBlockType,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 2, Column: 3, Byte: 24},
						End:      hcl.Pos{Line: 2, Column: 10, Byte: 31},
					},
				},
				{ // "setting"
					Type:      lang.TokenBlockLabel,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 2, Column: 11, Byte: 32},
						End:      hcl.Pos{Line: 2, Column: 20, Byte: 41},
					},
				},
				{ // content
					Type:      lang.TokenBlockType,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 3, Column: 5, Byte: 48},
						End:      hcl.Pos{Line: 3, Column: 12, Byte: 55},
					},
				},
			},
		},
		{
			"nested dynamic blocks",
			&schema.BodySchema{
				Blocks: map[string]*schema.BlockSchema{
					"myblock": {
						Labels: []*schema.LabelSchema{
							{
								Name:                   "type",
								IsDepKey:               true,
								Completable:            true,
								SemanticTokenModifiers: lang.SemanticTokenModifiers{lang.TokenModifierDependent},
							},
							{Name: "name"},
						},
						Body: &schema.BodySchema{
							Extensions: &schema.BodyExtensions{
								DynamicBlocks: true,
							},
							Blocks: make(map[string]*schema.BlockSchema, 0),
						},
						DependentBody: map[schema.SchemaKey]*schema.BodySchema{
							schema.NewSchemaKey(schema.DependencyKeys{
								Labels: []schema.LabelDependent{
									{Index: 0, Value: "foo"},
								},
							}): {
								Blocks: map[string]*schema.BlockSchema{
									"setting": {
										Body: &schema.BodySchema{
											Blocks: map[string]*schema.BlockSchema{
												"bar": {
													Body: schema.NewBodySchema(),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			`myblock "foo" "bar" {
  dynamic "setting" {
    content {
      dynamic "bar" {
        content {
        }
      }
    }
  }
}`,
			[]lang.SemanticToken{
				{ // myblock
					Type:      lang.TokenBlockType,
					Modifiers: []lang.SemanticTokenModifier{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
						End:      hcl.Pos{Line: 1, Column: 8, Byte: 7},
					},
				},
				{ // foo
					Type: lang.TokenBlockLabel,
					Modifiers: []lang.SemanticTokenModifier{
						lang.TokenModifierDependent,
					},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 1, Column: 9, Byte: 8},
						End:      hcl.Pos{Line: 1, Column: 14, Byte: 13},
					},
				},
				{ // bar
					Type:      lang.TokenBlockLabel,
					Modifiers: []lang.SemanticTokenModifier{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 1, Column: 15, Byte: 14},
						End:      hcl.Pos{Line: 1, Column: 20, Byte: 19},
					},
				},
				{ // dynamic
					Type:      lang.TokenBlockType,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 2, Column: 3, Byte: 24},
						End:      hcl.Pos{Line: 2, Column: 10, Byte: 31},
					},
				},
				{ // "setting"
					Type:      lang.TokenBlockLabel,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 2, Column: 11, Byte: 32},
						End:      hcl.Pos{Line: 2, Column: 20, Byte: 41},
					},
				},
				{ // content
					Type:      lang.TokenBlockType,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 3, Column: 5, Byte: 48},
						End:      hcl.Pos{Line: 3, Column: 12, Byte: 55},
					},
				},
				{ // dynamic
					Type:      lang.TokenBlockType,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 4, Column: 7, Byte: 64},
						End:      hcl.Pos{Line: 4, Column: 14, Byte: 71},
					},
				},
				{ // "setting"
					Type:      lang.TokenBlockLabel,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 4, Column: 15, Byte: 72},
						End:      hcl.Pos{Line: 4, Column: 20, Byte: 77},
					},
				},
				{ // content
					Type:      lang.TokenBlockType,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 5, Column: 9, Byte: 88},
						End:      hcl.Pos{Line: 5, Column: 16, Byte: 95},
					},
				},
			},
		},
		{
			"deeper nested dynamic blocks",
			&schema.BodySchema{
				Blocks: map[string]*schema.BlockSchema{
					"myblock": {
						Labels: []*schema.LabelSchema{
							{
								Name:                   "type",
								IsDepKey:               true,
								Completable:            true,
								SemanticTokenModifiers: lang.SemanticTokenModifiers{lang.TokenModifierDependent},
							},
							{Name: "name"},
						},
						Body: &schema.BodySchema{
							Extensions: &schema.BodyExtensions{
								DynamicBlocks: true,
							},
							Blocks: make(map[string]*schema.BlockSchema, 0),
						},
						DependentBody: map[schema.SchemaKey]*schema.BodySchema{
							schema.NewSchemaKey(schema.DependencyKeys{
								Labels: []schema.LabelDependent{
									{Index: 0, Value: "foo"},
								},
							}): {
								Blocks: map[string]*schema.BlockSchema{
									"setting": {
										Body: &schema.BodySchema{
											Blocks: map[string]*schema.BlockSchema{
												"foo": {
													Body: &schema.BodySchema{
														Blocks: map[string]*schema.BlockSchema{
															"bar": {
																Body: schema.NewBodySchema(),
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			`myblock "foo" "bar" {
  setting {
    foo {
      dynamic "bar" {

      }
    }
  }
}`,
			[]lang.SemanticToken{
				{ // myblock
					Type:      lang.TokenBlockType,
					Modifiers: []lang.SemanticTokenModifier{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
						End:      hcl.Pos{Line: 1, Column: 8, Byte: 7},
					},
				},
				{ // "foo"
					Type: lang.TokenBlockLabel,
					Modifiers: []lang.SemanticTokenModifier{
						lang.TokenModifierDependent,
					},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 1, Column: 9, Byte: 8},
						End:      hcl.Pos{Line: 1, Column: 14, Byte: 13},
					},
				},
				{ // "bar"
					Type:      lang.TokenBlockLabel,
					Modifiers: []lang.SemanticTokenModifier{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 1, Column: 15, Byte: 14},
						End:      hcl.Pos{Line: 1, Column: 20, Byte: 19},
					},
				},
				{ // setting
					Type:      lang.TokenBlockType,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 2, Column: 3, Byte: 24},
						End:      hcl.Pos{Line: 2, Column: 10, Byte: 31},
					},
				},
				{ // foo
					Type:      lang.TokenBlockType,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 3, Column: 5, Byte: 38},
						End:      hcl.Pos{Line: 3, Column: 8, Byte: 41},
					},
				},
				{ // dynamic
					Type:      lang.TokenBlockType,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 4, Column: 7, Byte: 50},
						End:      hcl.Pos{Line: 4, Column: 14, Byte: 57},
					},
				},
				{ // "bar"
					Type:      lang.TokenBlockLabel,
					Modifiers: lang.SemanticTokenModifiers{},
					Range: hcl.Range{
						Filename: "test.tf",
						Start:    hcl.Pos{Line: 4, Column: 15, Byte: 58},
						End:      hcl.Pos{Line: 4, Column: 20, Byte: 63},
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.name), func(t *testing.T) {
			ctx := context.Background()

			f, pDiags := hclsyntax.ParseConfig([]byte(tc.config), "test.tf", hcl.InitialPos)
			if len(pDiags) > 0 {
				t.Fatal(pDiags)
			}

			d := testPathDecoder(t, &PathContext{
				Schema: tc.bodySchema,
				Files: map[string]*hcl.File{
					"test.tf": f,
				},
			})

			tokens, err := d.SemanticTokensInFile(ctx, "test.tf")
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.expectedTokens, tokens, ctydebug.CmpOptions); diff != "" {
				t.Fatalf("unexpected tokens: %s", diff)
			}
		})
	}
}
