package main

import (
    "fmt"
    "math/rand"
    "time"
)

//I do not use close(channel) to kill threads, as this kills quoue also, which can contain beans.
//go is not optimized for recursion, so I did not use this.
//Note that slices (of arrays) are shared by the caller, 
// so I must ensure I do not use it afterwards. Not very CSP.

//Poisons given channels (to exit threads)
func poison(inChs []chan int){
    for _, ch := range inChs {
        ch <- -1 //kill
    }
}

func pin(recieveCh, sendChL, sendChR chan int, id, numberTillClose int) { 
    rand.Seed( time.Now().UTC().UnixNano()) //seed random generator
    numberClosed := 0
    bias := 50
    for {
        //Range over one input challange
        select {
        case i:= <- recieveCh:
            if i < 0 { //parentpin is closing
                if numberClosed++; numberClosed == numberTillClose {
                    a := [2]chan int{sendChL, sendChR}
                    poison(a[:])
                    return
                }
            } else {
                if rand.Intn(100) > bias { //This is a deterministic (semi) random generator. Bias turns to left channel
                    sendChL <- id
                } else {
                    sendChR <- id+1
                }
            }
        //case i:= <- oscillator channel
        }
    }
}

//Takes all recieve channels of the last pins.
func binCounter(inCh chan int, resultCh chan []int, numberBins, maxCount int){
    countBeans := 0
    bins := make([]int,numberBins) //initiated to 0
    deadCh := 0

    for countBeans < maxCount {
        i :=<- inCh
        if(i < 0){
            deadCh++
        } else if i < numberBins {
            bins[i]++
            countBeans++
        } else{
            fmt.Println("Error in bincounter! Got too large bin number.")
            return
        }
    }
    //Check that pins are dead. TODO:remove
    for deadCh < numberBins-1 {
        i :=<- inCh
        if(i < 0){
            deadCh++
        }
    }

    resultCh <- bins[:] //Return.
}

//Take the channel to the first pin, and the number of beans to send.
func beanPutter(sch chan int, numberOfBeans int){
    for i:=0; i<numberOfBeans; i++{
        sch <- 1 //send beans to first pin
    }
    a := [1]chan int{sch}
    poison(a[:])//kill beanMachine topdown
}

//Create a level with n pins and start them.
//Takes output channels of the pins above, ordered left to right.
//Assumes number of given channels = n, the number of pins to create
//Returns new set of n+1 output channels to this level of pins, 
//ordered left to right.
func createLevelPins(n int, inChs []chan int) []chan int{
    if len(inChs) != n { // also catches errorfull n's
        fmt.Println("createLevelPins: error in length of list")
        a := []chan int{}
        return a //TODO: error
    }

    //initiate channels
    outChs := make([]chan int, n+1)
    for i := range outChs {
        outChs[i] = make(chan int)
    } 

    //instantiate the n pins on their own threads
    numberTillClose := 2
    for i := 0; i < n; i++ { 
        if i==0 || i== n-1 {
            numberTillClose = 1
        } else {
            numberTillClose = 2
        }
        go pin(inChs[i], outChs[i], outChs[i+1], i, numberTillClose)
    }
    return outChs[:]
}

func createBeanMachine(levels int, numberOfBeans int) (binExitCh chan []int, entryCh chan int){
    headCh := [1]chan int{make(chan int)}
    entryCh = headCh[0]
    bins := levels+1

    outChs := createLevelPins(1, headCh[:])
    for i:= 2; i < levels; i++ {
        outChs = createLevelPins(i, outChs)
    }
    //last level not created. Needs special bin-channel.
    binInCh := make(chan int)
    numberTillClose := 2
    for i := 0; i < len(outChs); i++ {
        if i == 0 || i== len(outChs)-1 {
            numberTillClose = 1
        } else {
            numberTillClose = 2
        }
        go pin(outChs[i], binInCh, binInCh, i, numberTillClose)
    }

    binExitCh = make(chan []int)
    go binCounter(binInCh, binExitCh, bins, numberOfBeans)
    return
}

func main() {
    numberBeans := 1000
    numberLevels := 50
    binExit, entry := createBeanMachine(numberLevels, numberBeans)
    go beanPutter(entry, numberBeans)

    select {
    case bins :=<- binExit:
        count := 0
        fmt.Println("Beans in bins (left to right):")
        for _, bin := range bins {
            fmt.Println(bin)
            count += bin
        }
        fmt.Println("\nTotal counted beans in bins: ", count)
        return
        //Now done, what to do with results?
    }
}