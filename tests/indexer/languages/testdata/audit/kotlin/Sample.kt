package app.core

enum class Mode {
    FAST,
    SAFE
}

interface Worker {
    fun execute(input: String): Boolean
}

object Config {
    val version: String = "1.0"
    fun load(): String = version
}

class Service(val name: String) : Worker {
    val retries: Int = 3

    override fun execute(input: String): Boolean {
        val trimmed = input.trim()
        return helper(trimmed)
    }
}

fun helper(s: String): Boolean = s.isNotEmpty()
