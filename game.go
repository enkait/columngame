package main

import "sync"
import "fmt"
import "sort"
import "flag"
import "os"
import "log"
import "time"
import "runtime/pprof"

const MaxDepth = -1
const Columns = 3 * 4

var execSem chan bool = make(chan bool, 100)

type state []int

const debug = false

func min(a int, b int) int {
    if a < b {
        return a
    }
    return b
}

func max(a int, b int) int {
    if a > b {
        return a
    }
    return b
}

type reprtype [][]int

func (r reprtype) Len() int {
    return len(r)
}

func (r reprtype) Less(i, j int) bool {
    return Compare(r[i], r[j])
}

func (r reprtype) Swap(i, j int) {
    r[i], r[j] = r[j], r[i]
}

func Compare(a, b []int) bool {
    length := min(len(a), len(b))
    for i := 0; i < length; i++ {
        if a[i] != b[i] {
            return a[i] < b[i]
        }
    }
    return len(a) < len(b);
}

func (s state) GetRepr() [Columns]int {
    repr := [][]int{}

    temp := make([]int, 0, 3)
    for _, pawnValue := range s {
        temp = append(temp, pawnValue)
        if len(temp) == 3 {
            sort.Sort(sort.IntSlice(temp))
            repr = append(repr, temp)
            temp = make([]int, 0, 3)
        }
    }
    sort.Sort(reprtype(repr))

    result := [Columns]int{};
    for col, colValue := range repr {
        for element, elementValue := range colValue {
            result[col * 3 + element] = elementValue
        }
    }
    return result
}

/*
func (s state) String() string {
    repr := []string{}

    temp := make([]int, 0, 3)
    for _, pawnValue := range s {
        temp = append(temp, pawnValue)
        if len(temp) == 3 {
            sort.Sort(sort.IntSlice(temp))
            buf := bytes.Buffer{}
            for _, element := range temp {
                fmt.Fprintf(&buf, "%d,", element)
            }
            repr = append(repr, buf.String())
            temp = temp[:0]
        }
    }
    sort.Sort(sort.StringSlice(repr))
    result := bytes.Buffer{}
    for _, element := range repr {
        fmt.Fprintf(&result, "%s-", element)
    }
    fmt.Fprint(&result, ";")
    return result.String()
}
*/

func (s state) Clone() state {
    newstate := make(state, len(s))
    copy(newstate, s)
    return newstate
}

func (s state) Move(movespec uint) state {
    news := s.Clone()
    for pawn, _ := range s {
        if ((1<<uint(pawn)) & movespec) != 0 {
            news[pawn] += 1
        }
    }
    return news
}

func (s state) CheckMove(movespec uint) bool {
    for pawn, pawnValue := range s {
        if ((1<<uint(pawn)) & movespec) != 0 {
            if pawnValue == -1 {
                return false
            }
        }
    }
    return true
}

func (s state) CheckKill(killspec uint) bool {
    for pawn, pawnValue := range s {
        if (((1<<uint(pawn)) & killspec) != 0) && pawnValue == -1 {
            return false
        }
    }
    return true
}

func (s state) Kill(killspec uint) state {
    news := s.Clone()
    for pawn, _ := range s {
        if ((1<<uint(pawn)) & killspec) != 0 {
            news[pawn] = -1
        }
    }
    return news;
}

func (s state) Max() int {
    best := -1
    for pawn := uint(0); pawn < uint(len(s)); pawn++ {
        best = max(s[pawn], best)
    }
    return best;
}

func (s state) Dead() bool {
    return s.Max() == -1
}

var M map[[Columns]int]int = map[[Columns]int]int{}
var Mmutex sync.RWMutex

func killable(kill uint, movement uint) bool {
    return ((movement >> (3 * kill)) & 7) != 0
}

