package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"dagger.io/dagger/telemetry"
	"github.com/dagger/dagger/dagql"
	"github.com/dagger/dagger/dagql/call"
	"github.com/vektah/gqlparser/v2/ast"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// A frontend for LLM tool calling
type LLMTool struct {
	// Tool name
	Name string
	// Tool description
	Description string
	// Tool argument schema. Key is argument name. Value is unmarshalled json-schema for the argument.
	Schema map[string]any
	// Function implementing the tool.
	Call func(context.Context, any) (any, error)
}

type LLMEnv struct {
	// Saved objects
	bindings map[string]*LLMBinding
	// The current binding
	current *LLMBinding
}

func NewLLMEnv() *LLMEnv {
	return &LLMEnv{
		bindings: make(map[string]*LLMBinding),
		current: &LLMBinding{
			Name:  "_",
			Value: nil,
		},
	}
}

type LLMBinding struct {
	Name     string
	Value    dagql.Object
	Previous *LLMBinding
}

func (env *LLMEnv) Clone() *LLMEnv {
	cp := *env
	cp.bindings = cloneMap(env.bindings)
	return &cp
}

// Lookup dagql typedef for a given dagql value
func (env *LLMEnv) typedef(srv *dagql.Server, val dagql.Typed) *ast.Definition {
	return srv.Schema().Types[val.Type().Name()]
}

// Return the current selection
func (env *LLMEnv) Current() dagql.Object {
	return env.current.Value
}

func (env *LLMEnv) Bind(bnd *LLMBinding) {
	env.current = bnd
}

func (env *LLMEnv) With(obj dagql.Object) {
	env.current = &LLMBinding{
		Name:  "_",
		Value: obj,
		// TODO: ensure needed
		// Previous: env.current,
	}
}

// Save a value at the given key
func (env *LLMEnv) Set(key string, value dagql.Object) string {
	bnd := &LLMBinding{Name: key, Value: value}
	prev := env.bindings[key]
	if prev != nil {
		bnd.Previous = prev
	}
	env.bindings[key] = bnd
	// if obj, ok := dagql.UnwrapAs[dagql.Object](value); ok {
	// 	env.objsByHash[obj.ID().Digest()] = value
	// }
	if prev != nil {
		return fmt.Sprintf("The binding %q has changed from %s to %s.", key, env.describe(prev.Value), env.describe(value))
	}
	return fmt.Sprintf("The binding %q has been set to %s.", key, env.describe(value))
}

// Get a value saved at the given key
func (env *LLMEnv) Get(key string) (dagql.Object, error) {
	if val, exists := env.bindings[key]; exists {
		return val.Value, nil
	}
	// if _, hash, ok := strings.Cut(key, "@"); ok {
	// 	// strip Type@ prefix if present
	// 	// TODO: figure out the best place to do this
	// 	key = hash
	// }
	// if val, exists := env.objsByHash[digest.Digest(key)]; exists {
	// 	return val, nil
	// }
	var dbg string
	for k, b := range env.bindings {
		dbg += fmt.Sprintf("binding %s: %s\n", k, b.Value.Type().Name())
	}
	return nil, fmt.Errorf("binding not found: %s\n\n%s", key, dbg)
}

// Unset a saved value
func (env *LLMEnv) Unset(key string) {
	delete(env.bindings, key)
}

func (env *LLMEnv) Tools(srv *dagql.Server) []LLMTool {
	return append(env.Builtins(srv), env.tools(srv, env.Current())...)
}

func (env *LLMEnv) tools(srv *dagql.Server, obj dagql.Typed) []LLMTool {
	if obj == nil {
		return nil
	}
	typedef := env.typedef(srv, obj)
	typeName := typedef.Name
	var tools []LLMTool
	for _, field := range typedef.Fields {
		if strings.HasPrefix(field.Name, "_") {
			continue
		}
		if strings.HasPrefix(field.Name, "load") && strings.HasSuffix(field.Name, "FromID") {
			continue
		}
		tools = append(tools, LLMTool{
			Name:        typeName + "_" + field.Name, // TODO: try var_field.Name?
			Description: field.Description,
			Schema:      fieldArgsToJSONSchema(field),
			Call: func(ctx context.Context, args any) (_ any, rerr error) {
				ctx, span := Tracer(ctx).Start(ctx,
					fmt.Sprintf("🤖💻 %s %v", typeName+"."+field.Name, args),
					telemetry.Passthrough(),
					telemetry.Reveal())
				defer telemetry.End(span, func() error {
					return rerr
				})
				result, err := env.call(ctx, srv, field, args)
				if err != nil {
					return nil, err
				}
				stdio := telemetry.SpanStdio(ctx, InstrumentationLibrary)
				defer stdio.Close()
				switch v := result.(type) {
				case string:
					fmt.Fprint(stdio.Stdout, v)
				default:
					enc := json.NewEncoder(stdio.Stdout)
					enc.SetIndent("", "  ")
					enc.Encode(v)
				}
				return result, nil
			},
		})
	}
	return tools
}

