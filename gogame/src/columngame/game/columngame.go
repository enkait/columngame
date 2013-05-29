package main

import "sync"
import "fmt"
import "flag"
import "os"
import "log"
import "time"
import "runtime/pprof"
import "bufio"
import "strings"
import "strconv"
import "columngame/state"

const MaxDepth = 0
const Columns = 3 * 4
const KnownLimit = 6

const debug = false

var execSem chan bool = make(chan bool, 100)

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

var M map[[Columns]int]int = map[[Columns]int]int{}
var readM map[[Columns]int]int = map[[Columns]int]int{}
var Monce map[[Columns]int]*sync.Once = map[[Columns]int]*sync.Once{}
var fincounter = 0
var Mmutex sync.RWMutex
var readMmutex sync.RWMutex

func killable(kill uint, movement uint) bool {
    return ((movement >> (3 * kill)) & 7) != 0
}

func f(s state.State, depth int, returnchan chan int) {
    if (s.Max() + 1 == KnownLimit) {
        returnchan <- KnownLimit
        return
    }

    readMmutex.RLock()
    if val, ok := readM[s.GetRepr()]; ok {
        readMmutex.RUnlock()
        returnchan <- val
        if depth <= MaxDepth {
            Mmutex.Lock()
            fincounter += 1
            Mmutex.Unlock()
        }
        return
    }
    readMmutex.RUnlock()

    Mmutex.RLock()
    if val, ok := M[s.GetRepr()]; ok {
        Mmutex.RUnlock()
        returnchan <- val
        return
    }
    Mmutex.RUnlock()

    Mmutex.RLock()
    onlyonce, ok := Monce[s.GetRepr()]
    Mmutex.RUnlock()
    if !ok {
        Mmutex.Lock()
        onlyonce, ok = Monce[s.GetRepr()] // might have changed since last time
        if !ok {
            onlyonce = new(sync.Once)
            Monce[s.GetRepr()] = onlyonce
        }
        Mmutex.Unlock()
    }

    realf := func() {
        if s.Dead() {
            Mmutex.Lock()
            M[s.GetRepr()] = 0
            Mmutex.Unlock()
            return
        }
        masklen := uint(len(s))
        movementcodes := (uint(1))<<masklen

        takemin := func(resultchannel chan int, inputchannel chan int, taskcount int) {
            globalmin := 1000000000
            for task := 0; task < taskcount; task++ {
                result := <-inputchannel
                globalmin = min(globalmin, result)
            }
            resultchannel <- globalmin
        }

        highestresult := 0

        globalresultchannel := make(chan int, movementcodes)
        movements := 0
        for movement := uint(1); movement < movementcodes; movement++ {
            if s.CheckMove(movement) {
                news := s.Move(movement)
                highestresult = max(news.Max(), highestresult)
                resultchannel := make(chan int, len(s))
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
            highestresult = max(highestresult, result)
        }

        Mmutex.Lock()
        M[s.GetRepr()] = highestresult
        Mmutex.Unlock()
    }

    if depth > MaxDepth {
        onlyonce.Do(realf)

        Mmutex.RLock()
        if val, ok := M[s.GetRepr()]; ok {
            Mmutex.RUnlock()
            returnchan <- val
            return
        } else if val, ok := readM[s.GetRepr()]; ok { //perhaps we already copied the result
            Mmutex.RUnlock()
            returnchan <- val
            return
        } else {
            panic("something went terribly wrong")
        }
    } else {
        _ = <-execSem
        go func() {
            realf()
            Mmutex.RLock()
            if val, ok := M[s.GetRepr()]; ok {
                Mmutex.RUnlock()
                returnchan <- val
            } else if val, ok := readM[s.GetRepr()]; ok { // as above, maybe we already copied this to Mread
                Mmutex.RUnlock()
                returnchan <- val
            } else {
                panic("something went terribly wrong")
            }
            execSem <- false
            Mmutex.Lock()
            fincounter += 1
            Mmutex.Unlock()
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

    r := bufio.NewReader(f)

    Mmutex.Lock()
    for {
        key, err := r.ReadString(byte(';'))
        if err != nil {
            break
        }
        value, err := r.ReadString(byte(';'))
        if err != nil {
            break
        }
        key = strings.TrimSpace(key)
        key = strings.TrimRight(key, ";[]")
        key = strings.TrimLeft(key, ";[]")
        value = strings.TrimSpace(value)
        value = strings.TrimRight(value, ";")
        keytab := strings.Split(key, " ")

        keyrepr := [Columns]int{}

        for index, indexvalue := range keytab {
            value64bit, _ := strconv.ParseInt(indexvalue, 0, 32)
            keyrepr[index] = int(value64bit)
        }
        value64bit, _ := strconv.ParseInt(value, 0, 32)
        valuerepr := int(value64bit)

        readM[keyrepr] = valuerepr
    }
    Mmutex.Unlock()
    fmt.Println("loaded")
}

func dump_map_thread() {
    for {
        filename := ""
        fmt.Scanf("%s", &filename)
        dump_map(filename)
    }
}

func store_results() {
    fmt.Println("storing results")
    readMmutex.Lock()
    Mmutex.Lock()
    fmt.Println("got mutexes, storing M in readM")
    for key, value := range M {
        readM[key] = value
    }
    M = map[[Columns]int]int{}
    fmt.Println("releasing mutexes")
    Mmutex.Unlock()
    readMmutex.Unlock()
    fmt.Println("released mutexes")
}

func dump_map(filename string) {
    fmt.Println("dumping to", filename)
    f, err := os.Create(filename)
    if err != nil {
        panic(err)
    }
    defer f.Close()

    Mmutex.RLock()
    fmt.Println("dumping, got lock")
    for key, value := range M {
        fmt.Fprint(f, key);
        fmt.Fprint(f, ";");
        fmt.Fprint(f, value);
        fmt.Fprintln(f, ";");
    }
    fmt.Println("dumping readonly")
    for key, value := range readM {
        fmt.Fprint(f, key);
        fmt.Fprint(f, ";");
        fmt.Fprint(f, value);
        fmt.Fprintln(f, ";");
    }
    fmt.Println("dumping, releasing lock")
    Mmutex.RUnlock()
    fmt.Println("dumped")
}

var finished chan struct{}

func reporter() {
    counter := 0
    for {
        counter += 1
        fmt.Println("getting lock")
        Mmutex.RLock()
        fmt.Println("Calculated:", len(M) + len(readM), "states, new:", len(M), ", old:", len(readM))
        fmt.Println("Including:", fincounter, "starting states (num threads terminated)")
        Mmutex.RUnlock()
        if counter == 10 {
            counter = 0
            dump_map("defaultdump")
            store_results()
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
    result := make(chan int, 1)
    startstate := state.State{0,0,0,0,0,0,0,0,0,0,0,0}
    f(startstate, -1, result)
    val := <-result
    fmt.Println(val)
    finished <- struct{}{}
    dump_map("finished")
}
