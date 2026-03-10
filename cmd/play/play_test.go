package play

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type (
	Counter struct {
		Value int
	}

	Op func(Counter, int) int
)

// 1) standalone function
func Add(c Counter, n int) int {
	return c.Value + n
}

// 2) method (equivalent underlying shape: func(*Counter, int) int)
func (c Counter) Add(n int) int {
	return c.Value + n
}

// https://go.dev/play/p/29FPZ_Yi25K
func TestSugar(t *testing.T) {
	c := Counter{Value: 10}
	n := 5

	// function variable with the method/function signature
	var op Op

	// assign standalone function
	op = Add
	fmt.Println(op(c, n)) // 15

	// assign method expression (unbound method), same function signature
	op = Counter.Add
	fmt.Println(op(c, n)) // 15

	// normal method call sugar for comparison
	fmt.Println(c.Add(n)) // 15

	assert.Equal(t, "", "")
}
