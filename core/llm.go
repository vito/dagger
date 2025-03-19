package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"dagger.io/dagger/telemetry"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/dagger/dagger/dagql"
	"github.com/dagger/dagger/engine"
	"github.com/dagger/dagger/engine/client/secretprovider"
	"github.com/iancoleman/strcase"
	"github.com/joho/godotenv"
	"github.com/vektah/gqlparser/v2/ast"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	strcase.ConfigureAcronym("LLM", "LLM")
}

// TODO: is this the right place for this? is there an argument for or against
// it being here, and/or for it being overrideable?
const defaultSystemPrompt = `You are an AI assistant that interacts with an immutable GraphQL API by calling tools that return new state objects.
Instead of modifying objects in place, each tool call produces a new state, which updates the available set of tools.
Your environment changes dynamically as you navigate through different states.

State is preserved, and previous states can be accessed by saving them as variables using the _save tool.
To explore effectively, prioritize discovering new states over efficiency.
You may need to make exploratory tool calls to understand the available actions.

Your goal is to autonomously interact with the API, selecting and chaining tools to achieve tasks.
When completing a task, save the final result using the _save tool.`

// An instance of a LLM (large language model), with its state and tool calling environment
type LLM struct {
	Query *Query

	maxAPICalls int
	apiCalls    int
	Endpoint    *LLMEndpoint

	// If true: has un-synced state
	dirty bool
	// History of messages
	messages []ModelMessage
	// History of tool calls and their result
	calls      map[string]string
	promptVars []string

	env *LLMEnv
}

type LLMEndpoint struct {
	Model    string
	BaseURL  string
	Key      string
	Provider LLMProvider
	Client   LLMClient
}

type LLMProvider string

// LLMClient interface defines the methods that each provider must implement
type LLMClient interface {
	SendQuery(ctx context.Context, history []ModelMessage, tools []LLMTool) (*LLMResponse, error)
}

type LLMResponse struct {
	Content    string
	ToolCalls  []ToolCall
	TokenUsage TokenUsage
}

type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

// ModelMessage represents a generic message in the LLM conversation
type ModelMessage struct {
	Role        string     `json:"role"`
	Content     string     `json:"content"`
	ToolCalls   []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID  string     `json:"tool_call_id,omitempty"`
	ToolErrored bool       `json:"tool_errored,omitempty"`
	TokenUsage  TokenUsage `json:"token_usage,omitempty"`
}

type ToolCall struct {
	ID       string   `json:"id"`
	Function FuncCall `json:"function"`
	Type     string   `json:"type"`
}

type FuncCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

const (
	OpenAI    LLMProvider = "openai"
	Anthropic LLMProvider = "anthropic"
	Google    LLMProvider = "google"
	Meta      LLMProvider = "meta"
	Mistral   LLMProvider = "mistral"
	DeepSeek  LLMProvider = "deepseek"
	Other     LLMProvider = "other"
)

// A LLM routing configuration
type LLMRouter struct {
	AnthropicAPIKey  string
	AnthropicBaseURL string
	AnthropicModel   string

	OpenAIAPIKey       string
	OpenAIAzureVersion string
	OpenAIBaseURL      string
	OpenAIModel        string

	GeminiAPIKey  string
	GeminiBaseURL string
	GeminiModel   string
}

func (r *LLMRouter) isAnthropicModel(model string) bool {
	return strings.HasPrefix(model, "claude-") || strings.HasPrefix(model, "anthropic/")
}

func (r *LLMRouter) isOpenAIModel(model string) bool {
	return strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "openai/")
}

func (r *LLMRouter) isGoogleModel(model string) bool {
	return strings.HasPrefix(model, "gemini-") || strings.HasPrefix(model, "google/")
}

func (r *LLMRouter) isMistralModel(model string) bool {
	return strings.HasPrefix(model, "mistral-") || strings.HasPrefix(model, "mistral/")
}

func (r *LLMRouter) isReplay(model string) bool {
	return strings.HasPrefix(model, "replay-") || strings.HasPrefix(model, "replay/")
}

