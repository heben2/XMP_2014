package main

import (
    "fmt"
    "math/rand"
    "time"
    "strconv"
)

//Directions with enum type
type Direction int
const (
        N Direction = iota //0
        S //1
        E //2
        W //3
)

type Position struct {
    X, Y int
}

type Pedestrian struct {
    Id int
    Dir Direction
}

//Struct for messages send over channels
type Message struct {
    Tag string
    Pos Position
    Test bool
    Id int
    Pedestrian Pedestrian
    Drunk bool
    DrunkArea int
}
//Struct for a bidirectional channel, where a process only sends on Out and recieves on another In.
type Bichannel struct {
    In chan Message
    Out chan Message
}
//make a new Bichannel, return both versions of it
func makeNewBichannel() (Bichannel, Bichannel) {
    ch1 := make(chan Message)
    ch2 := make(chan Message)
    return Bichannel{In: ch1, Out: ch2}, Bichannel{In: ch2, Out: ch1}
}

func oppositeDirection(d Direction) Direction {
    switch d {
        case N: 
            return S
        case S:
            return N
        case E:
            return W
        case W:
            return E
    }
    return d //should never happen
}

/*
    A cell is an active automaton.
    Listens on neighbors.
    If occupied, tries to push pedestrian to next cell at each step.
*/
func cell(pos Position, neighbors [4]Bichannel, controlCh Bichannel){
    rand.Seed(time.Now().UTC().UnixNano()) //seed random generator
    var msg Message
    var pedestrian Pedestrian
    var orthoDirections [2]Direction
    drunk := false
    drunkEffect := false
    occupied := false
    neighborsDone := [4]bool{}
    doneRecieved := 0
    doneSend := 0

    numNeighbors := 0
    for i := range neighbors {
        if neighbors[i] != (Bichannel{}){
            numNeighbors++
        }
    }

    timeout := make(chan bool, 1)
    go func() {
        for {
            time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
            timeout <- true
        }
    }()

    occupy := func(ped Pedestrian){
        occupied = true;
        pedestrian = ped
        direction := pedestrian.Dir
        if direction == N || direction == S {
        orthoDirections = [2]Direction{E, W}
        } else if direction == E || direction == W {
            orthoDirections = [2]Direction{N, S}
        }
    }
    leave := func(){
        orthoDirections = [2]Direction{}
        pedestrian = Pedestrian{}
        occupied = false
    }

    spreadDrunk := func(fromIndex, area int){
        for j, ch := range neighbors {
            if j != fromIndex {
                ch.Out<-Message{Tag: "drunkArea", DrunkArea: area}
                msg=<-ch.In
                //get confirmation
                if msg.Tag != "drunkArea" { //TODO should not happen
                    fmt.Println("cell: ERROR when spreading drunkarea")
                }
            }
        }
    }

    killNext := func(d Direction){
        if neighbors[d] != (Bichannel{}) {
            neighbors[d].Out<-Message{Tag: "die"}
        }
    }

    listenNeighbors := func(){
        for i, c := range neighbors {
            select {
            case msg =<-c.In:
                switch msg.Tag {
                    case "occupy":
                        if occupied {
                            c.Out<-Message{Tag: "occupy", Test: false}
                        } else {
                            c.Out<-Message{Tag: "occupy", Test: true}
                            occupy(msg.Pedestrian)

                        }
                    case "die"://Must die. Cannot expect other input. Kill next
                        fmt.Println("cell: die")
                        switch i {
                        case 0:
                            killNext(S)
                        case 1:
                            killNext(N)
                        case 2:
                            killNext(W)
                        case 3:
                            killNext(E)
                        }
                        return
                    case "done": //a neighbor is done
                        doneRecieved++
                    case "drunkMove":
                        drunk = true
                        drunkEffect = true
                        spreadDrunk(-1, 3)
                    case "drunkArea": //spread drunk area
                        drunkEffect = true
                        k := msg.DrunkArea
                        if k > 1 {
                            spreadDrunk(i, k-1)
                        }
                        //confirm
                        c.Out<-Message{Tag: "drunkArea", Test: true}
                    default: //errorfull message tag, should not happen
                         fmt.Println("cell: Error in tag", msg.Tag, "pos:", pos)
                }
            default://skip
            }
        }
    }


    drunkMove := func(){
        i := rand.Intn(5)
        if i == 5 { //drunk stays
            spreadDrunk(-1, 3)
        } else {
            ch := neighbors[i]
            if ch != (Bichannel{}){ //dies if moving out of automaton.
                done := false
                for !done {
                    select{
                    case ch.Out<-Message{Tag: "drunkMove"}:
                        done = true
                    case <-timeout:
                        listenNeighbors() //TODO needed? other drunks could cause messages send, so yes?
                    }
                }
                
            }
            drunk = false
        }
    }

    tryMove := func(ch Bichannel) {
        if ch == (Bichannel{}) { //Check if ch is valid
            return
        }
        success := false
        for !success {
            select {
            case ch.Out<-Message{Tag: "occupy", Pedestrian: pedestrian}: //try to move ped to next cell
                msg =<-ch.In
                if msg.Tag == "occupy" && msg.Test {
                    leave()
                    //fmt.Println("cell: leave")
                }
                success = true
            case <-timeout: //force to listen to neighbors for a while
                //skip
            }
            listenNeighbors()
        }
    }

    waitNextStep := func(){
        drunkEffect = false //reset drunkeffect. May be added again after done-check if drunk still within range.
        //First send/get notification from all adjecent cells that no pedestrian is comming.
        done := false
        neighborSend := [4]bool{false, false, false, false}

        for !done {
            if doneRecieved == numNeighbors && doneSend == numNeighbors {
                done = true
            }
            for i, c := range neighbors {
                if !neighborSend[i] {
                    select{
                        case c.Out<-Message{Tag: "done"}:
                            neighborSend[i] = true
                            doneSend++
                        case <-timeout: //force to listen for a time
                            listenNeighbors()
                    }
                }
            }
        }
        //move drunk, if any. This blocks until moved.
        if drunk {
            drunkMove()
        }

        step := false
        for !step {
            select{
            case controlCh.Out<-Message{Tag:"status", Pos: pos, Test: occupied, Pedestrian: pedestrian}:
            case msg =<- controlCh.In: //step
                if msg.Tag == "drunkMove" {
                    drunk = true
                    drunkEffect = true
                    spreadDrunk(-1, 3)
                    //confirm drunk and effect added.
                    controlCh.Out<-Message{Tag: "drunkMove", Test: true}
                } else {
                    step = true
                }
            case <-timeout: //Force to listen for eventual die requests
                listenNeighbors()
            }
            
        }
        neighborsDone = [4]bool{}//reset
        doneRecieved = 0
        doneSend = 0
    }
    <- controlCh.In //initial step
    for {
        waitNextStep()
        if occupied { //only listen on control channel if occupied
            tryMove(neighbors[pedestrian.Dir])
            if occupied { //prime direction not possible, try orthogonal directions
                i := rand.Intn(2)
                tryMove(neighbors[orthoDirections[i]])
                if occupied { //try other orthogonal direction
                    i++
                    tryMove(neighbors[orthoDirections[i % 2]])
                }
            }
        }
    }
}




