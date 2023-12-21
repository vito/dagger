<?php

/**
 * This class has been generated by dagger-php-sdk. DO NOT EDIT.
 */

declare(strict_types=1);

namespace Dagger\Dagger;

/**
 * A definition of a parameter or return type in a Module.
 */
class TypeDef extends \Dagger\Client\AbstractDaggerObject implements \Dagger\Client\IdAble
{
    /**
     * If kind is LIST, the list-specific type definition.
     * If kind is not LIST, this will be null.
     */
    public function asList(): ListTypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('asList');
        return new \Dagger\Dagger\ListTypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * If kind is OBJECT, the object-specific type definition.
     * If kind is not OBJECT, this will be null.
     */
    public function asObject(): ObjectTypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('asObject');
        return new \Dagger\Dagger\ObjectTypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    public function id(): TypeDefId
    {
        $leafQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('id');
        return new \Dagger\Dagger\TypeDefId((string)$this->queryLeaf($leafQueryBuilder, 'id'));
    }

    /**
     * The kind of type this is (e.g. primitive, list, object)
     */
    public function kind(): TypeDefKind
    {
        $leafQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('kind');
        return \Dagger\Dagger\TypeDefKind::from((string)$this->queryLeaf($leafQueryBuilder, 'kind'));
    }

    /**
     * Whether this type can be set to null. Defaults to false.
     */
    public function optional(): bool
    {
        $leafQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('optional');
        return (bool)$this->queryLeaf($leafQueryBuilder, 'optional');
    }

    /**
     * Adds a function for constructing a new instance of an Object TypeDef, failing if the type is not an object.
     */
    public function withConstructor(FunctionId|Function_ $function): TypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('withConstructor');
        $innerQueryBuilder->setArgument('function', $function);
        return new \Dagger\Dagger\TypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Adds a static field for an Object TypeDef, failing if the type is not an object.
     */
    public function withField(string $name, TypeDefId|TypeDef $typeDef, ?string $description = null): TypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('withField');
        $innerQueryBuilder->setArgument('name', $name);
        $innerQueryBuilder->setArgument('typeDef', $typeDef);
        if (null !== $description) {
        $innerQueryBuilder->setArgument('description', $description);
        }
        return new \Dagger\Dagger\TypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Adds a function for an Object TypeDef, failing if the type is not an object.
     */
    public function withFunction(FunctionId|Function_ $function): TypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('withFunction');
        $innerQueryBuilder->setArgument('function', $function);
        return new \Dagger\Dagger\TypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Sets the kind of the type.
     */
    public function withKind(TypeDefKind $kind): TypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('withKind');
        $innerQueryBuilder->setArgument('kind', $kind);
        return new \Dagger\Dagger\TypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Returns a TypeDef of kind List with the provided type for its elements.
     */
    public function withListOf(TypeDefId|TypeDef $elementType): TypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('withListOf');
        $innerQueryBuilder->setArgument('elementType', $elementType);
        return new \Dagger\Dagger\TypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Returns a TypeDef of kind Object with the provided name.
     *
     * Note that an object's fields and functions may be omitted if the intent is
     * only to refer to an object. This is how functions are able to return their
     * own object, or any other circular reference.
     */
    public function withObject(string $name, ?string $description = null): TypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('withObject');
        $innerQueryBuilder->setArgument('name', $name);
        if (null !== $description) {
        $innerQueryBuilder->setArgument('description', $description);
        }
        return new \Dagger\Dagger\TypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }

    /**
     * Sets whether this type can be set to null.
     */
    public function withOptional(bool $optional): TypeDef
    {
        $innerQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('withOptional');
        $innerQueryBuilder->setArgument('optional', $optional);
        return new \Dagger\Dagger\TypeDef($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }
}