func (r *LLMRouter) getReplay(model string) (messages []ModelMessage, _ error) {
	model, ok := strings.CutPrefix(model, "replay-")
	if !ok {
		model, ok = strings.CutPrefix(model, "replay/")
		if !ok {
			return nil, fmt.Errorf("model %q is not replayable", model)
		}
	}

	result, err := base64.StdEncoding.DecodeString(model)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(result, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *LLMRouter) routeAnthropicModel() *LLMEndpoint {
	endpoint := &LLMEndpoint{
		BaseURL:  r.AnthropicBaseURL,
		Key:      r.AnthropicAPIKey,
		Provider: Anthropic,
	}
	endpoint.Client = newAnthropicClient(endpoint)

	return endpoint
}

func (r *LLMRouter) routeOpenAIModel() *LLMEndpoint {
	endpoint := &LLMEndpoint{
		BaseURL:  r.OpenAIBaseURL,
		Key:      r.OpenAIAPIKey,
		Provider: OpenAI,
	}
	endpoint.Client = newOpenAIClient(endpoint, r.OpenAIAzureVersion)

	return endpoint
}

func (r *LLMRouter) routeGoogleModel() (*LLMEndpoint, error) {
	endpoint := &LLMEndpoint{
		BaseURL:  r.GeminiBaseURL,
		Key:      r.GeminiAPIKey,
		Provider: Google,
	}
	client, err := newGenaiClient(endpoint)
	if err != nil {
		return nil, err
	}
	endpoint.Client = client

	return endpoint, nil
}

func (r *LLMRouter) routeOtherModel() *LLMEndpoint {
	// default to openAI compat from other providers
	endpoint := &LLMEndpoint{
		BaseURL:  r.OpenAIBaseURL,
		Key:      r.OpenAIAPIKey,
		Provider: Other,
	}
	endpoint.Client = newOpenAIClient(endpoint, r.OpenAIAzureVersion)

	return endpoint
}

func (r *LLMRouter) routeReplayModel(model string) (*LLMEndpoint, error) {
	replay, err := r.getReplay(model)
	if err != nil {
		return nil, err
	}
	endpoint := &LLMEndpoint{}
	endpoint.Client = newHistoryReplay(replay)
	return endpoint, nil
}

// Return a default model, if configured
func (r *LLMRouter) DefaultModel() string {
	for _, model := range []string{r.OpenAIModel, r.AnthropicModel, r.GeminiModel} {
		if model != "" {
			return model
		}
	}
	if r.OpenAIAPIKey != "" {
		return "gpt-4o"
	}
	if r.AnthropicAPIKey != "" {
		return anthropic.ModelClaude3_5SonnetLatest
	}
	if r.OpenAIBaseURL != "" {
		return "llama-3.2"
	}
	if r.GeminiAPIKey != "" {
		return "gemini-2.0-flash"
	}
	return ""
}

// Return an endpoint for the requested model
// If the model name is not set, a default will be selected.
func (r *LLMRouter) Route(model string) (*LLMEndpoint, error) {
	if model == "" {
		model = r.DefaultModel()
	}
	var endpoint *LLMEndpoint
	var err error
	switch {
	case r.isAnthropicModel(model):
		endpoint = r.routeAnthropicModel()
	case r.isOpenAIModel(model):
		endpoint = r.routeOpenAIModel()
	case r.isGoogleModel(model):
		endpoint, err = r.routeGoogleModel()
		if err != nil {
			return nil, err
		}
	case r.isMistralModel(model):
		return nil, fmt.Errorf("mistral models are not yet supported")
	case r.isReplay(model):
		endpoint, err = r.routeReplayModel(model)
		if err != nil {
			return nil, err
		}
	default:
		endpoint = r.routeOtherModel()
	}
	endpoint.Model = model
	return endpoint, nil
}

func (r *LLMRouter) LoadConfig(ctx context.Context, getenv func(context.Context, string) (string, error)) error {
	if getenv == nil {
		getenv = func(ctx context.Context, key string) (string, error) {
			return os.Getenv(key), nil
		}
	}
	var err error

	r.AnthropicAPIKey, err = getenv(ctx, "ANTHROPIC_API_KEY")
	if err != nil {
		return err
	}
	r.AnthropicBaseURL, err = getenv(ctx, "ANTHROPIC_BASE_URL")
	if err != nil {
		return err
	}
	r.AnthropicModel, err = getenv(ctx, "ANTHROPIC_MODEL")
	if err != nil {
		return err
	}

	r.OpenAIAPIKey, err = getenv(ctx, "OPENAI_API_KEY")
	if err != nil {
		return err
	}
	r.OpenAIAzureVersion, err = getenv(ctx, "OPENAI_AZURE_VERSION")
	if err != nil {
		return err
	}
	r.OpenAIBaseURL, err = getenv(ctx, "OPENAI_BASE_URL")
	if err != nil {
		return err
	}
	r.OpenAIModel, err = getenv(ctx, "OPENAI_MODEL")
	if err != nil {
		return err
	}

	r.GeminiAPIKey, err = getenv(ctx, "GEMINI_API_KEY")
	if err != nil {
		return err
	}
	r.GeminiBaseURL, err = getenv(ctx, "GEMINI_BASE_URL")
	if err != nil {
		return err
	}
	r.GeminiModel, err = getenv(ctx, "GEMINI_MODEL")
	if err != nil {
		return err
	}

	return nil
}

func NewLLMRouter(ctx context.Context, srv *dagql.Server) (_ *LLMRouter, rerr error) {
	router := new(LLMRouter)
	// Get the secret plaintext, from either a URI (provider lookup) or a plaintext (no-op)
	loadSecret := func(ctx context.Context, uriOrPlaintext string) (string, error) {
		if _, _, err := secretprovider.ResolverForID(uriOrPlaintext); err == nil {
			var result string
			// If it's a valid secret reference:
			if err := srv.Select(ctx, srv.Root(), &result,
				dagql.Selector{
					Field: "secret",
					Args:  []dagql.NamedInput{{Name: "uri", Value: dagql.NewString(uriOrPlaintext)}},
				},
				dagql.Selector{
					Field: "plaintext",
				},
			); err != nil {
				return "", err
			}
			return result, nil
		}
		// If it's a regular plaintext:
		return uriOrPlaintext, nil
	}
	ctx, span := Tracer(ctx).Start(ctx, "load LLM router config", telemetry.Internal(), telemetry.Encapsulate())
	defer telemetry.End(span, func() error { return rerr })
	env := make(map[string]string)
	// Load .env from current directory, if it exists
	if envFile, err := loadSecret(ctx, "file://.env"); err == nil {
		if e, err := godotenv.Unmarshal(envFile); err == nil {
			env = e
		}
	}
	err := router.LoadConfig(ctx, func(ctx context.Context, k string) (string, error) {
		// First lookup in the .env file
		if v, ok := env[k]; ok {
			return loadSecret(ctx, v)
		}
		// Second: lookup in client env directly
		if v, err := loadSecret(ctx, "env://"+k); err == nil {
			// Allow the env var itself to be a secret reference
			return loadSecret(ctx, v)
		}
		return "", nil
	})
	return router, err
}

func NewLLM(ctx context.Context, query *Query, model string, maxAPICalls int) (*LLM, error) {
	router, err := loadLLMRouter(ctx, query)
	if err != nil {
		return nil, err
	}

	if model == "" {
		model = router.DefaultModel()
	}
	endpoint, err := router.Route(model)
	if err != nil {
		return nil, err
	}
	if endpoint.Model == "" {
		return nil, fmt.Errorf("no valid LLM endpoint configuration")
	}
	return &LLM{
		Query:       query,
		Endpoint:    endpoint,
		maxAPICalls: maxAPICalls,
		calls:       make(map[string]string),
		env:         NewLLMEnv(),
	}, nil
}

// loadLLMRouter creates an LLM router that routes to the root client
func loadLLMRouter(ctx context.Context, query *Query) (*LLMRouter, error) {
	parentClient, err := query.NonModuleParentClientMetadata(ctx)
	if err != nil {
		return nil, err
	}
	ctx = engine.ContextWithClientMetadata(ctx, parentClient)
	mainSrv, err := query.Server.Server(ctx)
	if err != nil {
		return nil, err
	}
	return NewLLMRouter(ctx, mainSrv)
}

func (*LLM) Type() *ast.Type {
	return &ast.Type{
		NamedType: "LLM",
		NonNull:   true,
	}
}

func (llm *LLM) Clone() *LLM {
	cp := *llm
	cp.messages = cloneSlice(cp.messages)
	cp.calls = cloneMap(cp.calls)
	cp.promptVars = cloneSlice(cp.promptVars)
	cp.env = cp.env.Clone()
	return &cp
}

// Generate a human-readable documentation of tools available to the model
func (llm *LLM) ToolsDoc(ctx context.Context, srv *dagql.Server) (string, error) {
	var result string
	for _, tool := range llm.env.Tools(srv) {
		schema, err := json.MarshalIndent(tool.Schema, "", "  ")
		if err != nil {
			return "", err
		}
		result = fmt.Sprintf("%s## %s\n\n%s\n\n%s\n\n", result, tool.Name, tool.Description, string(schema))
	}
	return result, nil
}

func (llm *LLM) WithModel(ctx context.Context, model string, srv *dagql.Server) (*LLM, error) {
	llm = llm.Clone()
	router, err := NewLLMRouter(ctx, srv)
	if err != nil {
		return nil, err
	}
	endpoint, err := router.Route(model)
	if err != nil {
		return nil, err
	}
	if endpoint.Model == "" {
		return nil, fmt.Errorf("no valid LLM endpoint configuration")
	}
	llm.Endpoint = endpoint
	return llm, nil
}

// Append a user message (prompt) to the message history
func (llm *LLM) WithPrompt(
	ctx context.Context,
	// The prompt message.
	prompt string,
	srv *dagql.Server,
) (*LLM, error) {
	if len(llm.env.vars) > 0 {
		prompt = os.Expand(prompt, func(key string) string {
			val, err := llm.env.Get(key)
			if err != nil {
				return ""
			}
			return fmt.Sprintf("%s", val)
		})
	}
	llm = llm.Clone()
	func() {
		ctx, span := Tracer(ctx).Start(ctx, "LLM prompt", telemetry.Reveal(), trace.WithAttributes(
			attribute.String(telemetry.UIActorEmojiAttr, "🧑"),
			attribute.String(telemetry.UIMessageAttr, "sent"),
		))
		defer span.End()
		stdio := telemetry.SpanStdio(ctx, InstrumentationLibrary,
			log.String(telemetry.ContentTypeAttr, "text/markdown"))
		defer stdio.Close()
		fmt.Fprint(stdio.Stdout, prompt)
	}()
	llm.messages = append(llm.messages, ModelMessage{
		Role:    "user",
		Content: prompt,
	})
	llm.dirty = true
	return llm, nil
}

// WithPromptFile is like WithPrompt but reads the prompt from a file
func (llm *LLM) WithPromptFile(ctx context.Context, file *File, srv *dagql.Server) (*LLM, error) {
	contents, err := file.Contents(ctx)
	if err != nil {
		return nil, err
	}
	return llm.WithPrompt(ctx, string(contents), srv)
}

func (llm *LLM) WithPromptVar(name, value string) *LLM {
	llm = llm.Clone()
	llm.promptVars = append(llm.promptVars, name, value)
	return llm
}

// Append a system prompt message to the history
func (llm *LLM) WithSystemPrompt(prompt string) *LLM {
	llm = llm.Clone()
	llm.messages = append(llm.messages, ModelMessage{
		Role:    "system",
		Content: prompt,
	})
	llm.dirty = true
	return llm
}

// Return the last message sent by the agent
func (llm *LLM) LastReply(ctx context.Context, dag *dagql.Server) (string, error) {
	llm, err := llm.Sync(ctx, dag)
	if err != nil {
		return "", err
	}
	var reply string = "(no reply)"
	for _, msg := range llm.messages {
		if msg.Role != "assistant" {
			continue
		}
		txt := msg.Content
		if len(txt) == 0 {
			continue
		}
		reply = txt
	}
	return reply, nil
}

// send the context to the LLM endpoint, process replies and tool calls; continue in a loop
// Synchronize LLM state:
// 1. Send context to LLM endpoint
// 2. Process replies and tool calls
// 3. Continue in a loop until no tool calls, or caps are reached
func (llm *LLM) Sync(ctx context.Context, dag *dagql.Server) (*LLM, error) {
	if err := llm.allowed(ctx); err != nil {
		return nil, err
	}

	if !llm.dirty {
		return llm, nil
	}
	if len(llm.messages) == 0 {
		// dirty but no messages, possibly just a state change, nothing to do
		// until a prompt is given
		return llm, nil
	}
	llm = llm.Clone()
	for {
		if llm.maxAPICalls > 0 && llm.apiCalls >= llm.maxAPICalls {
			return nil, fmt.Errorf("reached API call limit: %d", llm.apiCalls)
		}
		llm.apiCalls++

		tools := llm.env.Tools(dag)
		res, err := llm.Endpoint.Client.SendQuery(ctx, llm.messages, tools)
		if err != nil {
			return nil, err
		}

		// Add the model reply to the history
		llm.messages = append(llm.messages, ModelMessage{
			Role:       "assistant",
			Content:    res.Content,
			ToolCalls:  res.ToolCalls,
			TokenUsage: res.TokenUsage,
		})
		// Handle tool calls
		// calls := res.Choices[0].Message.ToolCalls
		if len(res.ToolCalls) == 0 {
			break
		}
		for _, toolCall := range res.ToolCalls {
			for _, tool := range tools {
				if tool.Name == toolCall.Function.Name {
					result, isError := func() (string, bool) {
						result, err := tool.Call(ctx, toolCall.Function.Arguments)
						if err != nil {
							errResponse := err.Error()
							// propagate error values to the model
							var extErr dagql.ExtendedError
							if errors.As(err, &extErr) {
								var exts []string
								for k, v := range extErr.Extensions() {
									var ext strings.Builder
									fmt.Fprintf(&ext, "<%s>\n", k)

									switch v := v.(type) {
									case string:
										ext.WriteString(v)
									default:
										jsonBytes, err := json.Marshal(v)
										if err != nil {
											fmt.Fprintf(&ext, "error marshalling value: %s", err.Error())
										} else {
											ext.Write(jsonBytes)
										}
									}

									fmt.Fprintf(&ext, "\n</%s>", k)

									exts = append(exts, ext.String())
								}
								if len(exts) > 0 {
									sort.Strings(exts)
									errResponse += "\n\n" + strings.Join(exts, "\n\n")
								}
							}
							return errResponse, true
						}
						switch v := result.(type) {
						case string:
							// TODO: should we just JSON encode this too? what
							// is safer and/or better for the model?
							return v, false
						default:
							jsonBytes, err := json.Marshal(v)
							if err != nil {
								return fmt.Sprintf("error processing tool result: %s", err.Error()), true
							}
							return string(jsonBytes), false
						}
					}()
					func() {
						llm.calls[toolCall.ID] = result
						llm.messages = append(llm.messages, ModelMessage{
							Role:        "user", // Anthropic only allows tool calls in user messages
							Content:     result,
							ToolCallID:  toolCall.ID,
							ToolErrored: isError,
						})
					}()
				}
			}
		}
	}
	llm.dirty = false
	return llm, nil
}

func (llm *LLM) allowed(ctx context.Context) error {
	bk, err := llm.Query.Buildkit(ctx)
	if err != nil {
		return err
	}

	module, err := llm.Query.CurrentModule(ctx)
	if err != nil {
		// allow non-module calls
		if errors.Is(err, ErrNoCurrentModule) {
			return nil
		}
		return fmt.Errorf("failed to figure out module while deciding if llm is allowed: %w", err)
	}
	if module.Source.Self.Kind != ModuleSourceKindGit {
		return nil
	}

	return bk.AllowLLM(ctx, module.Source.Self.Git.CloneRef)
}

func (llm *LLM) History(ctx context.Context, dag *dagql.Server) ([]string, error) {
	llm, err := llm.Sync(ctx, dag)
	if err != nil {
		return nil, err
	}
	var history []string
	for _, msg := range llm.messages {
		switch msg.Role {
		case "user":
			history = append(history, "🧑 💬"+msg.Content)
		case "assistant":
			if len(msg.Content) > 0 {
				history = append(history, "🤖 💬"+msg.Content)
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

func (llm *LLM) HistoryJSON(ctx context.Context, dag *dagql.Server) (string, error) {
	llm, err := llm.Sync(ctx, dag)
	if err != nil {
		return "", err
	}
	result, err := json.MarshalIndent(llm.messages, "", "  ")
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func (llm *LLM) Set(ctx context.Context, dag *dagql.Server, key string, value dagql.Typed) (*LLM, error) {
	if id, ok := value.(dagql.IDType); ok {
		obj, err := dag.Load(ctx, id.ID())
		if err != nil {
			return nil, err
		}
		value = obj
	}
	llm = llm.Clone()
	llm.messages = append(llm.messages, ModelMessage{
		Role:    "user",
		Content: llm.env.Set(key, value),
	})
	llm.dirty = true
	return llm, nil
}

func (llm *LLM) Get(ctx context.Context, dag *dagql.Server, key string) (dagql.Typed, error) {
	llm, err := llm.Sync(ctx, dag)
	if err != nil {
		return nil, err
	}
	return llm.env.Get(key)
}

func (llm *LLM) With(ctx context.Context, dag *dagql.Server, value dagql.Typed) (*LLM, error) {
	if id, ok := value.(dagql.IDType); ok {
		obj, err := dag.Load(ctx, id.ID())
		if err != nil {
			return nil, err
		}
		value = obj
	}
	llm = llm.Clone()
	llm.env.With(value)
	llm.dirty = true
	return llm, nil
}

// A variable in the LLM environment
type LLMVariable struct {
	// The name of the variable
	Name string `field:"true"`
	// The type name of the variable's value
	TypeName string `field:"true"`
	// A hash of the variable's value, used to detect changes
	Hash string `field:"true"`
}

var _ dagql.Typed = (*LLMVariable)(nil)

func (v *LLMVariable) Type() *ast.Type {
	return &ast.Type{
		NamedType: "LLMVariable",
		NonNull:   true,
	}
}

func (llm *LLM) Variables(ctx context.Context, dag *dagql.Server) ([]*LLMVariable, error) {
	llm, err := llm.Sync(ctx, dag)
	if err != nil {
		return nil, err
	}
	vars := make([]*LLMVariable, 0, len(llm.env.vars))
	for k, v := range llm.env.vars {
		var hash string
		if obj, ok := dagql.UnwrapAs[dagql.Object](v); ok {
			hash = obj.ID().Digest().String()
		} else {
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			hash = dagql.HashFrom(string(jsonBytes)).String()
		}
		vars = append(vars, &LLMVariable{
			Name:     k,
			TypeName: v.Type().Name(),
			Hash:     hash,
		})
	}
	// NOTE: order matters! when a client is grabbing these values they'll be
	// "calling back" using IDs that embed index positions
	sort.Slice(vars, func(i, j int) bool {
		return vars[i].Name < vars[j].Name
	})
	return vars, nil
}

func (llm *LLM) CurrentType(ctx context.Context, dag *dagql.Server) (dagql.Nullable[dagql.String], error) {
	var res dagql.Nullable[dagql.String]
	llm, err := llm.Sync(ctx, dag)
	if err != nil {
		return res, err
	}
	if llm.env.Current() == nil {
		return res, nil
	}
	res.Value = dagql.String(llm.env.Current().Type().Name())
	res.Valid = true
	return res, nil
}

// FIXME: deprecated
func (llm *LLM) WithState(ctx context.Context, objID dagql.IDType, srv *dagql.Server) (*LLM, error) {
	obj, err := srv.Load(ctx, objID.ID())
	if err != nil {
		return nil, err
	}
	return llm.Set(ctx, srv, "default", obj)
}

// FIXME: deprecated
func (llm *LLM) State(ctx context.Context, dag *dagql.Server) (dagql.Typed, error) {
	return llm.Get(ctx, dag, "default")
}

type LLMHook struct {
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

func (s LLMHook) ExtendLLMType(targetType dagql.ObjectType) error {
	llmType, ok := s.Server.ObjectType(new(LLM).Type().Name())
	if !ok {
		return fmt.Errorf("failed to lookup llm type")
	}
	idType, ok := targetType.IDType()
	if !ok {
		return fmt.Errorf("failed to lookup ID type for %T", targetType)
	}
	typename := targetType.TypeName()
	// Install get<TargetType>()
	llmType.Extend(
		dagql.FieldSpec{
			Name:        "set" + typename,
			Description: fmt.Sprintf("Set a variable of type %s in the llm environment", typename),
			Type:        llmType.Typed(),
			Args: dagql.InputSpecs{
				{
					Name:        "name",
					Description: "The name of the variable",
					Type:        dagql.NewString(""),
				},
				{
					Name:        "value",
					Description: fmt.Sprintf("The %s value to assign to the variable", typename),
					Type:        idType,
				},
			},
		},
		func(ctx context.Context, self dagql.Object, args map[string]dagql.Input) (dagql.Typed, error) {
			llm := self.(dagql.Instance[*LLM]).Self
			name := args["name"].(dagql.String).String()
			value := args["value"].(dagql.Typed)
			return llm.Set(ctx, s.Server, name, value)
			// id := args["value"].(dagql.IDType)
		},
		dagql.CacheSpec{},
	)
	// Install get<targetType>()
	llmType.Extend(
		dagql.FieldSpec{
			Name:        "get" + typename,
			Description: fmt.Sprintf("Retrieve a variable in the llm environment, of type %s", typename),
			Type:        targetType.Typed(),
			Args: dagql.InputSpecs{{
				Name:        "name",
				Description: "The name of the variable",
				Type:        dagql.NewString(""),
			}},
		},
		func(ctx context.Context, self dagql.Object, args map[string]dagql.Input) (dagql.Typed, error) {
			llm := self.(dagql.Instance[*LLM]).Self
			name := args["name"].(dagql.String).String()
			val, err := llm.Get(ctx, s.Server, name)
			if err != nil {
				return nil, err
			}
			if val.Type().Name() != typename {
				return nil, fmt.Errorf("expected variable of type %s, got %s", typename, val.Type().Name())
			}
			return val, nil
		},
		dagql.CacheSpec{},
	)

	// BACKWARDS COMPATIBILITY:

	// Install with<TargetType>()
	llmType.Extend(
		dagql.FieldSpec{
			Name:        "with" + typename,
			Description: fmt.Sprintf("Set a variable of type %s in the llm environment", typename),
			Type:        llmType.Typed(),
			// DeprecatedReason: "use set<TargetType> instead",
			Args: dagql.InputSpecs{
				{
					Name:        "value",
					Description: fmt.Sprintf("The %s value to assign to the variable", typename),
					Type:        idType,
				},
			},
		},
		func(ctx context.Context, self dagql.Object, args map[string]dagql.Input) (dagql.Typed, error) {
			llm := self.(dagql.Instance[*LLM]).Self
			value := args["value"].(dagql.Typed)
			return llm.With(ctx, s.Server, value)
		},
		dagql.CacheSpec{},
	)
	// Install <targetType>()
	llmType.Extend(
		dagql.FieldSpec{
			Name:        gqlFieldName(typename),
			Description: fmt.Sprintf("Retrieve a the current value in the LLM environment, of type %s", typename),
			Type:        targetType.Typed(),
			// DeprecatedReason: "use get<TargetType> instead",
		},
		func(ctx context.Context, self dagql.Object, args map[string]dagql.Input) (dagql.Typed, error) {
			llm := self.(dagql.Instance[*LLM]).Self
			llm, err := llm.Sync(ctx, s.Server)
			if err != nil {
				return nil, err
			}
			val := llm.env.Current()
			if val == nil {
				return nil, fmt.Errorf("no value set for %s", typename)
			}
			if val.Type().Name() != typename {
				return nil, fmt.Errorf("expected variable of type %s, got %s", typename, val.Type().Name())
			}
			return val, nil
		},
		dagql.CacheSpec{},
	)
	return nil
}

func (s LLMHook) InstallObject(targetType dagql.ObjectType) {
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

	if err := s.ExtendLLMType(targetType); err != nil {
		panic(err)
	}
}

func (s LLMHook) ModuleWithObject(ctx context.Context, mod *Module, targetTypedef *TypeDef) (*Module, error) {
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
	if err := s.ExtendLLMType(targetType); err != nil {
		return nil, err
	}
	return mod, nil
}