/*
    Used at the ends of the side walk. 
    Injects new pedestrians when told (on a specific time step).
    Also kills cells on termination (and dies).
    Dies if all close cells tells it to.
    direction is the direction to put new pedestrians.
*/
func injector(cellChs []Bichannel, controlCh Bichannel, direction Direction){
    var msg Message

    die := func(index int){
        //expect "die" messages from all other adjecent cells but the initial one
        for i, c := range cellChs {
            if i != index {
                found := false
                for !found {
                    msg=<-c.In
                    if msg.Tag == "die" { //TODO can be removed, should not happen.
                        found = true
                    }
                }
            }
        }
        controlCh.Out<-Message{Tag: "die"}
        fmt.Println("injector: die end")
    }

    for {
        select {
        case msg =<-controlCh.In: //wait for time step
            switch msg.Tag {
            case "inject":
                //Inject at first free cell, chosen in random order.
                pedestrian := Pedestrian{Dir: direction, Id: msg.Id}
                found := false
                shuffledList := rand.Perm(len(cellChs))
                for _, i := range shuffledList {
                    ch := cellChs[i]
                    if !found {
                        ch.Out<-Message{Tag: "occupy", Pedestrian: pedestrian}
                        msg =<-ch.In
                        if msg.Tag == "occupy" {
                            found = msg.Test
                        } else if msg.Tag == "die" {
                            die(i)
                            return
                        }
                    }
                }
                if !found {
                    controlCh.Out<-Message{Tag: "congestion", Test: true}
                } else {
                    controlCh.Out<-Message{Tag: "congestion", Test: false}
                }
            case "die":
                fmt.Println("injector: kill-die")
                for _, ch := range cellChs {
                    ch.Out<-Message{Tag: "die"}
                }
                fmt.Println("injector: kill-die end")
                return //die
            default:
            }
        default://check inputs from cells
            for i, ch := range cellChs {
                select {
                case msg =<-ch.In:
                    switch msg.Tag {
                    case "occupy": //accept all pedestrians. They are now done.
                        ch.Out<-Message{Tag: "occupy", Test: true}
                    case "die":
                        fmt.Println("injector: die")
                        die(i)
                        return //no more cells, just die
                    case "done":
                        ch.Out<-Message{Tag: "done"}
                    case "drunkMove":
                        //skip
                    default://error, should not happen
                        fmt.Println("injector eror")
                    }
                default://skip
                }
            }
        }
    }
}



