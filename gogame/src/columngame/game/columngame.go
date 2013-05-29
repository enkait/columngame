package main

import "fmt"
import "flag"
import "os"
import "log"
import "time"
import "runtime/pprof"
import "columngame/state"
import "columngame/lookup"

const MaxDepth = 0
const Columns = 3 * 4
const KnownLimit = 6

const debug = false

var execSem chan bool = make(chan bool, 100)

var l lookup.Lookup = lookup.GetLookup()

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

func killable(kill uint, movement uint) bool {
    return ((movement >> (3 * kill)) & 7) != 0
}

func f(s state.State, depth int, returnchan chan lookup.Payload) {
    if (s.Max() + 1 == KnownLimit) {
        returnchan <- lookup.Payload{KnownLimit}
        return
    }

    if val, ok := l.Lookup(s); ok {
        returnchan <- val
    }

    onlyonce := l.GetOnce(s)

    realf := func() {
        if s.Dead() {
            l.Store(s, lookup.Payload{0})
            return
        }
        masklen := uint(len(s))
        movementcodes := (uint(1))<<masklen

        takemin := func(resultchannel chan lookup.Payload, inputchannel chan lookup.Payload, taskcount int) {
            globalmin := 1000000000
            for task := 0; task < taskcount; task++ {
                result := <-inputchannel
                globalmin = min(globalmin, result.Result)
            }
            resultchannel <- lookup.Payload{globalmin}
        }

        highestresult := 0

        globalresultchannel := make(chan lookup.Payload, movementcodes)
        movements := 0
        for movement := uint(1); movement < movementcodes; movement++ {
            if s.CheckMove(movement) {
                news := s.Move(movement)
                highestresult = max(news.Max(), highestresult)
                resultchannel := make(chan lookup.Payload, len(s))
                taskcount := 0
                for kill := uint(0); kill < uint(len(s)); kill++ {
                    killspec := (7 << (3 * kill)) & movement
                    if killspec != 0 && news.CheckKill(killspec) {
                        newkilleds := news.Kill(killspec)
                        f(newkilleds, depth+1, resultchannel)
                        taskcount += 1
                    }
                }
                if depth > MaxDepth {
                    takemin(globalresultchannel, resultchannel, taskcount)
                } else {
                    go takemin(globalresultchannel, resultchannel, taskcount)
                }
                movements += 1
            }
        }

        for task := 0; task < movements; task++ {
            result := <-globalresultchannel
            highestresult = max(highestresult, result.Result)
        }

        l.Store(s, lookup.Payload{highestresult})
    }

    if depth > MaxDepth {
        onlyonce.Do(realf)

        if val, ok := l.Lookup(s); ok {
            returnchan <- val
        } else {
            panic("something went terribly wrong")
        }
    } else {
        _ = <-execSem
        go func() {
            realf()
            if val, ok := l.Lookup(s); ok {
                returnchan <- val
            } else {
                panic("something went terribly wrong")
            }
            execSem <- false
        }();
    }

}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to this file")
var load = flag.String("load", "", "read map from file")

func load_map() {
    fmt.Println("loading from", *load)
    f, err := os.Open(*load)
    if err != nil {
        panic(err)
    }
    defer f.Close()

    l.Load(f)
}

func dump_map_thread() {
    for {
        filename := ""
        fmt.Scanf("%s", &filename)
        dump_map(filename)
    }
}

func dump_map(filename string) {
    fmt.Println("dumping to", filename)
    f, err := os.Create(filename)
    if err != nil {
        panic(err)
    }
    defer f.Close()

    l.Dump(f)
}

var finished chan struct{}

func reporter() {
    counter := 0
    for {
        counter += 1
        fmt.Println("getting lock")
        newdata, cacheddata := l.GetStats()
        fmt.Println("Calculated:", newdata + cacheddata, "states, new:", newdata, ", old:", cacheddata)
        if counter == 10 {
            counter = 0
            dump_map("defaultdump")
            l.Cache()
        }
        fmt.Println("released lock")
        select {
        case _ = <-finished:
            break
        default:
        }
        //runtime.gosched()
        time.Sleep(1000 * time.Millisecond)
    }
}

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

    finished = make(chan struct{})
    if *load != "" {
        load_map()
    }

    go reporter();
    go dump_map_thread();

    for _ = range [100]struct{}{} {
        execSem <- false
    }
    result := make(chan lookup.Payload, 1)
    startstate := state.State{0,0,0,0,0,0,0,0,0,0,0,0}
    f(startstate, -1, result)
    val := <-result
    fmt.Println(val.Result)
    finished <- struct{}{}
    dump_map("finished")
}
