lane
====

Lane package provides queue, priority queue, stack and deque data structures
implementations. Its was designed with simplicity, performance, and concurrent
usage in mind.


#### Priority Queue

Pqueue is a *heap priority queue* data structure implementation. It can be whether max or min ordered, is synchronized and is safe for concurrent operations. It performs insertion and max/min removal in O(log N) time.

##### Example

```go
	// Let's create a new max ordered priority queue
	var priorityQueue *PQueue = NewPQueue(MINPQ)

	// And push some prioritized content into it
	priorityQueue.Push("easy as", 3)
	priorityQueue.Push("123", 2)
	priorityQueue.Push("do re mi", 4)
	priorityQueue.Push("abc", 1)

	// Now let's take a look at the min element in
	// the priority queue
	headValue, headPriority := priorityQueue.Head()
	fmt.Println(headValue)    // "abc"
	fmt.Println(headPriority) // 1

	// Okay the song order seems to be preserved, let's
	// roll
	var jacksonFive []string = make([]string, priorityQueue.Size())

	for i := 0; i < len(jacksonFive); i++ {
		value, _ := priorityQueue.Pop()

		jacksonFive[i] = value.(string)
	}

	fmt.Println(strings.Join(jacksonFive, " "))
```

#### Deque

Deque is a *head-tail linked list data* structure implementation. It is based on a doubly-linked list container, so that every operations time complexity is O(1). All operations over an instiated Deque are synchronized and safe for concurrent usage. 

Deques can optionally be created with a limited capacity, whereby the return value of the `Append` and `Prepend` return false if the Deque was full and the item was not added.

##### Example

```go
	// Let's create a new deque data structure
	var deque *Deque = NewDeque()

	// And push some content into it using the Append
	// and Prepend methods
	deque.Append("easy as")
	deque.Prepend("123")
	deque.Append("do re mi")
	deque.Prepend("abc")

	// Now let's take a look at what are the first and
	// last element stored in the Deque
	firstValue := deque.First()
	lastValue := deque.Last()
	fmt.Println(firstValue) // "abc"
	fmt.Println(lastValue)  // 1

	// Okay now let's play with the Pop and Shift
	// methods to bring the song words together
	var jacksonFive []string = make([]string, deque.Size())

	for i := 0; i < len(jacksonFive); i++ {
		value := deque.Shift()
		jacksonFive[i] = value.(string)
	}

	// abc 123 easy as do re mi
	fmt.Println(strings.Join(jacksonFive, " "))
```

```go
	// Let's create a new musical quartet
	quartet := NewCappedDeque(4)

	// List of young hopeful musicians
	musicians := []string{"John", "Paul", "George", "Ringo", "Stuart"}

	// Add as many of them to the band as we can.
	for _, name := range musicians {
		if quartet.Append(name) {
			fmt.Printf("%s is in the band!\n", name)
		} else {
			fmt.Printf("Sorry - %s is not in the band.\n", name)
		}
	}

	// Assemble our new rock sensation
	var beatles = make([]string, quartet.Size())

	for i := 0; i < len(beatles); i++ {
		beatles[i] = quartet.Shift().(string)
	}

	fmt.Println("The Beatles are:", strings.Join(beatles, ", "))
```

#### Queue

Queue is a **FIFO** ( *First in first out* ) data structure implementation. It is based on a deque container and focuses its API on core functionalities: Enqueue, Dequeue, Head, Size, Empty. Every operations time complexity is O(1). As it is implemented using a Deque container, every operations over an instiated Queue are synchronized and safe for concurrent usage.

##### Example

```go
    import (
        "fmt"
        "github.com/oleiade/lane"
        "sync"
    )

    func worker(item interface{}, wg *sync.WaitGroup) {
        fmt.Println(item)
        wg.Done()
    }


    func main() {

        queue := lane.NewQueue()
        queue.Enqueue("grumpyClient")
        queue.Enqueue("happyClient")
        queue.Enqueue("ecstaticClient")

        var wg sync.WaitGroup

        // Let's handle the clients asynchronously
        for queue.Head() != nil {
            item := queue.Dequeue()

            wg.Add(1)
            go worker(item, &wg)
        }

        // Wait until everything is printed
        wg.Wait()
    }
```

#### Stack

Stack is a **LIFO** ( *Last in first out* ) data structure implementation. It is based on a deque container and focuses its API on core functionalities: Push, Pop, Head, Size, Empty. Every operations time complexity is O(1). As it is implemented using a Deque container, every operations over an instiated Stack are synchronized and safe for concurrent usage.

##### Example

```go
	// Create a new stack and put some plates over it
	var stack *Stack = NewStack()

	// Let's put some plates on the stack
	stack.Push("redPlate")
	stack.Push("bluePlate")
	stack.Push("greenPlate")

	fmt.Println(stack.Head) // greenPlate

	// What's on top of the stack?
	value := stack.Pop()
	fmt.Println(value.(string)) // greenPlate

	stack.Push("yellowPlate")
	value = stack.Pop()
	fmt.Println(value.(string)) // yellowPlate

	// What's on top of the stack?
	value = stack.Pop()
	fmt.Println(value.(string)) // bluePlate

	// What's on top of the stack?
	value = stack.Pop()
	fmt.Println(value.(string)) // redPlate
```


## Documentation

For a more detailled overview of lane, please refer to [Documentation](http://godoc.org/github.com/oleiade/lane)


[![Bitdeli Badge](https://d2weczhvl823v0.cloudfront.net/oleiade/lane/trend.png)](https://bitdeli.com/free "Bitdeli Badge")

