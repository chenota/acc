package ssa

import (
	"testing"

	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	r1 = NewReg(register.RegB)
	r2 = NewReg(register.Reg12)
	r3 = NewReg(register.Reg13)
)

func TestEdgeMoves_PhiAlreadyHoldingItsOperandMovesNothing(t *testing.T) {
	pred := requireEdge(t, phiFrom{r1, r1})

	assert.Empty(t, pred.EdgeMoves())
}

func TestEdgeMoves_PhiWithoutALocationMovesNothing(t *testing.T) {
	pred := requireEdge(t, phiFrom{Location{}, r1})

	assert.Empty(t, pred.EdgeMoves())
}

func TestEdgeMoves_ChainReadsEachSourceBeforeItIsOverwritten(t *testing.T) {
	// listed so that running the moves in phi order would overwrite r2 before r1 reads it
	pred := requireEdge(t, phiFrom{r2, r3}, phiFrom{r1, r2})

	moves := pred.EdgeMoves()
	require.Len(t, moves, 2)
	assert.Equal(t, MoveCopy, moves[0].Kind)
	assert.Equal(t, MoveCopy, moves[1].Kind)

	held := runMoves(moves)
	assert.Equal(t, r2, held(r1))
	assert.Equal(t, r3, held(r2))
}

func TestEdgeMoves_OneSourceFansOutToEveryPhiReadingIt(t *testing.T) {
	pred := requireEdge(t, phiFrom{r1, r3}, phiFrom{r2, r3})

	held := runMoves(pred.EdgeMoves())
	assert.Equal(t, r3, held(r1))
	assert.Equal(t, r3, held(r2))
}

func TestEdgeMoves_SwapIsOneExchange(t *testing.T) {
	pred := requireEdge(t, phiFrom{r1, r2}, phiFrom{r2, r1})

	moves := pred.EdgeMoves()
	require.Len(t, moves, 1)
	assert.Equal(t, MoveSwap, moves[0].Kind)

	held := runMoves(moves)
	assert.Equal(t, r2, held(r1))
	assert.Equal(t, r1, held(r2))
}

func TestEdgeMoves_RotationTakesOneExchangeFewerThanItsLength(t *testing.T) {
	pred := requireEdge(t, phiFrom{r1, r2}, phiFrom{r2, r3}, phiFrom{r3, r1})

	moves := pred.EdgeMoves()
	require.Len(t, moves, 2)
	assert.Equal(t, MoveSwap, moves[0].Kind)
	assert.Equal(t, MoveSwap, moves[1].Kind)

	held := runMoves(moves)
	assert.Equal(t, r2, held(r1))
	assert.Equal(t, r3, held(r2))
	assert.Equal(t, r1, held(r3))
}

func TestEdgeMoves_CopyHangingOffACycleReadsBeforeTheSwap(t *testing.T) {
	// r3 reads r1, which the swap of r1 and r2 is about to change
	pred := requireEdge(t, phiFrom{r1, r2}, phiFrom{r2, r1}, phiFrom{r3, r1})

	held := runMoves(pred.EdgeMoves())
	assert.Equal(t, r2, held(r1))
	assert.Equal(t, r1, held(r2))
	assert.Equal(t, r1, held(r3))
}

func TestEdgeMoves_SourceInMemoryIsCopied(t *testing.T) {
	stack := NewFrame(16)
	pred := requireEdge(t, phiFrom{r1, stack})

	moves := pred.EdgeMoves()
	require.Len(t, moves, 1)
	assert.Equal(t, Move{Kind: MoveCopy, Dst: r1, Src: stack, Type: types.Int()}, moves[0])
}

// phiFrom is a phi living at phi whose operand from the edge under test is held at operand
type phiFrom struct {
	phi     Location
	operand Location
}

// requireEdge builds a predecessor whose one successor has a phi for each of phis, and returns the predecessor.
// The successor has another predecessor listed first, so the operand under test is each phi's second.
func requireEdge(t *testing.T, phis ...phiFrom) *Block {
	t.Helper()

	other := &Block{Id: 0, Kind: BlockPlain}
	pred := &Block{Id: 1, Kind: BlockPlain}
	merge := &Block{Id: 2, Kind: BlockRet, Predecessors: []*Block{other, pred}}
	other.Successors = []*Block{merge}
	pred.Successors = []*Block{merge}

	for i, p := range phis {
		// the other edge's operand already sits where the phi does, so any move comes from pred
		fromOther := &Value{Id: 2 * i, Op: OpCopy, Type: types.Int(), Block: other, Loc: p.phi}
		fromPred := &Value{Id: 2*i + 1, Op: OpCopy, Type: types.Int(), Block: pred, Loc: p.operand}
		phi := &Value{Id: 2*len(phis) + i, Op: OpPhi, Type: types.Int(), Block: merge, Args: []*Value{fromOther, fromPred}, Loc: p.phi}
		merge.Values = append(merge.Values, phi)
	}

	require.Equal(t, len(phis), merge.phiIndex())
	return pred
}

// runMoves plays moves one at a time over locations that each start out holding their own name,
// and reports whose value each location holds at the end
func runMoves(moves []Move) func(Location) Location {
	held := make(map[Location]Location)
	read := func(l Location) Location {
		if v, ok := held[l]; ok {
			return v
		}
		return l
	}

	for _, m := range moves {
		switch m.Kind {
		case MoveCopy:
			held[m.Dst] = read(m.Src)
		case MoveSwap:
			held[m.Dst], held[m.Src] = read(m.Src), read(m.Dst)
		}
	}

	return read
}
