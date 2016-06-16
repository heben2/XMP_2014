package main

import (
    "fmt"
    "math/rand"
)

//go threads are connected only by channels (i.e. no thread id).
//Must create each layer while knowing the layer above.
//go is not optimized for recursion, so I did not use this.
//Note that slices (of arrays) are shared by the caller, 
// so I must ensure I do not use it afterwards. Not very CSP.

//Poisons given channels (to exit threads)
func poison(chs []chan int){
    for _, ch := range chs {
        ch <- -1 //kill
    }
}

func pinHandle(i int, sendChL, sendChR chan int) bool {
    if i < 0 { //killed
        a := [2]chan int{sendChL, sendChR}
        poison(a[:])
        return false
    }
    if rand.Intn(2) < 1 { //This is a deterministic (semi) random generator
        sendChL <- i
    } else {
        sendChR <- i
    }
    return true
}

func pin(recieveChL, recieveChR, sendChL, sendChR chan int) { //TODO: Include poison count (from 2 channels)
    for {
        select {
        case i:= <- recieveChL:
            if !pinHandle(i, sendChL, sendChR) {
                return //poisoned
            }
        case i:= <- recieveChR: //Ugly dublicate, fallthrough does not work in select
            if !pinHandle(i, sendChL, sendChR) {
                return //poisoned
            }
        }
    }
}

//Takes all recieve channels of the last pins.
func bin_counter(rchs []chan int, resultCh chan []int, maxCount int){
    counter := 0
    n := len(rchs)
    var bins [n]int //initiated to 0
    deadCh := 0
    for counter < maxCount {
        for i:=0; i<n; i++ { //Do not wait on a single channel.
            select {
            case j:=<- rchs[i]:
                counter++
                bins[i]++
                if j < 0 {
                    deadCh++
                    if deadCh == n {
                        return //all should be dead
                    }
                }
            default:
            }
        }
    }
    resultCh <- bins //Return.
}

//Create a level with n pins and start them.
//Given channels to the pins above, ordered left to right.
//Assumes number of given channels = n+2 (one null channel for each side)
//Returns new set of channels to this level of pins, 
//ordered left to right and with 2 null channels.
func createLevelPins(n int, chs []chan int) (returnChs []chan int){
    if len(chs) != n*2 { // also catches errorfull n's
        return //TODO: error
    }

    //initiate channels
    var returnChs [n*2+2]chan int
    for i := range returnChs {
        returnChs[i] = make(chan int)
    } 

    //instantiate the n pins on their own threads
    for i := 0; i < n; i++ { 
        go pin(chs[i*2], chs[i*2+1], returnChs[1+i*2], returnChs[1+i*2+1])
    }
}

func createBeanMachine(levels int, numberOfBeans int) (binExitCh chan []int, entryCh chan int){
    var headCh [2]chan int
    for i := range headCh {
        headCh[i] = make(chan int)
    }
    entryCh = headCh[0]
    returnChs := createLevelPins(1, headCh)
    for i:= 2; i < levels+1; i++ {
        returnChs = createLevelPins(i, returnChs)
    }
    binExitCh = make(chan int)
    go bin_counter(returnChs, binExitCh, numberOfBeans)
}

//Take the channel to the first pin, and the number of beans to send.
func beanPutter(sch chan int, numberOfBeans int){
    for i:=0; i<numberOfBeans; i++{
        sch <- 1 //send beans to first pin
    }
    a := [1]chan int{sch}
    poison(a[:])//kill beanMachine topdown
}

func main() {
    n := 100
    binExit, entry = createBeanMachine(2, n)
    go beanPutter(entry, n)

    for {
        select {
        case i:= <- binExit:
            //Now done, what to do with results?
        }
    }
}