# Concurrency

## Concurrency in Go - Tools and Techniques for Developers - Page 33

- Are you trying to guard internal state of a struct?

This is a great candidate for memory access synchronization primitives, and a
pretty strong indicator that you shouldn't use channels. By using memory access
synchronization primitives, you can hind the implementation detail of locking
your critical section from your callers. Here's a small example of a type that
is thread-safe, but doesn't expose that complexity to it's callers:

```go
type Counter struct {
    mu sync.Mutex
    value int
  }

func (c *Counter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.value++
  }
```

If you recall the concept of atomicity, we can say that what we've done here is
defined the scope of atomicity for the `Counter type`. Calls to the `Increment`
can be considered atomic.

Remember the key word here is `internal`. If you find yourself exposing locks
beyond a type, this should raise a red flag. Try to keep the locks constrained
to a small lexical scope.

- Is it a performance-critical section?

This absolutely does not mean, "I want my program to be performant, therefore I
will only use mutexes." Rather, if you have a section of your program that you
have profiled, and it turns out to be a major bottleneck that is orders of
magnitude slower than the rest of the program, using memory access
synchronization primitives may help this critical section perform under load.
This is because channels use memory access synchronization to operate,
therefore they can only be slower. Before we even consider this, however, a
performance-critical section might be hinting that we need to restructure our
program.

## Page 43 - Goroutines are not garbage collected.

We combine the fact that goroutines are not garbage collected with the runtime's
ability to introspect upon itself and measure the amount of memory allocated
before and after goroutine creation:

```go
memConsumed := func() uint64 {
    runtime.GC()
    var s runtime.MemStats
    runtime.ReadMemStats(&s)
    return s.Sys
  }

c := make(<-chan interface{})
wg := sync.WaitGroup{}
noop := func() { wg.Done(); <-c }

const numGoroutines = 1e4
wg.Add(numGoroutines)
before := memConsumed()
for i := numGoroutines; i > 0; i-- {
    go noop()
}
wg.Wait()
after := memConsumed()
fmt.Printf("%.3fkb", float64(after-before)/numGoroutines/1000)

```
