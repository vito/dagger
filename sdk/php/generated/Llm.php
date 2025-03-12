<?php

/**
 * This class has been generated by dagger-php-sdk. DO NOT EDIT.
 */

declare(strict_types=1);

namespace Dagger;

class Llm extends Client\AbstractObject implements Client\IdAble
{
    /**
     * Retrieve the llm state as a CacheVolume
     */
    public function cacheVolume(): CacheVolume
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('cacheVolume');
        return new \Dagger\CacheVolume($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a Container
     */
    public function container(): Container
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('container');
        return new \Dagger\Container($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a CurrentModule
     */
    public function currentModule(): CurrentModule
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('currentModule');
        return new \Dagger\CurrentModule($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a Directory
     */
    public function directory(): Directory
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('directory');
        return new \Dagger\Directory($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a EnumTypeDef
     */
    public function enumTypeDef(): EnumTypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('enumTypeDef');
        return new \Dagger\EnumTypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a EnumValueTypeDef
     */
    public function enumValueTypeDef(): EnumValueTypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('enumValueTypeDef');
        return new \Dagger\EnumValueTypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a Error
     */
    public function error(): Error
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('error');
        return new \Dagger\Error($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a ErrorValue
     */
    public function errorValue(): ErrorValue
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('errorValue');
        return new \Dagger\ErrorValue($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a FieldTypeDef
     */
    public function fieldTypeDef(): FieldTypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('fieldTypeDef');
        return new \Dagger\FieldTypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a File
     */
    public function file(): File
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('file');
        return new \Dagger\File($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a Function
     */
    public function function(): Function_
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('function');
        return new \Dagger\Function_($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a FunctionArg
     */
    public function functionArg(): FunctionArg
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('functionArg');
        return new \Dagger\FunctionArg($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a FunctionCall
     */
    public function functionCall(): FunctionCall
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('functionCall');
        return new \Dagger\FunctionCall($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a FunctionCallArgValue
     */
    public function functionCallArgValue(): FunctionCallArgValue
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('functionCallArgValue');
        return new \Dagger\FunctionCallArgValue($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a GeneratedCode
     */
    public function generatedCode(): GeneratedCode
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('generatedCode');
        return new \Dagger\GeneratedCode($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a GitRef
     */
    public function gitRef(): GitRef
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('gitRef');
        return new \Dagger\GitRef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a GitRepository
     */
    public function gitRepository(): GitRepository
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('gitRepository');
        return new \Dagger\GitRepository($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * return the llm message history
     */
    public function history(): array
    {
        $leafQueryBuilder = new \Dagger\Client\QueryBuilder('history');
        return (array)$this->queryLeaf($leafQueryBuilder, 'history');
    }

    /**
     * A unique identifier for this Llm.
     */
    public function id(): LlmId
    {
        $leafQueryBuilder = new \Dagger\Client\QueryBuilder('id');
        return new \Dagger\LlmId((string)$this->queryLeaf($leafQueryBuilder, 'id'));
    }

    /**
     * Retrieve the llm state as a InputTypeDef
     */
    public function inputTypeDef(): InputTypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('inputTypeDef');
        return new \Dagger\InputTypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a InterfaceTypeDef
     */
    public function interfaceTypeDef(): InterfaceTypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('interfaceTypeDef');
        return new \Dagger\InterfaceTypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * return the last llm reply from the history
     */
    public function lastReply(): string
    {
        $leafQueryBuilder = new \Dagger\Client\QueryBuilder('lastReply');
        return (string)$this->queryLeaf($leafQueryBuilder, 'lastReply');
    }

    /**
     * Retrieve the llm state as a ListTypeDef
     */
    public function listTypeDef(): ListTypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('listTypeDef');
        return new \Dagger\ListTypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a Llm
     */
    public function llm(): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('llm');
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * synchronize LLM state
     */
    public function loop(): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('loop');
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * return the model used by the llm
     */
    public function model(): string
    {
        $leafQueryBuilder = new \Dagger\Client\QueryBuilder('model');
        return (string)$this->queryLeaf($leafQueryBuilder, 'model');
    }

    /**
     * Retrieve the llm state as a Module
     */
    public function module(): Module
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('module');
        return new \Dagger\Module($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a ModuleConfigClient
     */
    public function moduleConfigClient(): ModuleConfigClient
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('moduleConfigClient');
        return new \Dagger\ModuleConfigClient($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a ModuleSource
     */
    public function moduleSource(): ModuleSource
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('moduleSource');
        return new \Dagger\ModuleSource($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a ObjectTypeDef
     */
    public function objectTypeDef(): ObjectTypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('objectTypeDef');
        return new \Dagger\ObjectTypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a PhpSdk
     */
    public function phpSdk(): PhpSdk
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('phpSdk');
        return new \Dagger\PhpSdk($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * return the provider used by the llm
     */
    public function provider(): string
    {
        $leafQueryBuilder = new \Dagger\Client\QueryBuilder('provider');
        return (string)$this->queryLeaf($leafQueryBuilder, 'provider');
    }

    /**
     * Retrieve the llm state as a ScalarTypeDef
     */
    public function scalarTypeDef(): ScalarTypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('scalarTypeDef');
        return new \Dagger\ScalarTypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a SDKConfig
     */
    public function sdkconfig(): SDKConfig
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('sdkconfig');
        return new \Dagger\SDKConfig($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a Secret
     */
    public function secret(): Secret
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('secret');
        return new \Dagger\Secret($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a Service
     */
    public function service(): Service
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('service');
        return new \Dagger\Service($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a Socket
     */
    public function socket(): Socket
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('socket');
        return new \Dagger\Socket($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Retrieve the llm state as a SourceMap
     */
    public function sourceMap(): SourceMap
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('sourceMap');
        return new \Dagger\SourceMap($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * synchronize LLM state
     */
    public function sync(): LlmId
    {
        $leafQueryBuilder = new \Dagger\Client\QueryBuilder('sync');
        return new \Dagger\LlmId((string)$this->queryLeaf($leafQueryBuilder, 'sync'));
    }

    /**
     * Retrieve the llm state as a Terminal
     */
    public function terminal(): Terminal
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('terminal');
        return new \Dagger\Terminal($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * print documentation for available tools
     */
    public function tools(): string
    {
        $leafQueryBuilder = new \Dagger\Client\QueryBuilder('tools');
        return (string)$this->queryLeaf($leafQueryBuilder, 'tools');
    }

    /**
     * Retrieve the llm state as a TypeDef
     */
    public function typeDef(): TypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('typeDef');
        return new \Dagger\TypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a CacheVolume
     */
    public function withCacheVolume(CacheVolumeId|CacheVolume $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withCacheVolume');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a Container
     */
    public function withContainer(ContainerId|Container $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withContainer');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a CurrentModule
     */
    public function withCurrentModule(CurrentModuleId|CurrentModule $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withCurrentModule');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a Directory
     */
    public function withDirectory(DirectoryId|Directory $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withDirectory');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a EnumTypeDef
     */
    public function withEnumTypeDef(EnumTypeDefId|EnumTypeDef $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withEnumTypeDef');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a EnumValueTypeDef
     */
    public function withEnumValueTypeDef(EnumValueTypeDefId|EnumValueTypeDef $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withEnumValueTypeDef');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a Error
     */
    public function withError(ErrorId|Error $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withError');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a ErrorValue
     */
    public function withErrorValue(ErrorValueId|ErrorValue $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withErrorValue');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a FieldTypeDef
     */
    public function withFieldTypeDef(FieldTypeDefId|FieldTypeDef $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withFieldTypeDef');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a File
     */
    public function withFile(FileId|File $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withFile');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a Function
     */
    public function withFunction(FunctionId|Function_ $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withFunction');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a FunctionArg
     */
    public function withFunctionArg(FunctionArgId|FunctionArg $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withFunctionArg');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a FunctionCall
     */
    public function withFunctionCall(FunctionCallId|FunctionCall $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withFunctionCall');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a FunctionCallArgValue
     */
    public function withFunctionCallArgValue(FunctionCallArgValueId|FunctionCallArgValue $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withFunctionCallArgValue');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a GeneratedCode
     */
    public function withGeneratedCode(GeneratedCodeId|GeneratedCode $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withGeneratedCode');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a GitRef
     */
    public function withGitRef(GitRefId|GitRef $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withGitRef');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a GitRepository
     */
    public function withGitRepository(GitRepositoryId|GitRepository $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withGitRepository');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a InputTypeDef
     */
    public function withInputTypeDef(InputTypeDefId|InputTypeDef $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withInputTypeDef');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a InterfaceTypeDef
     */
    public function withInterfaceTypeDef(InterfaceTypeDefId|InterfaceTypeDef $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withInterfaceTypeDef');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a ListTypeDef
     */
    public function withListTypeDef(ListTypeDefId|ListTypeDef $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withListTypeDef');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a Llm
     */
    public function withLlm(LlmId|Llm $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withLlm');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * swap out the llm model
     */
    public function withModel(string $model): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withModel');
        $innerQueryBuilder->setArgument('model', $model);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a Module
     */
    public function withModule(ModuleId|Module $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withModule');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a ModuleConfigClient
     */
    public function withModuleConfigClient(ModuleConfigClientId|ModuleConfigClient $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withModuleConfigClient');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a ModuleSource
     */
    public function withModuleSource(ModuleSourceId|ModuleSource $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withModuleSource');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a ObjectTypeDef
     */
    public function withObjectTypeDef(ObjectTypeDefId|ObjectTypeDef $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withObjectTypeDef');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a PhpSdk
     */
    public function withPhpSdk(PhpSdkId|PhpSdk $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withPhpSdk');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * append a prompt to the llm context
     */
    public function withPrompt(string $prompt): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withPrompt');
        $innerQueryBuilder->setArgument('prompt', $prompt);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * append the contents of a file to the llm context
     */
    public function withPromptFile(FileId|File $file): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withPromptFile');
        $innerQueryBuilder->setArgument('file', $file);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * set a variable for expansion in the prompt
     */
    public function withPromptVar(string $name, string $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withPromptVar');
        $innerQueryBuilder->setArgument('name', $name);
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a SDKConfig
     */
    public function withSDKConfig(SDKConfigId|SDKConfig $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withSDKConfig');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a ScalarTypeDef
     */
    public function withScalarTypeDef(ScalarTypeDefId|ScalarTypeDef $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withScalarTypeDef');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a Secret
     */
    public function withSecret(SecretId|Secret $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withSecret');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a Service
     */
    public function withService(ServiceId|Service $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withService');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a Socket
     */
    public function withSocket(SocketId|Socket $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withSocket');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a SourceMap
     */
    public function withSourceMap(SourceMapId|SourceMap $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withSourceMap');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a Terminal
     */
    public function withTerminal(TerminalId|Terminal $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withTerminal');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Set the llm state to a TypeDef
     */
    public function withTypeDef(TypeDefId|TypeDef $value): Llm
    {
        $innerQueryBuilder = new \Dagger\Client\QueryBuilder('withTypeDef');
        $innerQueryBuilder->setArgument('value', $value);
        return new \Dagger\Llm($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }
}
