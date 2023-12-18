package schema

// // The objects defined by this module, with namespacing applied
// func (m *UserMod) MainModuleObject(ctx context.Context) (*UserModObject, error) {
// 	objs, err := m.Objects(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	for _, obj := range objs {
// 		if obj.typeDef.AsObject.Value.Name == gqlObjectName(m.Name()) {
// 			return obj, nil
// 		}
// 	}
// 	return nil, fmt.Errorf("failed to find main module object %q", m.Name())
// }
