package schema

import (
	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/core/resourceid"
)

type secretSchema struct {
	*MergedSchemas
}

var _ ExecutableSchema = &secretSchema{}

func (s *secretSchema) Name() string {
	return "secret"
}

func (s *secretSchema) Schema() string {
	return Secret
}

func (s *secretSchema) Resolvers() Resolvers {
	return Resolvers{
		"Query": CacheByID(s.objects, ObjectResolver{
			"secret":    ToResolver(s.secret),
			"setSecret": ToResolver(s.setSecret),
		}),
		"Secret": CacheByID(s.objects, ObjectResolver{
			"id":        ToResolver(s.id),
			"plaintext": ToResolver(s.plaintext),
		}),
	}
}

func (s *secretSchema) Dependencies() []ExecutableSchema {
	return nil
}

func (s *secretSchema) id(ctx *core.Context, parent *core.Secret, args any) (core.SecretID, error) {
	return resourceid.FromProto[*core.Secret](parent.ID), nil
}

type secretArgs struct {
	ID core.SecretID
}

func (s *secretSchema) secret(ctx *core.Context, parent any, args secretArgs) (*core.Secret, error) {
	return args.ID.Decode(s.objects)
}

type SecretPlaintext string

// This method ensures that the progrock vertex info does not display the plaintext.
func (s SecretPlaintext) MarshalText() ([]byte, error) {
	return []byte("***"), nil
}

type setSecretArgs struct {
	Name      string
	Plaintext SecretPlaintext
}

func (s *secretSchema) setSecret(ctx *core.Context, parent any, args setSecretArgs) (*core.Secret, error) {
	s.secrets.AddSecret(args.Name, []byte(args.Plaintext))
	return core.NewCanonicalSecret(args.Name), nil
}

func (s *secretSchema) plaintext(ctx *core.Context, parent *core.Secret, args any) (string, error) {
	bytes, err := s.secrets.GetSecret(ctx, parent.Name)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
