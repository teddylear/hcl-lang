// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	// "bytes"
	// "context"
	// "sort"
    // "fmt"

	// "github.com/hashicorp/hcl-lang/decoder/internal/ast"
	// "github.com/hashicorp/hcl-lang/decoder/internal/schemahelper"
	"github.com/hashicorp/hcl-lang/lang"
    "github.com/hashicorp/hcl-lang/reference"
	// "github.com/hashicorp/hcl-lang/schema"
	"github.com/hashicorp/hcl/v2"
	// "github.com/hashicorp/hcl/v2/ext/typeexpr"
	// "github.com/zclconf/go-cty/cty"
)

// TODO: better name later ~ ReferenceTargetsForOriginAtPos
// TODO: Add another return type
func (d *Decoder) RenameTargets(path lang.Path, file string, pos hcl.Pos) (error) {
	pathCtx, err := d.pathReader.PathContext(path)
	if err != nil {
		return err
	}

	// matchingTargets := make(ReferenceTargets, 0)

	// origins, ok := pathCtx.ReferenceOrigins.AtPos(file, pos)
	_, ok := pathCtx.ReferenceOrigins.AtPos(file, pos)
	if !ok {
		// return matchingTargets, &reference.NoOriginFound{}
		return &reference.NoOriginFound{}
	}

    return nil
}
