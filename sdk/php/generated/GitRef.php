<?php

/**
 * This class has been generated by dagger-php-sdk. DO NOT EDIT.
 */

declare(strict_types=1);

namespace Dagger\Dagger;

/**
 * A git ref (tag, branch or commit).
 */
class GitRef extends \Dagger\Client\AbstractDaggerObject implements \Dagger\Client\IdAble
{
    /**
     * The resolved commit id at this ref.
     */
    public function commit(): string
    {
        $leafQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('commit');
        return (string)$this->queryLeaf($leafQueryBuilder, 'commit');
    }

    /**
     * Retrieves the content-addressed identifier of the git ref.
     */
    public function id(): GitRefId
    {
        $leafQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('id');
        return new \Dagger\Dagger\GitRefId((string)$this->queryLeaf($leafQueryBuilder, 'id'));
    }

    /**
     * The filesystem tree at this ref.
     */
    public function tree(?string $sshKnownHosts = null, SocketId|Socket|null $sshAuthSocket = null): Directory
    {
        $innerQueryBuilder = new \Dagger\Client\DaggerQueryBuilder('tree');
        if (null !== $sshKnownHosts) {
        $innerQueryBuilder->setArgument('sshKnownHosts', $sshKnownHosts);
        }
        if (null !== $sshAuthSocket) {
        $innerQueryBuilder->setArgument('sshAuthSocket', $sshAuthSocket);
        }
        return new \Dagger\Dagger\Directory($this->client, $this->queryBuilderChain->chain($innerQueryBuilder));
    }
}
