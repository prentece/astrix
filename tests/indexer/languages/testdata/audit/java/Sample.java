package sample;

import java.util.List;

@interface Marker {
    String value() default "";
}

public enum Color {
    RED, GREEN;

    public String label() {
        return name();
    }
}

public record Pair(int a, int b) {
    public Pair {
        if (a < 0) throw new IllegalArgumentException();
    }

    public int sum() {
        return a + b;
    }
}

public interface Repo<T> {
    T find(long id);
    default void touch() {}
}

public abstract class Base {
    private static final int LIMIT = 10;
    protected String name;
    int count;

    public Base(String name) {
        this.name = name;
    }

    public abstract void run();

    public static class Inner {
        void go() {}
    }
}
