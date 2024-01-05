// Code generated by dagger. DO NOT EDIT.

package dag

import (
	"context"
	"os"
	"sync"

	dagger "dagger.io/dagger"
)

var client *dagger.Client
var clientMu sync.Mutex

func initClient() *dagger.Client {
	clientMu.Lock()
	defer clientMu.Unlock()

	if client == nil {
		var err error
		client, err = dagger.Connect(context.Background(), dagger.WithLogOutput(os.Stdout))
		if err != nil {
			panic(err)
		}
	}
	return client
}

// Close the engine connection
func Close() error {
	clientMu.Lock()
	defer clientMu.Unlock()

	var err error
	if client != nil {
		err = client.Close()
		client = nil
	}
	return err
}

func Blob(digest string, size int, mediaType string, uncompressed string) *dagger.Directory {
	client := initClient()
	return client.Blob(digest, size, mediaType, uncompressed)
}

func CacheVolume(key string) *dagger.CacheVolume {
	client := initClient()
	return client.CacheVolume(key)
}

func CheckVersionCompatibility(ctx context.Context, version string) (bool, error) {
	client := initClient()
	return client.CheckVersionCompatibility(ctx, version)
}

func Container(opts ...dagger.ContainerOpts) *dagger.Container {
	client := initClient()
	return client.Container(opts...)
}

func CurrentFunctionCall() *dagger.FunctionCall {
	client := initClient()
	return client.CurrentFunctionCall()
}

func CurrentModule() *dagger.Module {
	client := initClient()
	return client.CurrentModule()
}

func CurrentTypeDefs(ctx context.Context) ([]dagger.TypeDef, error) {
	client := initClient()
	return client.CurrentTypeDefs(ctx)
}

func DefaultPlatform(ctx context.Context) (dagger.Platform, error) {
	client := initClient()
	return client.DefaultPlatform(ctx)
}

func Directory(opts ...dagger.DirectoryOpts) *dagger.Directory {
	client := initClient()
	return client.Directory(opts...)
}

func File(id dagger.FileID) *dagger.File {
	client := initClient()
	return client.File(id)
}

func Function(name string, returnType *dagger.TypeDef) *dagger.Function {
	client := initClient()
	return client.Function(name, returnType)
}

func GeneratedCode(code *dagger.Directory) *dagger.GeneratedCode {
	client := initClient()
	return client.GeneratedCode(code)
}

func Git(url string, opts ...dagger.GitOpts) *dagger.GitRepository {
	client := initClient()
	return client.Git(url, opts...)
}

func Host() *dagger.Host {
	client := initClient()
	return client.Host()
}

func HTTP(url string, opts ...dagger.HTTPOpts) *dagger.File {
	client := initClient()
	return client.HTTP(url, opts...)
}

func LoadCacheVolumeFromID(id dagger.CacheVolumeID) *dagger.CacheVolume {
	client := initClient()
	return client.LoadCacheVolumeFromID(id)
}

func LoadContainerFromID(id dagger.ContainerID) *dagger.Container {
	client := initClient()
	return client.LoadContainerFromID(id)
}

func LoadDirectoryFromID(id dagger.DirectoryID) *dagger.Directory {
	client := initClient()
	return client.LoadDirectoryFromID(id)
}

func LoadEnvVariableFromID(id dagger.EnvVariableID) *dagger.EnvVariable {
	client := initClient()
	return client.LoadEnvVariableFromID(id)
}

func LoadFieldTypeDefFromID(id dagger.FieldTypeDefID) *dagger.FieldTypeDef {
	client := initClient()
	return client.LoadFieldTypeDefFromID(id)
}

func LoadFileFromID(id dagger.FileID) *dagger.File {
	client := initClient()
	return client.LoadFileFromID(id)
}

func LoadFunctionArgFromID(id dagger.FunctionArgID) *dagger.FunctionArg {
	client := initClient()
	return client.LoadFunctionArgFromID(id)
}

