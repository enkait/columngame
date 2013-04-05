from itertools import *
import copy

def subsets(x): return [[y for j, y in enumerate(set(x)) if (i >> j) & 1] for i in range(0, 2**len(set(x)))]

mem = {}

def getkey(state):
    #print state
    return tuple(sorted(map(tuple, map(sorted, state))))

def f(state, m):
    if getkey(state) in mem: return mem[getkey(state)]
    L = [subsets(range(len(state[i]))) for i in range(len(state))]
    moves = product(*L)
    moves.next()

    result = (max([max(0, 0, *s) for s in state]) + 1, m+1)
    for move in moves:
        results = []
        for killpos, kill in enumerate(move):
            if len(kill) == 0: continue
            newstate = copy.deepcopy(state)
            for pos, submove in enumerate(move):
                if pos == killpos:
                    for deleted, elem in enumerate(submove):
                        del newstate[pos][elem - deleted]
                else:
                    for elem in submove:
                        newstate[pos][elem] += 1
            results.append(f(newstate, m+1))
        resultmove = min(results)
        result = max(result, resultmove)
    mem[getkey(state)] = result 
    return result

def p(state):
    print state, f(state, 0)

p([[0,0,0],[0,0,0],[0,0,0],[0,0,0],[0,0,0]])