func f(s state, depth int, returnchan chan int) {
    //fmt.Println(s.String())
    Mmutex.RLock()
    if val, ok := M[s.GetRepr()]; ok {
        Mmutex.RUnlock()
        returnchan <- val
        return
    }
    Mmutex.RUnlock()

    realf := func() {
        if s.Dead() {
            //fmt.Println("pushing 0")
            returnchan <- 0
            //fmt.Println("pushed 0")
            return
        }
        masklen := uint(len(s))
        movementcodes := (uint(1))<<masklen

        takemin := func(resultchannel chan int, inputchannel chan int, taskcount int) {
            globalmin := 1000000000
            for task := 0; task < taskcount; task++ {
                //fmt.Println("wating on task: ", task, " of ", taskcount)
                result := <-inputchannel
                //fmt.Println("running on task: ", task, " of ", taskcount)
                globalmin = min(globalmin, result)
            }
            //fmt.Println("pushing globalmin", globalmin)
            resultchannel <- globalmin
            //fmt.Println("pushed globalmin", globalmin)
        }

        highestresult := 0

        globalresultchannel := make(chan int, movementcodes)
        movements := 0
        for movement := uint(1); movement < movementcodes; movement++ {
            if s.CheckMove(movement) {
                news := s.Move(movement)
                //fmt.Println(movement, "movement", s.String(), news.String())
                highestresult = max(news.Max(), highestresult)
                resultchannel := make(chan int, len(s))
                taskcount := 0
                for kill := uint(0); kill < uint(len(s)); kill++ {
                    killspec := (7 << (3 * kill)) & movement
                    if killspec != 0 && news.CheckKill(killspec) {
                        newkilleds := news.Kill(killspec)
                        //fmt.Println(killspec, "kill", news.String(), newkilleds.String())
                        f(newkilleds, depth+1, resultchannel)
                        taskcount += 1
                    }
                }
                if depth > MaxDepth {
                    takemin(globalresultchannel, resultchannel, taskcount)
                } else {
                    //fmt.Println("spawning")
                    go takemin(globalresultchannel, resultchannel, taskcount)
                }
                movements += 1
            }
        }

        for task := 0; task < movements; task++ {
            //fmt.Println("wating on movement: ", task, " of ", movements)
            result := <-globalresultchannel
            //fmt.Println("running on movement: ", task, " of ", movements)
            highestresult = max(highestresult, result)
        }

        Mmutex.Lock()
        M[s.GetRepr()] = highestresult
        Mmutex.Unlock()
        //fmt.Println("pushing highestresult", highestresult)
        returnchan <- highestresult
        //fmt.Println("pushed highestresult", highestresult)
    }
    if depth > MaxDepth {
        realf()
    } else {
        //fmt.Println("spawning")
        _ = <-execSem
        go func() {
            realf()
            execSem <- false
        }();
    }
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to this file")

func main() {
    flag.Parse()
    if *cpuprofile != "" {
        fmt.Println("profiling")
        f, err := os.Create(*cpuprofile)
        if err != nil {
            log.Fatal(err)
        }
        pprof.StartCPUProfile(f)
        defer pprof.StopCPUProfile()
    }
    fmt.Println("running")
    for _ = range [20]struct{}{} {
        execSem <- false
    }
    finished := make(chan struct{})
    reporter := func() {
        for {
            time.Sleep(10000 * time.Millisecond)
            panic("bla")
            fmt.Println("getting lock")
            Mmutex.RLock()
            fmt.Println(len(M))
            Mmutex.RUnlock()
            fmt.Println("released lock")
            select {
            case _ = <-finished:
                break
            default:
            }
        }
    }
    go reporter();
    result := make(chan int, 1)
    startstate := state{0,0,0,0,0,0,0,0,0,0,0,0}
    fmt.Println(startstate.GetRepr())
    movestate := startstate.Move(69)
    fmt.Println(movestate.GetRepr())
    killstate := movestate.Kill(31)
    fmt.Println(killstate.GetRepr())
    f(startstate, 0, result)
    val := <-result
    fmt.Println(val)
    finished <- struct{}{}
}
