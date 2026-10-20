import os
from typing import Optional

MAX_RETRIES = 3
_cache: dict = {}
DEBUG: bool = False


def top_level(x: int) -> int:
    return x + 1


async def fetch(url: str) -> str:
    return url


class Outer:
    LIMIT = 5
    name: str = "outer"

    class Nested:
        def inner(self) -> None:
            pass

    def __init__(self, value: int) -> None:
        self.value = value

    @property
    def doubled(self) -> int:
        return self.value * 2

    @staticmethod
    def make() -> "Outer":
        return Outer(1)

    @classmethod
    def build(cls) -> "Outer":
        return cls(2)


@decorator
class Decorated:
    pass


square = lambda n: n * n
