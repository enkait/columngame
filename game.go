package main

import "bytes"
import "sync"
import "fmt"

const MaxDepth = -1

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

func (s state) String() string {
	buf := bytes.Buffer{}
	for _, t := range s {
        fmt.Fprintf(&buf, "%d,", t)
	}
    fmt.Fprint(&buf, ";")
	return buf.String()
}

func (s state) Clone() state {
    newstate := make(state, len(s))
    copy(newstate, s)
    return newstate
}

func (s state) Move(movespec uint) state {
    news := s.Clone()
    for pawn := uint(0); pawn < uint(len(s)); pawn++ {
        if ((1<<pawn) & movespec) != 0 {
            news[pawn] += 1
        }
    }
    return news
}

func (s state) CheckMove(movespec uint) bool {
    for pawn := uint(0); pawn < uint(len(s)); pawn++ {
        if ((1<<pawn) & movespec) != 0 {
            if s[pawn] == -1 {
                return false
            }
        }
    }
    return true
}

func (s state) CheckKill(killspec uint) bool {
    for pawn := uint(0); pawn < uint(len(s)); pawn++ {
        if (((1<<pawn) & killspec) != 0) && s[pawn] == -1 {
            return false
        }
    }
    return true
}

func (s state) Kill(killspec uint) state {
    news := s.Clone()
    for pawn := uint(0); pawn < uint(len(s)); pawn++ {
        if ((1<<pawn) & killspec) != 0 {
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

var M map[string]int = map[string]int{}
var Mmutex sync.RWMutex

func killable(kill uint, movement uint) bool {
    return ((movement >> (3 * kill)) & 7) != 0
}

func f(s state, depth int, returnchan chan int) {
    //fmt.Println(s.String())
    Mmutex.RLock()
    if val, ok := M[s.String()]; ok {
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
        M[s.String()] = highestresult
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

func main() {
    fmt.Println("running")
    for _ = range [20]struct{}{} {
        execSem <- false
    }
    result := make(chan int, 1)
    startstate := state{0,0,0,0,0,0,0,0,0}
    f(startstate, 0, result)
    val := <-result
    fmt.Println(val)
}