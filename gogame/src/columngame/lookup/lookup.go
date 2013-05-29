package lookup

import "sync"
//import "fmt"
//import "os"
//import "log"
//import "time"
//import "runtime/pprof"
//import "bufio"
import "fmt"
import "columngame/state"
import "io"
import "bufio"
import "strings"
import "strconv"

const MaxDepth = 0
const Columns = 3 * 4
const KnownLimit = 6

const debug = false

type Payload struct {
    result int
}

func Deserialize(s string) Payload {
    value64bit, _ := strconv.ParseInt(s, 0, 32)
    valuerepr := int(value64bit)
    return Payload{valuerepr}
}

func (p Payload) GetRepr() int {
    return p.result
}

type Lookup struct {
    M map[[Columns]int]Payload
    readM map[[Columns]int]Payload
    Monce map[[Columns]int]*sync.Once
    Mmutex sync.RWMutex
    readMmutex sync.RWMutex
}

func (l Lookup) ReadOnlyLookup(s state.State) (Payload, bool) {
    l.readMmutex.RLock()
    val, ok := l.readM[s.GetRepr()]
    l.readMmutex.RUnlock()
    return val, ok
}

func (l Lookup) WriteableLookup(s state.State) (Payload, bool) {
    l.Mmutex.RLock()
    val, ok := l.M[s.GetRepr()]
    l.Mmutex.RUnlock()
    return val, ok
}

func (l Lookup) Lookup(s state.State) (Payload, bool) {
    if val, ok := l.ReadOnlyLookup(s); ok {
        return val, ok
    }
    if val, ok := l.WriteableLookup(s); ok {
        return val, ok
    }
    return Payload{}, false
}

func (l Lookup) GetOnce(s state.State) *sync.Once {
    l.Mmutex.RLock()
    onlyonce, ok := l.Monce[s.GetRepr()]
    l.Mmutex.RUnlock()
    if !ok {
        l.Mmutex.Lock()
        onlyonce, ok = l.Monce[s.GetRepr()] // might have changed since last time
        if !ok {
            onlyonce = new(sync.Once)
            l.Monce[s.GetRepr()] = onlyonce
        }
        l.Mmutex.Unlock()
    }
    return onlyonce
}

func (l Lookup) Cache() {
    fmt.Println("storing results")
    l.readMmutex.Lock()
    l.Mmutex.Lock()
    fmt.Println("got mutexes, storing M in readM")
    for key, value := range l.M {
        l.readM[key] = value
    }
    l.M = map[[Columns]int]Payload{}
    fmt.Println("releasing mutexes")
    l.Mmutex.Unlock()
    l.readMmutex.Unlock()
    fmt.Println("released mutexes")
}

func (l Lookup) ParseIntArray(s string) [Columns]int {
    s = strings.TrimSpace(s)
    s = strings.TrimRight(s, ";[]")
    s = strings.TrimLeft(s, ";[]")
    stab := strings.Split(s, " ")

    srepr := [Columns]int{}

    for index, indexvalue := range stab {
        value64bit, _ := strconv.ParseInt(indexvalue, 0, 32)
        srepr[index] = int(value64bit)
    }
    return srepr
}

func (l Lookup) Load(input io.Reader) {
    r := bufio.NewReader(input)

    l.Mmutex.Lock()
    for {
        key, err := r.ReadString(byte(';'))
        if err != nil {
            break
        }
        value, err := r.ReadString(byte(';'))
        if err != nil {
            break
        }

        keyrepr := l.ParseIntArray(key)
        l.readM[keyrepr] = Deserialize(value)
    }
    l.Mmutex.Unlock()
    fmt.Println("loaded")
}

func (l Lookup) Dump(output io.Writer) {
    l.Mmutex.RLock()
    fmt.Println("dumping, got lock")
    for key, value := range l.M {
        fmt.Fprint(output, key);
        fmt.Fprint(output, ";");
        fmt.Fprint(output, value);
        fmt.Fprintln(output, ";");
    }
    fmt.Println("dumping readonly")
    for key, value := range l.readM {
        fmt.Fprint(output, key);
        fmt.Fprint(output, ";");
        fmt.Fprint(output, value);
        fmt.Fprintln(output, ";");
    }
    fmt.Println("dumping, releasing lock")
    l.Mmutex.RUnlock()
    fmt.Println("dumped")
}