func control(injectChs, cellChs []Bichannel, initInjectRate, maxTimeSteps int){
    var s string
    var msg Message
    inputCh := make(chan string)
    inputQuitCh := make(chan string)
    injectRate := initInjectRate //Init should be 4
    injectCounter := 0
    pedestrainCount := 0
    pause := false


    userInput := func(c, quit chan string) {//TODO how to close, fmt.Scan blocks....
        var s string
        for {
            fmt.Scan(&s)
            select {
            case <-quit:
                return
            case c<-s: //skip
            /*default: //skip
                c<-s*/
            }
        }
    }

    userInputHandler := func(s string) bool {
        switch s {
        case "exit":
            fmt.Println("Terminating")
            injectChs[0].Out<-Message{Tag: "die"} //tell first injector to die.
            for i, ch := range injectChs { //wait for injectors to die.
                if i>0 { //first injector die silent
                    msg =<-ch.In
                }
            }
            //when last injector is dead, all cells are already dead too.
            fmt.Println("Enter any text to exit")  
            inputQuitCh<-"" //this is the ONLY way to kill a thread reading stdin in go (making a user write)... reading stdin blocks.
            return true //terminate
        case "pause":
            fmt.Println("pausing")
            pause = true
        case "resume":
            fmt.Println("resume")
            pause = false
        case "rate":
            s =<-inputCh
            i, _ := strconv.Atoi(s)
            if 1 <= i && i <= 10 {
                injectRate = i
                fmt.Println("Inject rate changed to", i)
            } else {
                fmt.Println("Given inject rate not between 1 and 10")
            }
        case "addDrunk": //at drunk at given position
            s =<-inputCh
            x, _ := strconv.Atoi(s)
            s =<-inputCh
            y, _ := strconv.Atoi(s)
            //TODO
        default:
            fmt.Println("Unknown command")
        }
        return false
    }

    go userInput(inputCh, inputQuitCh) //thread to handle user input
    
    for i:=0; i < maxTimeSteps; i++ {
        fmt.Println("Controller: step number", i)
        
        //before next timestep
        
        for _, c :=range cellChs { //then nextstep cells
            c.Out<-Message{Tag: "next"}
        }
        injectCounter++
        if injectCounter >= injectRate { //inject time!
             //one new pedestrian per injector
            for _, c := range injectChs {
                c.Out<-Message{Tag: "inject", Id: pedestrainCount}
                pedestrainCount++
            }
        }

        if injectCounter >= injectRate {
            //Check for congestion
            for _, c := range injectChs {
                msg =<-c.In
                if msg.Tag == "congestion" && msg.Test{
                    fmt.Println("Congestion! Unable to inject new pedestrian at one end.")
                }
            }
            injectCounter = 0
        }

        //listen on inputs from pedestrians and user
        select {
        case s = <-inputCh: //TODO: this is only time user input is registered!
            if userInputHandler(s) {
                fmt.Println("Controller exiting")
                return
            } 
            for pause { //wait until unpause
                fmt.Println("Controller: In pause section")
                s =<-inputCh
                if userInputHandler(s) {
                    fmt.Println("Controller exiting")
                    return
                }
            }
        default: //skip
        }
        for _, c := range cellChs { 
            msg=<-c.In
            if msg.Test {
                fmt.Println("Pedestrian", msg.Pedestrian.Id, "is at position:", msg.Pos)
            }
        }
    }
    userInputHandler("exit") //we are done.
}

