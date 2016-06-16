package main

import (
    "fmt"
    "math/rand"
    "time"
)

//Poisons given channels (to exit threads)
func poison(inChs []chan int){
    for _, ch := range inChs {
        ch <- -1 //kill
        //fmt.Println("Poison")
    }
}

func pin(recieveCh, sendChL, sendChR, oscCh chan int, id, numberTillClose int) { 
    rand.Seed( time.Now().UTC().UnixNano()) //seed random generator
    numberClosed := 0
    for {
        //Range over one input challange
        select {
        case i := <- recieveCh:
            if i < 0 { //parentpin is closing
                if numberClosed++; numberClosed == numberTillClose {
                    a := [2]chan int{sendChL, sendChR}
                    poison(a[:])
                    return
                }
            } else {
                oscCh <- 1
                bias :=<- oscCh
                if rand.Intn(100) > bias { //This is a deterministic (semi) random generator. Bias turns to left channel
                    sendChL <- id
                } else {
                    sendChR <- id+1
                }
            }
       //case bias :=<- oscCh:

        }
    }
}

//Takes all recieve channels of the last pins.
func binCounter(inCh, oscQuitCh chan int, resultCh chan []int, numberBins, maxCount int){
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
        } else {
            fmt.Println("Error in bincounter! Got too large bin number.")
            return
        }
    }
    //Just to show, check that pins are dead. Could be removed
    for deadCh < numberBins-1 {
        i :=<- inCh
        if(i < 0){
            deadCh++
        }
    }
    oscQuitCh <- -1 //kill oscillator
    resultCh <- bins[:] //terminate.
}

//Take the channel to the first pin, and the number of beans to send.
func beanPutter(sCh, oCh chan int, numberOfBeans int){
    for i:=0; i<numberOfBeans; i++{
        sCh <- 1 //send beans to first pin
        oCh <- 1 //send update to oscillator
    }
    a := [1]chan int{sCh}
    poison(a[:])//kill beanMachine topdown
}

//Takes a list of channels to pins, an update channel from bean putter and a quit
//channel. Also takes an integer d, when 0 the oscillator value will always be 0.5 
//(equal), otherwise will incr/decr by d*0.01 in range [0:0.5].
func oscillator(chs []chan int, updCh, quit chan int, d int) {
    v := 50
    direction := -1
    //putterDead := false
    for {
        for _, c := range chs {
            select {
                case <- c: //only send value when requested.
                    c <- v
                case <-updCh:
                    v += d*direction
                    if v <= 0 {
                        direction = 1
                    } else if v >= 50 {
                        direction = -1
                    }
                case <-quit:
                    return
            default :
            }
        }
    }
}


//Create a level with n pins and start them.
//Takes output channels of the pins above, ordered left to right.
//Assumes number of given channels = n, the number of pins to create
//Returns new set of n+1 output channels to this level of pins, 
//ordered left to right.
func createLevelPins(n int, inChs, oscChs []chan int) []chan int{
    if len(inChs) != n || len(oscChs) != n { // also catches errorfull n's
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
        go pin(inChs[i], outChs[i], outChs[i+1], oscChs[i], i, numberTillClose)
    }
    return outChs[:]
}

func createBeanMachine(levels, numberOfBeans, skew int) (binExitCh chan []int){
    headCh := [1]chan int{make(chan int)}
    entryCh := headCh[0]
    bins := levels+1
    numberOfPins := (bins*bins-bins)/2
    pinOscChs := make([]chan int, numberOfPins) //total number of pins.
    for j := range pinOscChs {
        pinOscChs[j] = make(chan int)
    }
    outChs := headCh[:]

    for i:= 1; i < levels; i++ {
        j := (i*i-i)/2
        oscChs := pinOscChs[j:j+i] //j+i-1??
        outChs = createLevelPins(i, outChs, oscChs)
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
        go pin(outChs[i], binInCh, binInCh, pinOscChs[numberOfPins-len(outChs)+i], i, numberTillClose)
    }

    binExitCh = make(chan []int)
    oscillatorCh := make(chan int)
    oscillatorQuitCh := make(chan int)
    go binCounter(binInCh, oscillatorQuitCh, binExitCh, bins, numberOfBeans)
    go oscillator(pinOscChs, oscillatorCh, oscillatorQuitCh, skew)
    go beanPutter(entryCh, oscillatorCh, numberOfBeans)
    return
}

func main() {
    numberOfBeans := 1000
    levels := 50
    skew := 0 //1 for skew of problem 3
    binExit := createBeanMachine(levels, numberOfBeans, skew)
    
    select {
    case bins :=<- binExit:
        count := 0
        fmt.Println("\nBeans in bins (left to right):")
        for _, bin := range bins {
            fmt.Println(bin)
            count += bin
        }
        fmt.Println("\nTotal counted beans in bins: ", count)
        return
        //Now done, what to do with results?
    }
}