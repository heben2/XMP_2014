package main

import (
    "fmt"
//    "math/rand"
//    "time"
//    "container/list"
//    "os"
//    "strings"
)

func playerInput(c chan string) {
    var s string
    //c := make(chan string)
    for i:=0;i<10;i++ {
        fmt.Scan(&s)
        c<-s
    }
}


func main() {

    c := make(chan string)
    go playerInput(c)

    for i:=0;i<10;i++ {
        fmt.Println(<-c)
    } 




/*
    for {
        var i string
        fmt.Scan(&i)
        fmt.Println("read string", i, "from stdin")
        if i == "quit" {
            return
        }
    }
    */
    /*
    m := make(map[string]int)
    m["a"] = 1
    m["b"] = 2
    m["c"] = 3
    for k, v := range m {
        fmt.Println("k:", k, "v:", v)
    }
    var ok bool
    _, ok = m["t"]
    fmt.Println(ok)
    _, ok = m["a"]
    fmt.Println(ok)

    s := "testing you"
    k := strings.Fields(s)
    fmt.Println(k[1])

*/
    /*
    players := list.New()
    c := make(chan int)
    //c := "s"
    players.PushFront(c)

    

    a := players.Front()
    if c == a.Value {
        fmt.Println("test")
    } else {
        fmt.Println("fuck")
    }

    players.Remove(&Element{Value: c})
    
    //fmt.Println("players", a.Value)
    */
    /*
    for e := players.Front(); e != nil; e = e.Next() {
        fmt.Println("playerval", e.Value)
    } */

}