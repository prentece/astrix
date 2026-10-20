<?php

namespace App\Sample;

use App\Contracts\Repo;

const TOP_LEVEL = 1;

function helper(int $x): int
{
    return $x + 1;
}

trait Greets
{
    public function hello(): string
    {
        return 'hi';
    }
}

enum Status: string
{
    case Active = 'active';
    case Off = 'off';
}

interface Named
{
    const PREFIX = 'n_';

    public function name(): string;
}

abstract class Model implements Named
{
    use Greets;

    public const TABLE = 'models';
    protected string $title = '';
    private static int $count = 0;

    public function __construct(protected int $id)
    {
    }

    abstract public function save(): void;

    public function name(): string
    {
        return $this->title;
    }
}

$anon = function () {
    return 1;
};
