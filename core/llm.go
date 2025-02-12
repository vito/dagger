package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"dagger.io/dagger/telemetry"
	"github.com/dagger/dagger/core/bbi"
	_ "github.com/dagger/dagger/core/bbi/empty"
	_ "github.com/dagger/dagger/core/bbi/flat"
	"github.com/dagger/dagger/dagql"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/vektah/gqlparser/v2/ast"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

// An instance of a LLM (large language model), with its state and tool calling environment
type Llm struct {
	Query *Query

	Config *LlmConfig

	// History of messages
	// FIXME: rename to 'messages'
	history []openai.ChatCompletionMessageParamUnion
	// History of tool calls and their result
	calls      map[string]string
	promptVars []string

	// LLM state
	// Can hold typed variables for all the types available in the schema
	// This state is what gets extended by our graphql middleware
	// FIXME: Agent.ref moves here
	// FIXME: Agent.self moves here
	// FIXME: Agent.selfType moves here
	// FIXME: Agent.Self moves here
	// state map[string]dagql.Typed
	state dagql.Typed
}

type LlmConfig struct {
	Model string
	Key   string
	Host  string
	Path  string
}

// FIXME: engine-wide global config
// this is a workaround to enable modules to "just work" without bringing their own config
var globalLlmConfig *LlmConfig

func loadGlobalLlmConfig(ctx context.Context, srv *dagql.Server) (*LlmConfig, error) {
	loadSecret := func(uri string) (string, error) {
		var result string
		if u, err := url.Parse(uri); err != nil || u.Scheme == "" {
			result = uri
		} else if err := srv.Select(ctx, srv.Root(), &result,
			dagql.Selector{
				Field: "secret",
				Args: []dagql.NamedInput{
					{
						Name:  "uri",
						Value: dagql.NewString(uri),
					},
				},
			},
			dagql.Selector{
				Field: "plaintext",
			},
		); err != nil {
			return "", err
		}
		return result, nil
	}
	if globalLlmConfig != nil {
		return globalLlmConfig, nil
	}
	cfg := new(LlmConfig)
	envFile, err := loadSecret("file://.env")
	if err != nil {
		return nil, err
	}
	env, err := godotenv.Unmarshal(string(envFile))
	if err != nil {
		return nil, err
	}
	// Configure API key
	if keyConfig, ok := env["LLM_KEY"]; ok {
		key, err := loadSecret(keyConfig)
		if err != nil {
			return nil, err
		}
		cfg.Key = key
	}
	if host, ok := env["LLM_HOST"]; ok {
		cfg.Host = host
	}
	if path, ok := env["LLM_PATH"]; ok {
		cfg.Path = path
	}
	if model, ok := env["LLM_MODEL"]; ok {
		cfg.Model = model
	} else {
		cfg.Model = "gpt-4o"
	}
	if cfg.Key == "" && cfg.Host == "" {
		return nil, fmt.Errorf("error loading llm configuration: .env must set LLM_KEY or LLM_HOST")
	}
	globalLlmConfig = cfg
	return cfg, nil
}

func NewLlm(ctx context.Context, query *Query, srv *dagql.Server) (*Llm, error) {
	// FIXME: make the llm key/host/path/model configurable
	config, err := loadGlobalLlmConfig(ctx, srv)
	if err != nil {
		return nil, err
	}
	return &Llm{
		Query:  query,
		Config: config,
		calls:  make(map[string]string),
		// FIXME: support multiple variables in state
		//state:  make(map[string]dagql.Typed),
	}, nil
}

func (*Llm) Type() *ast.Type {
	return &ast.Type{
		NamedType: "Llm",
		NonNull:   true,
	}
}

func (llm *Llm) Clone() *Llm {
	cp := *llm
	cp.history = cloneSlice(cp.history)
	cp.promptVars = cloneSlice(cp.promptVars)
	cp.calls = cloneMap(cp.calls)
	// FIXME: support multiple variables in state
	// cp.state = cloneMap(cp.state)
	return &cp
}

// Generate a human-readable documentation of tools available to the model via BBI
func (llm *Llm) ToolsDoc(ctx context.Context, srv *dagql.Server) (string, error) {
	session, err := llm.BBI(srv)
	if err != nil {
		return "", err
	}
	var result string
	for _, tool := range session.Tools() {
		schema, err := json.MarshalIndent(tool.Schema, "", "  ")
		if err != nil {
			return "", err
		}
		result = fmt.Sprintf("%s## %s\n\n%s\n\n%s\n\n", result, tool.Name, tool.Description, string(schema))
	}
	return result, nil
}

// A convenience function to ask the model a question directly, and get an answer
// The state of the agent is not changed.
func (llm *Llm) Ask(ctx context.Context, question string, srv *dagql.Server) (string, error) {
	llm, err := llm.WithPrompt(ctx, question, false, srv)
	if err != nil {
		return "", err
	}
	return llm.LastReply()
}

