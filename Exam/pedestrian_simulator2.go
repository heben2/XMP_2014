package main

import (
    "fmt"
    "math/rand"
    "time"
    "strconv"
    "runtime"
    "os"
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

func getDirString(d Direction) string {
    switch d {
        case N: 
            return "N"
        case S:
            return "S"
        case E:
            return "E"
        case W:
            return "W"
    }
    return "" //should never happen
}

/*
    A cell is an active part of the automaton.
    Listens on neighbors and controller channels
    If occupied, tries to push pedestrian to next cell at each step.
    Same with drunk.
*/
func cell(pos Position, neighbors [4]Bichannel, controlCh Bichannel){
    rand.Seed(time.Now().UTC().UnixNano()) //seed random generator
    var msg Message
    var pedestrian Pedestrian
    var orthoDirections [2]Direction
    drunk := false
    drunkEffect := false
    occupied := false


    numNeighbors := 0
    for i := range neighbors {
        if neighbors[i] != (Bichannel{}){
            numNeighbors++
        }
    }

    timeout := make(chan bool)
    timeoutQuitCh := make(chan bool)
    go func() {
        for {
            select {
            case <-timeoutQuitCh:
                runtime.Goexit()
            case timeout <- true:
                time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
            }
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

    killNext := func(i int){ //kills cell opposite of i
        var d Direction
        switch i {
        case 0:
            d = S
        case 1:
            d = N
        case 2:
            d = W
        case 3:
            d = E
        default: //should not happen
            fmt.Println("Cell: error when killing")
            return
        }
        if neighbors[d] != (Bichannel{}) {
            neighbors[d].Out<-Message{Tag: "die"}
        }
        timeoutQuitCh<-true
        runtime.Goexit()
    }

    spreadDrunk := func(fromIndex, area int){
        neighborSend := [4]bool{false, false, false, false}
        reset := false
        var count int
        if fromIndex < 0 {
            count = numNeighbors
        } else {
            count = numNeighbors-1
        }

        listenNeighborsDrunk := func(i int){
            for j, ch := range neighbors {
                if j != i {
                    select {
                    case msg =<-ch.In:
                        switch msg.Tag {
                        case "die":
                            neighbors[fromIndex].Out<-Message{Tag: "drunkArea", Test: true} //free fromIndex
                            killNext(j)
                        case "drunkArea":
                            if !msg.Test && msg.DrunkArea-1 > area {
                                neighbors[fromIndex].Out<-Message{Tag: "drunkArea", Test: true} //free fromIndex
                                fromIndex = j //spread new, higher value drunk area
                                area = msg.DrunkArea-1 //restart
                                count = numNeighbors-1
                                neighborSend = [4]bool{false, false, false, false}
                                reset = true
                            } else { //confirm
                                neighbors[j].Out<-Message{Tag: "drunkArea", Test: true}
                            }
                        default: //should not happen
                            fmt.Println("Cell: error in msg tag when spreading drunkness", msg)
                        }
                    default: //skip
                    }
                }
            }
        }

        for count > 0 {
            for j, ch := range neighbors {
                if j != fromIndex && ch != (Bichannel{}) && !neighborSend[j] {
                    select {
                    case ch.Out<-Message{Tag: "drunkArea", DrunkArea: area, Test: false}:
                        success := false
                        for !success {
                            select {
                            case msg =<-ch.In: //get confirmation
                                success = true
                                switch msg.Tag {
                                case "die":
                                    neighbors[fromIndex].Out<-Message{Tag: "drunkArea", Test: true} //free fromIndex
                                    killNext(j)
                                default: 
                                    if !reset {
                                        neighborSend[j] = true
                                        count--
                                    } else {
                                        reset = false
                                    }
                                }
                            case <-timeout:
                                listenNeighborsDrunk(j)
                            }
                        }
                        
                    case <-timeout:
                        listenNeighborsDrunk(-1)
                    }
                }
            }
        }
        if fromIndex >= 0 { //confirm
            neighbors[fromIndex].Out<-Message{Tag: "drunkArea", Test: true}
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
                    case "die"://Must die. Kill next
                        fmt.Println("cell: die")
                        killNext(i)
                    case "drunkMove":
                        if !drunk && msg.Drunk {
                            drunk = true
                            drunkEffect = true
                            c.Out<-Message{Tag: "drunkMove", Test: true}
                        } else {
                            c.Out<-Message{Tag: "drunkMove", Test: false}
                        }
                    case "drunkArea": //spread drunk area
                        drunkEffect = true
                        k := msg.DrunkArea
                        if k > 1 {
                            spreadDrunk(i, k-1)
                        } else {
                            c.Out<-Message{Tag: "drunkArea", Test: true} //confirm
                        }
                    default: //errorfull message tag, should not happen
                         fmt.Println("cell: Error in tag", msg.Tag, "pos:", pos)
                }
            default://skip
            }
        }
    }


    drunkMove := func(){
        i := rand.Intn(5)
        if i < len(neighbors) {
            ch := neighbors[i]
            if ch == (Bichannel{}) { //Check if ch is valid
                drunk = false //drunk goes away
                return
            }
            success := false
            for !success {
                select {
                case ch.Out<-Message{Tag: "drunkMove", Test: false, Drunk: true}:
                    for !success {
                        select{
                        case msg =<-ch.In:
                            if msg.Tag == "drunkMove" && msg.Test {
                                drunk = false
                            }
                            success = true
                        case <-timeout:
                            listenNeighbors()
                        }
                    }
                case <-timeout: //force to listen to neighbors for a while
                    listenNeighbors()
                }
            }
        }
    }

    pedMove := func(ch Bichannel) {
        if ch == (Bichannel{}) { //Check if ch is valid
            return
        }
        success := false
        for !success {
            select {
            case ch.Out<-Message{Tag: "occupy", Pedestrian: pedestrian}: //try to move ped to next cell
                for !success {
                    select{
                    case msg =<-ch.In:
                        if msg.Tag == "occupy" && msg.Test {
                            leave()
                        }
                        success = true
                    case <-timeout:
                        listenNeighbors()
                    }
                }
            case <-timeout: //force to listen to neighbors for a while
                //skip
            }
            listenNeighbors()
        }
    }

    waitNextStep := func(){
        drunkEffect = false //reset drunkeffect. May be added again after done-check if drunk still within range.
        //synchronize cells, drunk effect spread phase
        step := false
        for !step {
            select{
            case controlCh.Out<-Message{Tag:"status", Pos: pos}:
            case msg =<- controlCh.In: //step 1
                if msg.Tag == "drunkMove" {
                    drunk = true
                } else {
                    step = true
                }
            case <-timeout: //Force to listen for eventual die requests
                listenNeighbors()
            }
        }
        
        if drunk { //spread drunk effect
            drunkEffect = true
            spreadDrunk(-1, 3)
        }
        
        //synchronize cells, next step
        step = false
        for !step {
            select{
            case controlCh.Out<-Message{Tag:"status", Pos: pos, Test: occupied, Pedestrian: pedestrian, Drunk: drunk}:
            case msg =<- controlCh.In: //step
                step = true
            case <-timeout: //Force to listen for eventual die requests
                listenNeighbors()
            }
        }
    }

    <- controlCh.In //initial step
    for {
        waitNextStep()
        if occupied { //only listen on control channel if occupied
            if !drunkEffect {
                pedMove(neighbors[pedestrian.Dir])
            }
            if occupied { //prime direction not possible, try orthogonal directions
                i := rand.Intn(2)
                pedMove(neighbors[orthoDirections[i]])
                if occupied { //try other orthogonal direction
                    i++
                    pedMove(neighbors[orthoDirections[i % 2]])
                }
            }
        }
        if drunk {
            drunkMove()
        }
    }
}




/*
    Used at the ends of the side walk. 
    Injects new pedestrians when told by the controller.
    Also kills cells on termination (and terminate itself).
    Dies if all closest cells tells it to.
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
        runtime.Goexit()
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
                runtime.Goexit()
            default:
            }
        default://check inputs from cells, accept everything
            for i, ch := range cellChs {
                select {
                case msg =<-ch.In:
                    switch msg.Tag {
                    case "occupy": //accept all pedestrians. They are now done.
                        ch.Out<-Message{Tag: "occupy", Test: true}
                    case "die":
                        fmt.Println("injector: die")
                        die(i)
                    case "done":
                        ch.Out<-Message{Tag: "done"}
                    case "drunkArea":
                        ch.Out<-Message{Tag: "drunkArea", Test: true}
                    case "drunkMove":
                        ch.Out<-Message{Tag: "drunkMove", Test: true}
                    default://error, should not happen
                        fmt.Println("injector eror")
                    }
                default://skip
                }
            }
        }
    }
}

/*
    Controller of the automaton.
    Interacts with the user.
    Controls the inject rate and tells injector when to inject.
    Injects drunks at given locations.
    Synchronizes cells (making steps).
*/
func control(injectChs []Bichannel, cellChs [][]Bichannel, initInjectRate, maxTimeSteps int){
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
                runtime.Goexit()
            case c<-s:
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
            fmt.Println("resuming")
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
            xMax := len(cellChs)
            yMax := len(cellChs[0])
            if 0 <= x && x < xMax && 0 <= y && y < yMax {
                //add drunk at position.
                cellChs[x][y].Out<-Message{Tag: "drunkMove"}
                //<-cellChs[x][y].In //block until drunk is placed.
            } else {
                fmt.Println("Invalid position given! Must be within automaton, dimension:", xMax, "X", yMax)
            }
            fmt.Println("Drunk added")
        default:
            fmt.Println("Unknown command")
        }
        return false
    }

    checkUser := func() {
        select {
        case s = <-inputCh: //TODO: this is only time user input is registered!
            if userInputHandler(s) {
                fmt.Println("Controller exiting")
                os.Exit(0)
            } 
            for pause { //wait until unpause
                fmt.Println("System paused")
                s =<-inputCh
                if userInputHandler(s) {
                    fmt.Println("Controller exiting")
                    os.Exit(0)
                }
            }
        default: //skip
        }
    }

    go userInput(inputCh, inputQuitCh) //thread to handle user input

    for i:=0; i < maxTimeSteps; i++ {

        fmt.Println("Controller: step number", i)

        //next timestep
        for _, l := range cellChs {
            for _, c := range l { //then nextstep cells
                c.Out<-Message{Tag: "next"}
            }
        }
        //inject phase
        injectCounter++
        if injectCounter >= injectRate { //inject time!
             //one new pedestrian per injector
            for _, c := range injectChs {
                c.Out<-Message{Tag: "inject", Id: pedestrainCount}
                pedestrainCount++
            }
            //Check for congestion
            for _, c := range injectChs {
                msg =<-c.In
                if msg.Tag == "congestion" && msg.Test{
                    fmt.Println("Congestion! Unable to inject new pedestrian at one end.")
                }
            }
            injectCounter = 0
        }
        //listen on inputs from user
        checkUser()
        //Synchronize cells for drunk management
        for _, l := range cellChs {
            for _, c := range l {
                msg=<-c.In
            }
        }
        for _, l := range cellChs {
            for _, c := range l {
                c.Out<-Message{Tag: "syncDrunk"}
            }
        }
        //synchronize cells for report of automaton, before next step
        for _, l := range cellChs {
            for _, c := range l {
                msg=<-c.In
                if msg.Test && msg.Pedestrian != (Pedestrian{}) {
                    dir := getDirString(msg.Pedestrian.Dir)
                    fmt.Println("Pedestrian", msg.Pedestrian.Id, "is at position:", msg.Pos, "going", dir)
                }
                if msg.Drunk {
                    fmt.Println("Drunk is at position:", msg.Pos)
                }
            }
        }
    }
    userInputHandler("exit") //we are done.
}

/*
    Hardcoded to create a minimum board, as described in assignment.
    Returns bichannels to injecters (end of sidewalk)
*/
func setupMinSideWalk(length, width int) ([]Bichannel, [][]Bichannel) {
    var cellToInjectorChs [][]Bichannel
    numInjectors := 2
    injectorControlChs := make([]Bichannel, numInjectors)
    
    controlCellChs := make([][]Bichannel, length)
    for i := range controlCellChs {
        controlCellChs[i] = make([]Bichannel, width)
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
            cellControlch1, ch2 := makeNewBichannel()
            controlCellChs[x][y] = ch2
            go cell(pos, neighbors, cellControlch1)
            neighbors = [4]Bichannel{}
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
        cellControlch1, ch2 := makeNewBichannel()
        controlCellChs[length-1][y] = ch2
        go cell(pos, neighbors, cellControlch1)
    }
    
    return injectorControlChs[:], controlCellChs[:][:]
}

func main(){
    length := 10
    width := 5
    initInjectRate := 4
    maxTimeSteps := 200
    injectChs, controlCellChs := setupMinSideWalk(length, width)

    control(injectChs, controlCellChs, initInjectRate, maxTimeSteps)
}