// Low-level function call plumbing
func (env *LLMEnv) call(ctx context.Context,
	srv *dagql.Server,
	// The definition of the dagql field to call. Example: Container.withExec
	fieldDef *ast.FieldDefinition,
	// The arguments to the call. Example: {"args": ["go", "build"], "redirectStderr", "/dev/null"}
	args any,
) (any, error) {
	// 1. CONVERT CALL INPUTS (BRAIN -> BODY)
	argsMap, ok := args.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tool call: %s: expected arguments to be a map - got %#v", fieldDef.Name, args)
	}
	cur := env.Current()
	if cur == nil {
		return nil, fmt.Errorf("no current context")
	}
	curObjType, ok := srv.ObjectType(cur.Type().Name())
	if !ok {
		return nil, fmt.Errorf("dagql object type not found: %s", cur.Type().Name())
	}
	// FIXME: we have to hardcode *a* version here, otherwise Container.withExec disappears
	// It's still kind of hacky
	field, ok := curObjType.FieldSpec(fieldDef.Name, "v0.13.2")
	if !ok {
		return nil, fmt.Errorf("field %q not found in object type %q", fieldDef.Name, curObjType)
	}
	fieldSel := dagql.Selector{
		Field: fieldDef.Name,
	}
	for _, arg := range field.Args {
		val, ok := argsMap[arg.Name]
		if !ok {
			continue
		}
		if _, ok := dagql.UnwrapAs[dagql.IDable](arg.Type); ok {
			if idStr, ok := val.(string); ok {
				envVal, err := env.Get(idStr)
				if err != nil {
					return nil, fmt.Errorf("tool call: %s: failed to get self: %w", fieldDef.Name, err)
				}
				if obj, ok := dagql.UnwrapAs[dagql.Object](envVal); ok {
					enc, err := obj.ID().Encode()
					if err != nil {
						return nil, fmt.Errorf("tool call: %s: failed to encode ID: %w", fieldDef.Name, err)
					}
					val = enc
				} else {
					return nil, fmt.Errorf("tool call: %s: expected object, got %T", fieldDef.Name, val)
				}
			} else {
				return nil, fmt.Errorf("tool call: %s: expected string, got %T", fieldDef.Name, val)
			}
		}
		input, err := arg.Type.Decoder().DecodeInput(val)
		if err != nil {
			return nil, fmt.Errorf("decode arg %q (%T): %w", arg.Name, val, err)
		}
		fieldSel.Args = append(fieldSel.Args, dagql.NamedInput{
			Name:  arg.Name,
			Value: input,
		})
	}
	// 2. MAKE THE CALL

	// 2a. OBJECT RETURN - update current binding if same type
	fieldTypeName := field.Type.Type().Name()
	if retObjType, ok := srv.ObjectType(fieldTypeName); ok {
		var retObj dagql.Object
		if sync, ok := retObjType.FieldSpec("sync"); ok {
			syncSel := dagql.Selector{
				Field: sync.Name,
			}
			idType, ok := retObjType.IDType()
			if !ok {
				return nil, fmt.Errorf("field %q is not an ID type", sync.Name)
			}
			if err := srv.Select(ctx, cur, &idType, fieldSel, syncSel); err != nil {
				return nil, fmt.Errorf("failed to sync: %w", err)
			}
			syncedObj, err := srv.Load(ctx, idType.ID())
			if err != nil {
				return nil, fmt.Errorf("failed to load synced object: %w", err)
			}
			retObj = syncedObj
		} else if err := srv.Select(ctx, cur, &retObj, fieldSel); err != nil {
			return nil, err
		}
		return env.UpdateOrBranch(retObj), nil
	}

	// 2b. SCALAR RETURN - just return the value
	var val dagql.Typed
	if err := srv.Select(ctx, cur, &val, fieldSel); err != nil {
		return nil, fmt.Errorf("failed to sync: %w", err)
	}
	if id, ok := val.(dagql.IDType); ok {
		// avoid dumping full IDs, show the type and hash instead
		return env.describe(id), nil
	}
	return val, nil
}