func LoadFunctionCallArgValueFromID(id dagger.FunctionCallArgValueID) *dagger.FunctionCallArgValue {
	client := initClient()
	return client.LoadFunctionCallArgValueFromID(id)
}

func LoadFunctionCallFromID(id dagger.FunctionCallID) *dagger.FunctionCall {
	client := initClient()
	return client.LoadFunctionCallFromID(id)
}

func LoadFunctionFromID(id dagger.FunctionID) *dagger.Function {
	client := initClient()
	return client.LoadFunctionFromID(id)
}

func LoadGeneratedCodeFromID(id dagger.GeneratedCodeID) *dagger.GeneratedCode {
	client := initClient()
	return client.LoadGeneratedCodeFromID(id)
}

func LoadGitRefFromID(id dagger.GitRefID) *dagger.GitRef {
	client := initClient()
	return client.LoadGitRefFromID(id)
}

func LoadGitRepositoryFromID(id dagger.GitRepositoryID) *dagger.GitRepository {
	client := initClient()
	return client.LoadGitRepositoryFromID(id)
}

func LoadHostFromID(id dagger.HostID) *dagger.Host {
	client := initClient()
	return client.LoadHostFromID(id)
}

func LoadInterfaceTypeDefFromID(id dagger.InterfaceTypeDefID) *dagger.InterfaceTypeDef {
	client := initClient()
	return client.LoadInterfaceTypeDefFromID(id)
}

func LoadLabelFromID(id dagger.LabelID) *dagger.Label {
	client := initClient()
	return client.LoadLabelFromID(id)
}

func LoadListTypeDefFromID(id dagger.ListTypeDefID) *dagger.ListTypeDef {
	client := initClient()
	return client.LoadListTypeDefFromID(id)
}

func LoadModuleConfigFromID(id dagger.ModuleConfigID) *dagger.ModuleConfig {
	client := initClient()
	return client.LoadModuleConfigFromID(id)
}

func LoadModuleFromID(id dagger.ModuleID) *dagger.Module {
	client := initClient()
	return client.LoadModuleFromID(id)
}

func LoadObjectTypeDefFromID(id dagger.ObjectTypeDefID) *dagger.ObjectTypeDef {
	client := initClient()
	return client.LoadObjectTypeDefFromID(id)
}

func LoadPortFromID(id dagger.PortID) *dagger.Port {
	client := initClient()
	return client.LoadPortFromID(id)
}

func LoadSecretFromID(id dagger.SecretID) *dagger.Secret {
	client := initClient()
	return client.LoadSecretFromID(id)
}

func LoadServiceFromID(id dagger.ServiceID) *dagger.Service {
	client := initClient()
	return client.LoadServiceFromID(id)
}

func LoadSocketFromID(id dagger.SocketID) *dagger.Socket {
	client := initClient()
	return client.LoadSocketFromID(id)
}

func LoadTypeDefFromID(id dagger.TypeDefID) *dagger.TypeDef {
	client := initClient()
	return client.LoadTypeDefFromID(id)
}

func Module() *dagger.Module {
	client := initClient()
	return client.Module()
}

func ModuleConfig(sourceDirectory *dagger.Directory, opts ...dagger.ModuleConfigOpts) *dagger.ModuleConfig {
	client := initClient()
	return client.ModuleConfig(sourceDirectory, opts...)
}

func Pipeline(name string, opts ...dagger.PipelineOpts) *dagger.Client {
	client := initClient()
	return client.Pipeline(name, opts...)
}

func Secret(id dagger.SecretID) *dagger.Secret {
	client := initClient()
	return client.Secret(id)
}

func SetSecret(name string, plaintext string) *dagger.Secret {
	client := initClient()
	return client.SetSecret(name, plaintext)
}

func Socket(id dagger.SocketID) *dagger.Socket {
	client := initClient()
	return client.Socket(id)
}

func TypeDef() *dagger.TypeDef {
	client := initClient()
	return client.TypeDef()
}