// Append a user message (prompt) to the message history
func (llm *Llm) WithPrompt(ctx context.Context,
	// The prompt message.
	prompt string,
	// Append the prompt to the context, but don't sync with the LLM endpoint
	lazy bool,
	srv *dagql.Server,
) (*Llm, error) {
	vars := llm.promptVars
	if len(vars) > 0 {
		prompt = os.Expand(prompt, func(key string) string {
			// Iterate through vars array taking elements in pairs, looking
			// for a key that matches the template variable being expanded
			for i := 0; i < len(vars)-1; i += 2 {
				if vars[i] == key {
					return vars[i+1]
				}
			}
			// If vars array has odd length and the last key has no value,
			// return empty string when that key is looked up
			if len(vars)%2 == 1 && vars[len(vars)-1] == key {
				return ""
			}
			return key
		})
	}
	llm = llm.Clone()
	func() {
		ctx, span := Tracer(ctx).Start(ctx, "LLM prompt", telemetry.Reveal(), trace.WithAttributes(
			attribute.String("dagger.io/ui.actor", "🧑"),
			attribute.String("dagger.io/ui.message", "sent"),
		))
		defer span.End()
		stdio := telemetry.SpanStdio(ctx, InstrumentationLibrary,
			log.String("dagger.io/content.type", "text/markdown"))
		defer stdio.Close()
		fmt.Fprint(stdio.Stdout, prompt)
	}()
	llm.history = append(llm.history, openai.UserMessage(prompt))
	if lazy {
		return llm, nil
	}
	return llm.Sync(ctx, 0, srv)
}

// WithPromptFile is like WithPrompt but reads the prompt from a file
func (llm *Llm) WithPromptFile(ctx context.Context, file *File, lazy bool, srv *dagql.Server) (*Llm, error) {
	contents, err := file.Contents(ctx)
	if err != nil {
		return nil, err
	}
	return llm.WithPrompt(ctx, string(contents), lazy, srv)
}

func (llm *Llm) WithPromptVar(name, value string) *Llm {
	llm = llm.Clone()
	llm.promptVars = append(llm.promptVars, name, value)
	return llm
}

// Append a system prompt message to the history
func (llm *Llm) WithSystemPrompt(prompt string) *Llm {
	llm = llm.Clone()
	llm.history = append(llm.history, openai.SystemMessage(prompt))
	return llm
}

// Return the last message sent by the agent
func (llm *Llm) LastReply() (string, error) {
	messages, err := llm.messages()
	if err != nil {
		return "", err
	}
	var reply string = "(no reply)"
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		txt, err := msg.Text()
		if err != nil {
			return "", err
		}
		if len(txt) == 0 {
			continue
		}
		reply = txt
	}
	return reply, nil
}

// Start a new BBI (Brain-Body Interface) session.
// BBI allows a LLM to consume the Dagger API via tool calls
func (llm *Llm) BBI(srv *dagql.Server) (bbi.Session, error) {
	var target dagql.Object
	if llm.state != nil {
		target = llm.state.(dagql.Object)
	}
	return bbi.NewSession("flat", target, srv)
}

func (llm *Llm) Sync(
	ctx context.Context,
	maxLoops int,
	srv *dagql.Server,
) (*Llm, error) {
	llm = llm.Clone()
	// Start a new BBI session
	session, err := llm.BBI(srv)
	if err != nil {
		return nil, err
	}
	for i := 0; maxLoops == 0 || i < maxLoops; i++ {
		tools := session.Tools()
		res, err := llm.sendQuery(ctx, tools)
		if err != nil {
			return nil, err
		}
		reply := res.Choices[0].Message
		// Add the model reply to the history
		llm.history = append(llm.history, reply)
		// Handle tool calls
		calls := res.Choices[0].Message.ToolCalls
		if len(calls) == 0 {
			break
		}
		for _, toolCall := range calls {
			for _, tool := range tools {
				if tool.Name == toolCall.Function.Name {
					var args map[string]any
					decoder := json.NewDecoder(strings.NewReader(toolCall.Function.Arguments))
					decoder.UseNumber()
					if err := decoder.Decode(&args); err != nil {
						return llm, fmt.Errorf("failed to unmarshal arguments: %w", err)
					}
					result := func() string {
						ctx, span := Tracer(ctx).Start(ctx,
							fmt.Sprintf("🤖 💻 %s", toolCall.Function.Name),
							telemetry.Passthrough(),
							telemetry.Reveal())
						defer span.End()
						result, err := tool.Call(ctx, args)
						if err != nil {
							// If the BBI driver itself returned an error,
							// send that error to the model
							span.SetStatus(codes.Error, err.Error())
							return fmt.Sprintf("error calling tool %q: %s", tool.Name, err.Error())
						}
						stdio := telemetry.SpanStdio(ctx, InstrumentationLibrary)
						defer stdio.Close()
						switch v := result.(type) {
						case string:
							fmt.Fprint(stdio.Stdout, v)
							return v
						default:
							jsonBytes, err := json.Marshal(v)
							if err != nil {
								span.SetStatus(codes.Error, err.Error())
								return fmt.Sprintf("error processing tool result: %s", err.Error())
							}
							fmt.Fprint(stdio.Stdout, string(jsonBytes))
							return string(jsonBytes)
						}
					}()
					func() {
						llm.calls[toolCall.ID] = result
						llm.history = append(llm.history, openai.ToolMessage(toolCall.ID, result))
					}()
				}
			}
		}
	}
	llm.state = session.Self()
	return llm, nil
}