func (env *LLMEnv) UpdateOrBranch(retObj dagql.Object) string {
	if env.current.Value != nil {
		if env.current.Value.Type().Name() == retObj.Type().Name() {
			newBnd := env.current.Continue(retObj)
			env.bindings[env.current.Name] = newBnd
			env.current = newBnd
			return fmt.Sprintf("Updated $%s to %s (unsaved).", env.current.Name, env.describe(retObj))
		}
	}
	env.With(retObj)
	return fmt.Sprintf("Switched to %s (unsaved).", env.describe(retObj))
}

func (bnd *LLMBinding) Continue(retObj dagql.Object) *LLMBinding {
	return &LLMBinding{
		Name:     bnd.Name,
		Value:    retObj,
		Previous: bnd,
	}
}

func (env *LLMEnv) callObjects(ctx context.Context, _ any) (any, error) {
	var result string
	for name, obj := range env.bindings {
		result += "- " + name + " (" + env.describe(obj.Value) + ")\n"
	}
	return result, nil
}

// func (env *LLMEnv) callSelectTools(ctx context.Context, args any) (any, error) {
// 	name := args.(map[string]any)["name"].(string)
// 	value, err := env.Get(name)
// 	if err != nil {
// 		return nil, err
// 	}
// 	env.history = append(env.history, value)
// 	return fmt.Sprintf("Switched tools to %s.", env.describe(value)), nil
// }

func (env *LLMEnv) callSave(ctx context.Context, args any) (any, error) {
	name := args.(map[string]any)["name"].(string)
	return env.Set(name, env.Current()), nil
}

func (env *LLMEnv) callUndo(ctx context.Context, _ any) (any, error) {
	if env.current.Previous != nil {
		env.current = env.current.Previous
		env.bindings[env.current.Name] = env.current
	}
	return env.describe(env.Current()), nil
}

// describe returns a string representation of a typed object or object ID
func (env *LLMEnv) describe(val dagql.Typed) string {
	if val == nil {
		return fmt.Sprintf("<nil> (%T)", val)
	}
	if obj, ok := dagql.UnwrapAs[dagql.IDable](val); ok {
		return obj.ID().Type().ToAST().Name() + "@" + obj.ID().Digest().String()
	}
	if list, ok := dagql.UnwrapAs[dagql.Enumerable](val); ok {
		return "[" + val.Type().Name() + "] (length: " + strconv.Itoa(list.Len()) + ")"
	}
	return val.Type().Name()
}

