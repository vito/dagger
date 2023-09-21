package schema

import (
	"fmt"
	"log"
	"runtime/debug"
	"sort"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/core/idproto"
	"github.com/dagger/dagger/core/pipeline"
	"github.com/dagger/dagger/core/resourceid"
	"github.com/dagger/graphql"
	"github.com/mitchellh/mapstructure"
	"github.com/vito/progrock"
)

type Resolvers map[string]Resolver

type Resolver interface {
	_resolver()
}

type FieldResolvers interface {
	Resolver
	Fields() map[string]graphql.FieldResolveFn
	SetField(string, graphql.FieldResolveFn)
}

type ObjectResolver map[string]graphql.FieldResolveFn

func (ObjectResolver) _resolver() {}

func (r ObjectResolver) Fields() map[string]graphql.FieldResolveFn {
	return r
}

func (r ObjectResolver) SetField(name string, fn graphql.FieldResolveFn) {
	r[name] = fn
}

func CacheByID(store *core.ObjectStore, obj ObjectResolver) ObjectResolver {
	wrapped := make(ObjectResolver, len(obj))
	for name, fn := range obj {
		wrapped[name] = wrap(store, fn)
	}
	return wrapped
}

func wrap(store *core.ObjectStore, fn graphql.FieldResolveFn) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		var shouldCache bool

		var id *idproto.ID
		if idable, ok := p.Source.(IDable); ok {
			id = idable.GetID()
		}

		if id == nil {
			id = idproto.New()
		}

		// if the query returns a non-nullable Object, calculate an ID for
		// referring to the object
		if nonNull, ok := p.Info.ReturnType.(*graphql.NonNull); ok {
			if retObj, ok := nonNull.OfType.(*graphql.Object); ok {
				args := make([]*idproto.Argument, 0, len(p.Args))
				for k, v := range p.Args {
					switch x := v.(type) {
					case string:
						if id, err := resourceid.Decode(x); err == nil {
							log.Println("!!! FOUND A STRING ID", k)
							v = id
						}
					}
					args = append(args, &idproto.Argument{
						Name:  k,
						Value: idproto.LiteralValue(v),
					})
				}
				sort.Slice(args, func(i, j int) bool {
					return args[i].Name < args[j].Name
				})

				id = id.Chain(retObj.Name(), p.Info.FieldName, args...)

				log.Println("!!! I HAVE CHAINED THE ID TO", id)

				shouldCache = true
			} else {
				log.Println("!!! RETURN IS NOT OBJECT", nonNull)
			}
		} else {
			log.Println("!!! RETURN IS NULLABLE", p.Info.ReturnType)
		}

		var res any
		var err error
		if shouldCache {
			dig, _ := id.Digest()
			log.Println("!!! LOADING OR SAVING", dig)
			res, err = store.LoadOrSave(id, func() (any, error) {
				res, err := fn(p)
				if err != nil {
					return nil, err
				}

				obj, ok := res.(IDable)
				if !ok {
					// NB: if you're here, perhaps you forgot to embed IDable in the
					// struct?
					//
					// this is all built on the premise that all objects are ID-able.
					return nil, fmt.Errorf(
						"unexpected: %s.%s: %s returned %T which is not IDable",
						p.Info.ParentType,
						p.Info.FieldName,
						p.Info.ReturnType,
						res,
					)
				}

				if obj.GetID() == nil {
					// by default, set the ID to the query ID that constructed the object.
					//
					// resolvers are free to set an ID of their own if they want to have
					// more control, for example container.from could return an ID that is
					// pinned and therefore not "tainted" by an unresolved tag.
					//
					// NB: this mutates in-place; the assumption is that if no ID is set,
					// this is a newly created object and it's OK to mutate it before it
					// gets saved away.
					obj.SetID(id)

					log.Println("!!! SETTING DEFAULT ID", id)
				} else {
					log.Println("!!! RESPECTING EXISTING ID")
				}

				return obj, nil
			})
		} else {
			log.Println("!!! NOT BOTHERING TO CACHE")
			res, err = fn(p)
		}
		if err != nil {
			return nil, err
		}

		return res, nil
	}
}

type ScalarResolver struct {
	Serialize    graphql.SerializeFn
	ParseValue   graphql.ParseValueFn
	ParseLiteral graphql.ParseLiteralFn
}

func (ScalarResolver) _resolver() {}

type IDable interface {
	GetID() *idproto.ID
	SetID(*idproto.ID)
}

// ToResolver transforms any function f with a *Context, a parent P and some args A that returns a Response R and an error
// into a graphql resolver graphql.FieldResolveFn.
func ToResolver[P any, A any, R any](f func(*core.Context, P, A) (R, error)) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		defer func() {
			if err := recover(); err != nil {
				log.Println("!!! PICNIC")
				debug.PrintStack()
			}
		}()

		recorder := progrock.FromContext(p.Context)

		var args A
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result:           &args,
			ErrorUnused:      true,
			WeaklyTypedInput: true,
			DecodeHook:       mapstructure.TextUnmarshallerHookFunc(),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create decoder: %w", err)
		}
		if err := decoder.Decode(p.Args); err != nil {
			return nil, fmt.Errorf("failed to decode args: %w", err)
		}

		var parent P
		if err := mapstructure.Decode(p.Source, &parent); err != nil {
			return nil, fmt.Errorf("source is wrong type: %T != %T", p.Source, parent)
		}

		if pipelineable, ok := p.Source.(pipeline.Pipelineable); ok {
			recorder = pipelineable.PipelinePath().RecorderGroup(recorder)
			p.Context = progrock.ToContext(p.Context, recorder)
		}

		vtx, err := queryVertex(recorder, p.Info.FieldName, p.Source, args)
		if err != nil {
			return nil, err
		}

		ctx := core.Context{
			Context:       p.Context,
			ResolveParams: p,
			Vertex:        vtx,
		}

		res, err := f(&ctx, parent, args)
		if err != nil {
			vtx.Done(err)
			return nil, err
		}

		if edible, ok := any(res).(resourceid.Digestible); ok {
			dg, err := edible.Digest()
			if err != nil {
				return nil, fmt.Errorf("failed to compute digest: %w", err)
			}
			vtx.Output(dg)
		}

		vtx.Done(nil)

		return res, nil
	}
}

func StaticResolver(val any) graphql.FieldResolveFn {
	return ToResolver(func(*core.Context, any, any) (any, error) {
		return val, nil
	})
}

func PassthroughResolver(p graphql.ResolveParams) (any, error) {
	return ToResolver(func(ctx *core.Context, parent any, args any) (any, error) {
		if parent == nil {
			parent = struct{}{}
		}
		return parent, nil
	})(p)
}

func ErrResolver(err error) graphql.FieldResolveFn {
	return ToResolver(func(ctx *core.Context, parent any, args any) (any, error) {
		return nil, err
	})
}
