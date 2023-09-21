package core

import (
	"fmt"
	"log"
	"reflect"

	"github.com/dagger/dagger/core/idproto"
	"github.com/dagger/dagger/core/resourceid"
	"github.com/opencontainers/go-digest"
)

type ContainerID = resourceid.ID[*Container]

type SocketID = resourceid.ID[*Socket]

type ServiceID = resourceid.ID[*Service]

type CacheID = resourceid.ID[*CacheVolume]

type DirectoryID = resourceid.ID[*Directory]

type FileID = resourceid.ID[*File]

type SecretID = resourceid.ID[*Secret]

type ModuleID = resourceid.ID[*Module]

type FunctionID = resourceid.ID[*Function]

// IDable can be embedded into any object to make the object automatically
// ID-able.
//
// Typically, an object that embeds this also implements Clone() to make a copy
// of the object. As part of Clone() it should also set the ID to nil to
// reflect that this new copy of the object differs from the original, i.e.
// that it cannot be accessed from the same ID.
//
// A resolver may choose to manually ID on a newly created object, or it may
// leave it nil, in which case the object ID will be automatically assigned to
// the ID from the query that created it.
type IDable struct {
	ID *idproto.ID `json:"id"`
}

// GetID returns the current ID, if any.
func (id *IDable) GetID() *idproto.ID {
	return id.ID
}

// SetID sets the ID.
func (id *IDable) SetID(id_ *idproto.ID) {
	id.ID = id_
}

type ObjectStore struct {
	id2object *CacheMap[digest.Digest, any]
}

func NewIDStore() *ObjectStore {
	return &ObjectStore{
		id2object: NewCacheMap[digest.Digest, any](),
	}
}

func (store *ObjectStore) Load(id *idproto.ID, dest any) error {
	if id == nil {
		return fmt.Errorf("cannot load nil ID")
	}
	dig, err := id.Digest()
	if err != nil {
		return err
	}
	log.Println("!!! STORE LOAD", dig, id)
	val, err := store.id2object.Get(dig)
	if err != nil {
		return err
	}
	destV := reflect.ValueOf(dest)
	valV := reflect.ValueOf(val)
	if destV.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be a pointer")
	}
	if !valV.Type().AssignableTo(destV.Elem().Type()) {
		return fmt.Errorf("destination type %T does not match value type %T", dest, val)
	}
	destV.Elem().Set(valV)
	return nil
}

func (store *ObjectStore) LoadOrSave(id *idproto.ID, init func() (any, error)) (any, error) {
	dig, err := id.Digest()
	if err != nil {
		return nil, err
	}
	log.Println("!!! STORE SAVE [OR LOAD]", dig, id)
	// TODO save by returned ID instead? how to deal with secrets
	return store.id2object.GetOrInitialize(dig, init)
}