func (env *LLMEnv) Builtins(srv *dagql.Server) []LLMTool {
	builtins := []LLMTool{
		// {
		// 	Name:        "_objects",
		// 	Description: "List saved objects with their types. IMPORTANT: call this any time you seem to be missing objects for the request. Learn what objects are available, and then learn what tools they provide.",
		// 	Schema: map[string]any{
		// 		"type":       "object",
		// 		"properties": map[string]any{},
		// 	},
		// 	Call: env.callObjects,
		// },
		// {
		// 	Name:        "_selectTools",
		// 	Description: "Load an object's functions/tools. IMPORTANT: call this any time you seem to be missing tools for the request. This is a cheap option, so there is never a reason to give up without trying it first.",
		// 	Schema: map[string]any{
		// 		"type": "object",
		// 		"properties": map[string]any{
		// 			"name": map[string]any{
		// 				"type":        "string",
		// 				"description": "Variable name or hash of the object to load",
		// 			},
		// 		},
		// 		"required": []string{"name"},
		// 	},
		// 	Call: env.callSelectTools,
		// },
		{
			Name:        "_save",
			Description: "Save the current object as a named variable",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Variable name to save the object as",
					},
				},
				"required":             []string{"name"},
				"strict":               true,
				"additionalProperties": false,
			},
			Call: env.callSave,
		},
		{
			Name:        "_undo",
			Description: "Roll back the last action",
			Schema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"strict":               true,
				"required":             []string{},
				"additionalProperties": false,
			},
			Call: env.callUndo,
		},
		// TODO: don't think we need this
		// {
		// 	Name:        "_type",
		// 	Description: "Print the type of a saved object",
		// 	Schema: map[string]any{
		// 		"type": "object",
		// 		"properties": map[string]any{
		// 			"name": map[string]any{
		// 				"type":        "string",
		// 				"description": "Variable name to print the type of",
		// 			},
		// 		},
		// 	},
		// 	Call: env.callType,
		// },
		// {
		// 	Name:        "_current",
		// 	Description: "Print the value of the current object",
		// 	Schema: map[string]any{
		// 		"type":       "object",
		// 		"properties": map[string]any{},
		// 	},
		// 	Call: env.callCurrent,
		// },
		{
			Name:        "_scratch",
			Description: "Clear the current environment",
			Schema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"strict":               true,
				"required":             []string{},
				"additionalProperties": false,
			},
			Call: func(ctx context.Context, _ any) (any, error) {
				// TODO: ?
				env.current = &LLMBinding{
					Name:     "_",
					Value:    nil,
					Previous: env.current,
				}
				return nil, nil
			},
		},
	}
	for name, bnd := range env.bindings {
		desc := fmt.Sprintf("Bind the environment to %s (%s):\n", name, env.describe(bnd.Value))
		tools := env.tools(srv, bnd.Value)
		for _, tool := range tools {
			desc += fmt.Sprintf("\n- %s", tool.Name)
		}
		builtins = append(builtins, LLMTool{
			Name:        "_select_" + name,
			Description: desc,
			Schema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"strict":               true,
				"required":             []string{},
				"additionalProperties": false,
			},
			Call: func(ctx context.Context, _ any) (any, error) {
				env.Bind(bnd)
				return fmt.Sprintf("Switched environment to $%s.", name), nil
			},
		})
		// builtins = append(builtins, LLMTool{
		// 	Name:        "_duplicate_" + name,
		// 	Description: "Set a new variable starting from $" + name,
		// 	Schema: map[string]any{
		// 		"type": "object",
		// 		"properties": map[string]any{
		// 			"name": map[string]any{
		// 				"type": "string",
		// 			},
		// 		},
		// 		"required": []string{"name"},
		// 	},
		// 	Call: func(ctx context.Context, _ any) (any, error) {
		// 		env.With(name, bnd)
		// 		return fmt.Sprintf("Switched environment to $%s.", name), nil
		// 	},
		// })
	}
	// Attach builtin telemetry
	for i, builtin := range builtins {
		builtins[i].Call = func(ctx context.Context, args any) (_ any, rerr error) {
			id := toolToID(builtin.Name, args)
			callAttr, err := id.Call().Encode()
			if err != nil {
				return nil, err
			}
			ctx, span := Tracer(ctx).Start(ctx, builtin.Name,
				trace.WithAttributes(
					attribute.String(telemetry.DagDigestAttr, id.Digest().String()),
					attribute.String(telemetry.DagCallAttr, callAttr),
					attribute.String(telemetry.UIActorEmojiAttr, "🤖"),
				),
				telemetry.Reveal())
			defer telemetry.End(span, func() error { return rerr })
			stdio := telemetry.SpanStdio(ctx, InstrumentationLibrary)
			defer stdio.Close()
			res, err := builtin.Call(ctx, args)
			if err != nil {
				return nil, err
			}
			fmt.Fprintln(stdio.Stdout, res)
			return res, nil
		}
	}
	return builtins
}

func toolToID(name string, args any) *call.ID {
	var callArgs []*call.Argument
	if argsMap, ok := args.(map[string]any); ok {
		for k, v := range argsMap {
			lit, err := call.ToLiteral(v)
			if err != nil {
				lit = call.NewLiteralString(fmt.Sprintf("!(%v)(%s)", v, err))
			}
			callArgs = append(callArgs, call.NewArgument(k, lit, false))
		}
	}
	return call.New().Append(
		&ast.Type{
			NamedType: "String",
			NonNull:   true,
		},
		name, // fn name
		"",   // view
		nil,  // module
		0,    // nth
		"",   // custom digest
		callArgs...,
	)
}

func fieldArgsToJSONSchema(field *ast.FieldDefinition) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	properties := schema["properties"].(map[string]any)
	required := []string{}
	for _, arg := range field.Arguments {
		argSchema := typeToJSONSchema(arg.Type)

		// Add description if present
		if arg.Description != "" {
			argSchema["description"] = arg.Description
		}

		// Add default value if present
		if arg.DefaultValue != nil {
			argSchema["default"] = arg.DefaultValue.Raw
		}

		properties[arg.Name] = argSchema

		// Track required fields (non-null without default)
		if arg.Type.NonNull && arg.DefaultValue == nil {
			required = append(required, arg.Name)
		}
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func typeToJSONSchema(t *ast.Type) map[string]any {
	schema := map[string]any{}

	// Handle lists
	if t.Elem != nil {
		schema["type"] = "array"
		schema["items"] = typeToJSONSchema(t.Elem)
		return schema
	}

	// Handle base types
	switch t.NamedType {
	case "Int":
		schema["type"] = "integer"
	case "Float":
		schema["type"] = "number"
	case "String":
		schema["type"] = "string"
	case "Boolean":
		schema["type"] = "boolean"
	case "ID":
		schema["type"] = "string"
		schema["format"] = "id"
	default:
		// For custom types, use string format with the type name
		schema["type"] = "string"
		schema["format"] = t.NamedType
	}

	return schema
}
