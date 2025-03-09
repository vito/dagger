package schema

import (
	"context"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/dagql"
)

type llmSchema struct {
	srv *dagql.Server
}

var _ SchemaResolvers = &llmSchema{}

func (s llmSchema) Install() {
	dagql.Fields[*core.Query]{
		dagql.Func("llm", s.llm).
			Doc(`Initialize a Large Language Model (LLM)`).
			ArgDoc("model", "Model to use").
			ArgDoc("maxAPICalls", "Cap the number of API calls for this LLM"),
	}.Install(s.srv)
	llmType := dagql.Fields[*core.Llm]{
		dagql.Func("model", s.model).
			Doc("return the model used by the llm"),
		dagql.Func("provider", s.provider).
			Doc("return the provider used by the llm"),
		dagql.Func("history", s.history).
			Doc("return the llm message history"),
		dagql.Func("lastReply", s.lastReply).
			Doc("return the last llm reply from the history"),
		dagql.Func("withModel", s.withModel).
			Doc("swap out the llm model").
			ArgDoc("model", "The model to use"),
		dagql.Func("withPrompt", s.withPrompt).
			Doc("append a prompt to the llm context").
			ArgDoc("prompt", "The prompt to send"),
		dagql.Func("withPromptFile", s.withPromptFile).
			Doc("append the contents of a file to the llm context").
			ArgDoc("file", "The file to read the prompt from"),
		dagql.Func("setString", s.setString).
			Doc("Add a string variable to the LLM's environment").
			ArgDoc("name", "The variable name").
			ArgDoc("value", "The variable value"),
		dagql.NodeFunc("sync", func(ctx context.Context, self dagql.Instance[*core.Llm], _ struct{}) (dagql.ID[*core.Llm], error) {
			var zero dagql.ID[*core.Llm]
			var inst dagql.Instance[*core.Llm]
			if err := s.srv.Select(ctx, self, &inst, dagql.Selector{
				Field: "loop",
			}); err != nil {
				return zero, err
			}
			return dagql.NewID[*core.Llm](inst.ID()), nil
		}).
			Doc("synchronize LLM state"),
		dagql.Func("loop", s.loop).
			Deprecated("use sync").
			Doc("synchronize LLM state"),
		dagql.Func("tools", s.tools).
			Doc("print documentation for available tools"),
	}
	llmType.Install(s.srv)
	s.srv.SetMiddleware(core.LlmMiddleware{Server: s.srv})
}

func (s *llmSchema) model(ctx context.Context, llm *core.Llm, args struct{}) (string, error) {
	return llm.Endpoint.Model, nil
}

func (s *llmSchema) provider(ctx context.Context, llm *core.Llm, args struct{}) (string, error) {
	return string(llm.Endpoint.Provider), nil
}

func (s *llmSchema) lastReply(ctx context.Context, llm *core.Llm, args struct{}) (dagql.String, error) {
	reply, err := llm.LastReply(ctx, s.srv)
	if err != nil {
		return "", err
	}
	return dagql.NewString(reply), nil
}

func (s *llmSchema) withModel(ctx context.Context, llm *core.Llm, args struct {
	Model string
}) (*core.Llm, error) {
	return llm.WithModel(ctx, args.Model, s.srv)
}

func (s *llmSchema) withPrompt(ctx context.Context, llm *core.Llm, args struct {
	Prompt string
}) (*core.Llm, error) {
	return llm.WithPrompt(ctx, args.Prompt, s.srv)
}

func (s *llmSchema) setString(ctx context.Context, llm *core.Llm, args struct {
	Name  dagql.String
	Value dagql.String
}) (*core.Llm, error) {
	return llm.Set(ctx, args.Name.String(), args.Value)
}

func (s *llmSchema) withPromptFile(ctx context.Context, llm *core.Llm, args struct {
	File core.FileID
}) (*core.Llm, error) {
	file, err := args.File.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return llm.WithPromptFile(ctx, file.Self, s.srv)
}

func (s *llmSchema) loop(ctx context.Context, llm *core.Llm, args struct{}) (*core.Llm, error) {
	return llm.Sync(ctx, s.srv)
}

func (s *llmSchema) llm(ctx context.Context, parent *core.Query, args struct {
	Model       dagql.Optional[dagql.String]
	MaxAPICalls dagql.Optional[dagql.Int] `name:"maxAPICalls"`
}) (*core.Llm, error) {
	var model string
	if args.Model.Valid {
		model = args.Model.Value.String()
	}
	var maxAPICalls int
	if args.MaxAPICalls.Valid {
		maxAPICalls = args.MaxAPICalls.Value.Int()
	}
	return core.NewLlm(ctx, parent, s.srv, model, maxAPICalls)
}

func (s *llmSchema) history(ctx context.Context, llm *core.Llm, _ struct{}) (dagql.Array[dagql.String], error) {
	history, err := llm.History(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return dagql.NewStringArray(history...), nil
}

func (s *llmSchema) tools(ctx context.Context, llm *core.Llm, _ struct{}) (dagql.String, error) {
	doc, err := llm.ToolsDoc(ctx, s.srv)
	return dagql.NewString(doc), err
}
