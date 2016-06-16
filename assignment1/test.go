package main

import "fmt"

func fibonacci(c chan int) {
    for {
        select {
        case c, ok := <- c:
            if(ok){
                fmt.Println(c)
            }else{
                return
            }
        }
    }
}

func main() {
    c := make(chan int)
    go fibonacci(c)
    for i := 0; i < 10; i++ {
            c <- i
        }
    close(c)
}