/*
    Hardcoded to create a minimum board, as described in assignment.
    Returns bichannels to injecters (end of sidewalk)
*/
func setupMinSideWalk(length, width int) ([]Bichannel, []Bichannel) {
    var cellToInjectorChs [][]Bichannel
    numInjectors := 2
    injectorControlChs := make([]Bichannel, numInjectors)
    cellControlChs := make([]Bichannel, width*length)
    controlCellChs := make([]Bichannel, width*length)
    for i := range cellControlChs {
        ch1, ch2 := makeNewBichannel()
        cellControlChs[i] = ch1
        controlCellChs[i] = ch2
    }
    
    //make injectors, hardcoded to 2.
    injectorChs := make([]Bichannel, width)
    cellToInjectorChs = make([][]Bichannel, numInjectors)
    cellToInjectorChs[0] = make([]Bichannel, width)
    cellToInjectorChs[1] = make([]Bichannel, width)
    for j:= range cellToInjectorChs[0] {
        ch1, ch2 := makeNewBichannel()
        cellToInjectorChs[0][j] = ch1
        injectorChs[j] = ch2
    }
    ch1, ch2 := makeNewBichannel() //control channels
    go injector(injectorChs[:], ch1, E)
    injectorControlChs[0] = ch2

    injectorChs = make([]Bichannel, width)
    for j:= range cellToInjectorChs[1] {
        ch1, ch2 := makeNewBichannel()
        cellToInjectorChs[1][j] = ch1
        injectorChs[j] = ch2
    }
    ch1, ch2 = makeNewBichannel()
    go injector(injectorChs[:], ch1, W)
    injectorControlChs[1] = ch2
    
    //make cells
    i := 0
    westCellRowChs := make([]Bichannel, width)
    copy(westCellRowChs, cellToInjectorChs[0]) //TODO might be cellToInjectorChs[0][:]
    var southCh, northCh, eastCh, westCh Bichannel
    var neighbors [4]Bichannel
    for x:=0; x < length-1; x++ { //do all rows but the last 
        for y:=0; y < width; y++ {
            neighbors[S] = southCh
            if y < width-1 {
                northCh, southCh = makeNewBichannel()
            } else {
                northCh = Bichannel{}
                southCh = Bichannel{}
            }
            neighbors[N] = northCh
            neighbors[W] = westCellRowChs[y]
            eastCh, westCh = makeNewBichannel()
            westCellRowChs[y] = westCh
            neighbors[E] = eastCh
            pos := Position{x,y}
            controlCh := cellControlChs[i]
            go cell(pos, neighbors, controlCh)
            neighbors = [4]Bichannel{}
            i++
        }
    }
    for y:=0; y < width; y++ { //do last row with special west channel to injector
        neighbors[S] = southCh
        if y < width-1 {
            northCh, southCh = makeNewBichannel()
        } else {
            northCh = Bichannel{}
            southCh = Bichannel{}
        }
        neighbors[N] = northCh
        neighbors[W] = westCellRowChs[y]
        neighbors[E] = cellToInjectorChs[1][y]
        pos := Position{length-1,y}
        controlCh := cellControlChs[i]
        go cell(pos, neighbors, controlCh)
        i++
    }
    
    return injectorControlChs[:], controlCellChs[:]
}

func main(){
    length := 2
    width := 2
    initInjectRate := 4
    maxTimeSteps := 20
    injectChs, controlCellChs := setupMinSideWalk(length, width)

    control(injectChs, controlCellChs, initInjectRate, maxTimeSteps)
}