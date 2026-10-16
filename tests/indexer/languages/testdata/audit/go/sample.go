package sample

import (
	"fmt"
	"strings"
)

const Version = "1.0"

const (
	ModeA = iota
	ModeB
)

var DefaultName = "x"

var handler = func(s string) string { return strings.ToUpper(s) }

type Alias = string

type Weekday int

type Shape interface {
	Area() float64
	Perimeter() float64
}

type Point struct {
	X, Y  int
	Label string
}

type Named struct {
	Point
	Tags []string `json:"tags"`
}

func New(x, y int) *Point {
	return &Point{X: x, Y: y}
}

func (p *Point) String() string {
	return fmt.Sprintf("%d,%d", p.X, p.Y)
}

func Generic[T any](v T) T {
	return v
}

type Stack[T any] struct {
	items []T
}

func (s *Stack[T]) Push(v T) {
	s.items = append(s.items, v)
}
