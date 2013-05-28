package state

import "sort"

const Columns = 3 * 4

type State []int

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

func (s State) GetRepr() [Columns]int {
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

func (s State) Clone() State {
    newstate := make(State, len(s))
    copy(newstate, s)
    return newstate
}

func (s State) Move(movespec uint) State {
    news := s.Clone()
    for pawn, _ := range s {
        if ((1<<uint(pawn)) & movespec) != 0 {
            news[pawn] += 1
        }
    }
    return news
}

func (s State) CheckMove(movespec uint) bool {
    for pawn, pawnValue := range s {
        if ((1<<uint(pawn)) & movespec) != 0 {
            if pawnValue == -1 {
                return false
            }
        }
    }
    return true
}

func (s State) CheckKill(killspec uint) bool {
    for pawn, pawnValue := range s {
        if (((1<<uint(pawn)) & killspec) != 0) && pawnValue == -1 {
            return false
        }
    }
    return true
}

func (s State) Kill(killspec uint) State {
    news := s.Clone()
    for pawn, _ := range s {
        if ((1<<uint(pawn)) & killspec) != 0 {
            news[pawn] = -1
        }
    }
    return news;
}

func (s State) Max() int {
    best := -1
    for pawn := uint(0); pawn < uint(len(s)); pawn++ {
        best = max(s[pawn], best)
    }
    return best;
}

func (s State) Dead() bool {
    return s.Max() == -1
}
