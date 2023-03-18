package main

import (
	"fmt"
	"math/rand"
	"time"
)

// Generator: function that returns a channel
func boring(msg string) <-chan string { // returns receive-only channel of strings
	c := make(chan string)
	go func() {
		for i := 0; ; i++ {
			c <- fmt.Sprintf("%s %d", msg, i)
			time.Sleep(time.Duration(rand.Intn(1e3)) * time.Millisecond)
		}
	}()
	return c
}

func main() {
	c := boring("boring")
	d := boring("new boring")
	for i := 0; i < 5; i++ {
		fmt.Printf("You say: %q\n", <-c) // receive on the channel
		fmt.Printf("You say: %q\n", <-d) // receive on the channel
	}
	fmt.Println("You're boring; I'm leaving.")
}
