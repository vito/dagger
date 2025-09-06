package main

import (
	"context"
	"fmt"

	"dagger.io/dagger"
)

func pipelineRun(ctx context.Context, dag *dagger.Client, returnType *modTypeDef, response any) error {
	allFns := returnType.AsFunctionProvider().GetFunctions()

	obj := dag.QueryBuilder().
		Select(fmt.Sprintf("load%sFromID", returnType.Name())).
		Arg("id", response)

	// example module functions:
	//
	// Build(): Built!
	// Test(): Tested!
	// Release(built: Built!, tested: Tested!): Released!

	// 1. identify leaf functions: functions that can be called without any
	// arguments
	// 2. call each leaf against a newly connected engine (e.g. so `--cloud` runs
	// against MULTIPLE engines, not all on one)
	// 2. collect results: if a function returns a type like "Built", retain it
	// 3. as calls complete, check if any new functions can be called with the
	// collected results

	var runnable []*modFunction
	for _, fn := range allFns {
		if len(fn.RequiredArgs()) == 0 {
			runnable = append(runnable, fn)
		}
	}

	haveTypes := map[string]any{}
	for i := 0; i < len(runnable); i++ {
		fn := runnable[i]
		sel := obj.Select(fn.Name)
		q := handleObjectLeaf(sel, fn.ReturnType)

		var response any
		if err := makeRequest(ctx, q, &response); err != nil {
			return err
		}

		if obj := fn.ReturnType.AsObject; obj != nil && !obj.IsCore() {
			if _, alreadyHave := haveTypes[obj.Name]; alreadyHave {
				return fmt.Errorf("result type %q returned by multiple functions", obj.Name)
			}
			haveTypes[obj.Name] = response
		}
	}

	return nil
}
