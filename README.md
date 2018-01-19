```go

package main

import (
	"fmt"
	"github.com/encobrain/go-context"
)

func test () {
    abc := context.Get("abc").(string)	
    
    fmt.Println("Got abc:", abc)
}

func main () {
	
	context.Run(func() {
		context.Set("abc","test ok")
		
		test()
	})
}

```