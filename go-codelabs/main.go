package main

import (
	"fmt"
)

func main() {
	ch := make(chan string)

	go func() {
		msg := <-ch
		fmt.Println(msg)
	}()

	ch <- "hello"

	fmt.Println("main")
}
