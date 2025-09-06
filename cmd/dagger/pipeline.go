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

	executed := make(map[string]bool)
	haveTypes := map[string]any{}

	// Helper function to check if a function can be executed
	canExecute := func(fn *modFunction) bool {
		if executed[fn.Name] {
			return false
		}
		for _, arg := range fn.RequiredArgs() {
			if arg.TypeDef.AsObject == nil {
				// we can only call functions whose return values came from prior
				// functions; the presence of a core type or non-object type implies
				// that an input needs to be manually provided somehow (maybe via the
				// web UI?)
				return false
			}
			if _, have := haveTypes[arg.TypeDef.AsObject.Name]; !have {
				return false
			}
		}
		return true
	}

	// Keep running functions until no more can be executed
	for {
		// Find next runnable function
		var nextFn *modFunction
		for _, fn := range allFns {
			if canExecute(fn) {
				nextFn = fn
				break
			}
		}

		if nextFn == nil {
			break
		}

		executed[nextFn.Name] = true

		sel := obj.Select(nextFn.Name)
		// Add arguments if needed
		for _, arg := range nextFn.RequiredArgs() {
			if arg.TypeDef.AsObject != nil {
				if val, have := haveTypes[arg.TypeDef.AsObject.Name]; have {
					sel = sel.Arg(arg.Name, val)
				}
			}
		}

		q := handleObjectLeaf(sel, nextFn.ReturnType)

		var response any
		if err := makeRequest(ctx, q, &response); err != nil {
			return err
		}

		if obj := nextFn.ReturnType.AsObject; obj != nil {
			if _, alreadyHave := haveTypes[obj.Name]; alreadyHave {
				return fmt.Errorf("result type %q returned by multiple functions", obj.Name)
			}
			haveTypes[obj.Name] = response
		}
	}

	return nil
}
