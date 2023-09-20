package core

import (
	"fmt"

	"github.com/dagger/dagger/core/idproto"
	"github.com/dagger/dagger/core/resourceid"
)

type ContainerID = resourceid.ID[Container]

type SocketID = resourceid.ID[Socket]

type ServiceID = resourceid.ID[Service]

type CacheID = resourceid.ID[CacheVolume]

type DirectoryID = resourceid.ID[Directory]

type FileID = resourceid.ID[File]

type SecretID = resourceid.ID[Secret]

type ModuleID = resourceid.ID[Module]

type FunctionID = resourceid.ID[Function]

// ResourceFromID returns the resource corresponding to the given ID.
func ResourceFromID(idStr string) (any, error) {
	id, err := resourceid.Decode(idStr)
	if err != nil {
		return nil, err
	}
	switch id.TypeName {
	case ContainerID{}.ResourceTypeName():
		return ContainerID{ID: id}.Decode()
	case CacheID{}.ResourceTypeName():
		return CacheID{ID: id}.Decode()
	case DirectoryID{}.ResourceTypeName():
		return DirectoryID{ID: id}.Decode()
	case FileID{}.ResourceTypeName():
		return FileID{ID: id}.Decode()
	case SecretID{}.ResourceTypeName():
		return SecretID{ID: id}.Decode()
	case resourceid.ID[Module]{}.ResourceTypeName():
		return ModuleID{ID: id}.Decode()
	case FunctionID{}.ResourceTypeName():
		return FunctionID{ID: id}.Decode()
	case SocketID{}.ResourceTypeName():
		return SocketID{ID: id}.Decode()
	}
	return nil, fmt.Errorf("unknown resource type: %v", id.TypeName)
}

// ResourceToID returns the ID string corresponding to the given resource.
func ResourceToID(r any) (*idproto.ID, error) {
	var id *idproto.ID
	switch r := r.(type) {
	case *Container:
		id = r.ID
	case *CacheVolume:
		id = r.ID
	case *Directory:
		id = r.ID
	case *File:
		id = r.ID
	case *Secret:
		id = r.ID
	case *Module:
		id = r.ID
	case *Function:
		id = r.ID
	case *Socket:
		id = r.ID
	default:
		return nil, fmt.Errorf("unknown resource type: %T", r)
	}
	if id == nil {
		return nil, fmt.Errorf("%T has a null ID", r) // TODO this might be valid
	}
	return id, nil
}