func (llm *Llm) History() ([]string, error) {
	messages, err := llm.messages()
	if err != nil {
		return nil, err
	}
	var history []string
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			txt, err := msg.Text()
			if err != nil {
				return nil, err
			}
			history = append(history, "🧑 💬"+txt)
		case "assistant":
			txt, err := msg.Text()
			if err != nil {
				return nil, err
			}
			if len(txt) > 0 {
				history = append(history, "🤖 💬"+txt)
			}
			for _, call := range msg.ToolCalls {
				history = append(history, fmt.Sprintf("🤖 💻 %s(%s)", call.Function.Name, call.Function.Arguments))
				if result, ok := llm.calls[call.ID]; ok {
					history = append(history, fmt.Sprintf("💻 %s", result))
				}
			}
		}
	}
	return history, nil
}

func (llm *Llm) messages() ([]openAIMessage, error) {
	// FIXME: ugly hack
	data, err := json.Marshal(llm.history)
	if err != nil {
		return nil, err
	}
	var messages []openAIMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (llm *Llm) WithState(ctx context.Context, objId dagql.IDType, srv *dagql.Server) (*Llm, error) {
	obj, err := srv.Load(ctx, objId.ID())
	if err != nil {
		return nil, err
	}
	llm = llm.Clone()
	llm.state = obj
	return llm, nil
}

func (llm *Llm) State(ctx context.Context) (dagql.Typed, error) {
	return llm.state, nil
}

func (llm *Llm) sendQuery(ctx context.Context, tools []bbi.Tool) (res *openai.ChatCompletion, rerr error) {
	params := openai.ChatCompletionNewParams{
		Seed:     openai.Int(0),
		Model:    openai.F(openai.ChatModel(llm.Config.Model)),
		Messages: openai.F(llm.history),
	}
	if len(tools) > 0 {
		var toolParams []openai.ChatCompletionToolParam
		for _, tool := range tools {
			toolParams = append(toolParams, openai.ChatCompletionToolParam{
				Type: openai.F(openai.ChatCompletionToolTypeFunction),
				Function: openai.F(openai.FunctionDefinitionParam{
					Name:        openai.String(tool.Name),
					Description: openai.String(tool.Description),
					Parameters:  openai.F(openai.FunctionParameters(tool.Schema)),
				}),
			})
		}
		params.Tools = openai.F(toolParams)
	}

	stream := llm.client().Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	var logsW io.Writer
	acc := new(openai.ChatCompletionAccumulator)
	for stream.Next() {
		if stream.Err() != nil {
			return nil, stream.Err()
		}

		res := stream.Current()
		acc.AddChunk(res)

		if content := res.Choices[0].Delta.Content; content != "" {
			if logsW == nil {
				// only show a message if we actually get a text response back
				// (as opposed to tool calls)
				ctx, span := Tracer(ctx).Start(ctx, "LLM response", telemetry.Reveal(), trace.WithAttributes(
					attribute.String("dagger.io/ui.actor", "🤖"),
					attribute.String("dagger.io/ui.message", "received"),
				))
				defer telemetry.End(span, func() error { return rerr })

				stdio := telemetry.SpanStdio(ctx, "",
					log.String("dagger.io/content.type", "text/markdown"))

				logsW = stdio.Stdout
			}

			fmt.Fprint(logsW, content)
		}
	}

	return &acc.ChatCompletion, nil
}

// Create a new openai client
func (llm *Llm) client() *openai.Client {
	var opts []option.RequestOption
	opts = append(opts, option.WithHeader("Content-Type", "application/json"))
	if llm.Config.Key != "" {
		opts = append(opts, option.WithAPIKey(llm.Config.Key))
	}
	if llm.Config.Host != "" || llm.Config.Path != "" {
		var base url.URL
		base.Scheme = "https"
		base.Host = llm.Config.Host
		base.Path = llm.Config.Path
		opts = append(opts, option.WithBaseURL(base.String()))
	}
	return openai.NewClient(opts...)
}

type openAIMessage struct {
	Role       string      `json:"role", required`
	Content    interface{} `json:"content", required`
	ToolCallID string      `json:"tool_call_id"`
	ToolCalls  []struct {
		// The ID of the tool call.
		ID string `json:"id"`
		// The function that the model called.
		Function struct {
			Arguments string `json:"arguments"`
			// The name of the function to call.
			Name string `json:"name"`
		} `json:"function"`
		// The type of the tool. Currently, only `function` is supported.
		Type openai.ChatCompletionMessageToolCallType `json:"type"`
	} `json:"tool_calls"`
}

func (msg openAIMessage) Text() (string, error) {
	contentJson, err := json.Marshal(msg.Content)
	if err != nil {
		return "", err
	}
	switch msg.Role {
	case "user", "tool":
		var content []struct {
			Text string `json:"text", required`
		}
		if err := json.Unmarshal(contentJson, &content); err != nil {
			return "", fmt.Errorf("malformatted user or tool message: %s", err.Error())
		}
		if len(content) == 0 {
			return "", nil
		}
		return content[0].Text, nil
	case "assistant":
		var content string
		if err := json.Unmarshal(contentJson, &content); err != nil {
			return "", fmt.Errorf("malformatted assistant message: %#v", content)
		}
		return content, nil
	}
	return "", fmt.Errorf("unsupported message role: %s", msg.Role)
}

type LlmMiddleware struct {
	Server *dagql.Server
}

// We don't expose these types to modules SDK codegen, but
// we still want their graphql schemas to be available for
// internal usage. So we use this list to scrub them from
// the introspection JSON that module SDKs use for codegen.
var TypesHiddenFromModuleSDKs = []dagql.Typed{
	&Host{},

	&Engine{},
	&EngineCache{},
	&EngineCacheEntry{},
	&EngineCacheEntrySet{},
}

func (s LlmMiddleware) extendLlmType(targetType dagql.ObjectType) error {
	llmType, ok := s.Server.ObjectType(new(Llm).Type().Name())
	if !ok {
		return fmt.Errorf("failed to lookup llm type")
	}
	idType, ok := targetType.IDType()
	if !ok {
		return fmt.Errorf("failed to lookup ID type for %T", targetType)
	}
	typename := targetType.TypeName()
	// Install with<targetType>()
	llmType.Extend(
		dagql.FieldSpec{
			Name:        "with" + typename,
			Description: fmt.Sprintf("Set the llm state to a %s", typename),
			Type:        llmType.Typed(),
			Args: dagql.InputSpecs{
				{
					Name:        "value",
					Description: fmt.Sprintf("The value of the %s to save", typename),
					Type:        idType,
				},
			},
		},
		func(ctx context.Context, self dagql.Object, args map[string]dagql.Input) (dagql.Typed, error) {
			llm := self.(dagql.Instance[*Llm]).Self
			id := args["value"].(dagql.IDType)
			return llm.WithState(ctx, id, s.Server)
		},
		nil,
	)
	// Install <targetType>()
	llmType.Extend(
		dagql.FieldSpec{
			Name:        typename,
			Description: fmt.Sprintf("Retrieve the llm state as a %s", typename),
			Type:        targetType.Typed(),
		},
		func(ctx context.Context, self dagql.Object, args map[string]dagql.Input) (dagql.Typed, error) {
			llm := self.(dagql.Instance[*Llm]).Self
			return llm.State(ctx)
		},
		nil,
	)
	return nil
}

func (s LlmMiddleware) InstallObject(targetType dagql.ObjectType, install func(dagql.ObjectType)) {
	install(targetType)
	typename := targetType.TypeName()
	if strings.HasPrefix(typename, "_") {
		return
	}

	// don't extend LLM for types that we hide from modules, lest the codegen yield a
	// WithEngine(*Engine) that refers to an unknown *Engine type.
	//
	// FIXME: in principle LLM should be able to refer to these types, so this should
	// probably be moved to codegen somehow, i.e. if a field refers to a type that is
	// hidden, don't codegen the field.
	for _, hiddenType := range TypesHiddenFromModuleSDKs {
		if hiddenType.Type().Name() == typename {
			return
		}
	}

	if err := s.extendLlmType(targetType); err != nil {
		panic(err)
	}
}

func (s LlmMiddleware) ModuleWithObject(ctx context.Context, mod *Module, targetTypedef *TypeDef) (*Module, error) {
	// Install the target type
	mod, err := mod.WithObject(ctx, targetTypedef)
	if err != nil {
		return nil, err
	}
	typename := targetTypedef.Type().Name()
	targetType, ok := s.Server.ObjectType(typename)
	if !ok {
		return nil, fmt.Errorf("can't retrieve object type %s", typename)
	}
	if err := s.extendLlmType(targetType); err != nil {
		return nil, err
	}
	return mod, nil
}
