package schema

import (
	"context"

	"github.com/dagger/dagger/core"
	"github.com/vito/dagql"
)

type secretSchema struct {
	srv *dagql.Server
}

var _ SchemaResolvers = &secretSchema{}

func (s *secretSchema) Install() {
	dagql.Fields[*core.Query]{
		dagql.Func("setSecret", s.setSecret).Impure().
			Doc(`Sets a secret given a user defined name to its plaintext and returns the secret.`,
				`The plaintext value is limited to a size of 128000 bytes.`).
			ArgDoc("name", `The user defined name for this secret`).
			ArgDoc("plaintext", `The plaintext of the secret`),

		dagql.Func("secret", s.secret).
			Doc(`Loads a secret from its ID.`).
			Deprecated("Use `loadSecretFromID` instead."),
	}.Install(s.srv)

	dagql.Fields[*core.Secret]{
		dagql.Func("plaintext", s.plaintext).Impure().
			Doc(`The value of this secret.`),
	}.Install(s.srv)
}

type secretArgs struct {
	ID core.SecretID
}

func (s *secretSchema) secret(ctx context.Context, parent *core.Query, args secretArgs) (dagql.Instance[*core.Secret], error) {
	return args.ID.Load(ctx, s.srv)
}

type SecretPlaintext dagql.String

// This method ensures that the progrock vertex info does not display the plaintext.
func (s SecretPlaintext) MarshalText() ([]byte, error) {
	//FIXME(vito): use this again, or delete it
	return []byte("***"), nil
}

type setSecretArgs struct {
	Name      dagql.String
	Plaintext dagql.String
}

func (s *secretSchema) setSecret(ctx context.Context, parent *core.Query, args setSecretArgs) (*core.Secret, error) {
	if err := parent.Secrets.AddSecret(ctx, args.Name.String(), []byte(args.Plaintext)); err != nil {
		return nil, err
	}

	// FIXME(vito): return an Object instead that just gets the secret by name,
	// to avoid putting the plaintext value in the graph

	return &core.Secret{
		Query: parent,
		Name:  args.Name.String(),
	}, nil
}

func (s *secretSchema) plaintext(ctx context.Context, parent *core.Secret, args struct{}) (dagql.String, error) {
	bytes, err := parent.Plaintext(ctx)
	if err != nil {
		return "", err
	}

	return dagql.NewString(string(bytes)), nil
}
