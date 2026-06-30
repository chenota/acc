package ssa

import (
	"math/big"

	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/types"
)

func buildFunc(n *ir.Node) (*Func, error) {
	if n.Op != ir.OpFunction {
		return nil, diagnostic.NewError(n.Pos, "expected function node")
	}

	f := &Func{
		Name: n.Sym.Name,
	}

	b := &builder{targetFunc: f, vars: make(map[*ir.Sym]*Value)}

	entry := f.newBlock()
	f.Entry = entry
	b.currentBlock = entry

	for _, stmt := range n.List {
		if err := b.genStatement(stmt); err != nil {
			return nil, err
		}
	}

	return f, nil
}

type builder struct {
	targetFunc   *Func
	currentBlock *Block
	vars         map[*ir.Sym]*Value
}

func (b *builder) genStatement(stmt *ir.Node) error {
	switch stmt.Op {
	case ir.OpReturn:
		return b.genReturn(stmt)
	case ir.OpDeclaration:
		return b.genDecl(stmt)
	case ir.OpAssignment:
		return b.genAssign(stmt)
	case ir.OpPlusEq, ir.OpMinusEq, ir.OpTimesEq, ir.OpDivEq:
		return b.genAssignOp(stmt)
	default:
		return diagnostic.NewError(stmt.Pos, "unknown statement operation: %d", stmt.Op)
	}
}

func (b *builder) genAssignOp(n *ir.Node) error {
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "assignment operator missing target or expression")
	}
	target := n.List[0]

	// variable location
	alloca := b.vars[target.Sym]
	if alloca == nil {
		return diagnostic.NewError(target.Pos, "variable used before declared: %s", target.Ident())
	}

	// load variable value
	loadOp := b.targetFunc.appendValue(OpLoad, target.Sym.Type, b.currentBlock)
	loadOp.Args = []*Value{alloca}

	// generate expression value
	exprVal, err := b.genExpr(n.List[1])
	if err != nil {
		return err
	}

	// glue together with arithmetic bop
	arithOp := b.targetFunc.insertValueAfter(exprVal, numericBopFrom(n), target.Sym.Type, exprVal.Block)
	arithOp.Args = []*Value{loadOp, exprVal}

	// insert store operation into stack location
	storeOp := b.targetFunc.insertValueAfter(arithOp, OpStore, exprVal.Type, exprVal.Block)
	storeOp.Args = []*Value{arithOp, alloca}

	return nil
}

func (b *builder) genReturn(n *ir.Node) error {
	if len(n.List) != 1 {
		return diagnostic.NewError(n.Pos, "return statement missing expression")
	}

	retVal, err := b.genExpr(n.List[0])
	if err != nil {
		return err
	}

	if b.currentBlock != nil && b.currentBlock.Kind == BlockUnset {
		b.currentBlock.Kind = BlockRet
		b.currentBlock.Control = retVal
	}

	return nil
}

func (b *builder) genDecl(n *ir.Node) error {
	if len(n.List) != 3 {
		return diagnostic.NewError(n.Pos, "variable declaration missing type or expression")
	}

	exprVal, err := b.genExpr(n.List[2])
	if err != nil {
		return err
	}

	// make sure this isn't already allocated
	if _, ok := b.vars[n.Sym]; ok {
		return diagnostic.NewError(n.Pos, "variable already allocated: %s", n.List[0].Ident())
	}

	// come up with stack location for the new variable
	alloca := b.targetFunc.newValue(OpAlloca, exprVal.Type, b.currentBlock)
	b.vars[n.Sym] = alloca

	// insert store operation into the new stack location
	storeOp := b.targetFunc.insertValueAfter(exprVal, OpStore, exprVal.Type, exprVal.Block)
	storeOp.Args = []*Value{exprVal, alloca}

	return nil
}

func (b *builder) genAssign(n *ir.Node) error {
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "variable assignment missing target or expression")
	}
	target := n.List[0]

	exprVal, err := b.genExpr(n.List[1])
	if err != nil {
		return err
	}

	alloca := b.vars[target.Sym]
	if alloca == nil {
		return diagnostic.NewError(target.Pos, "variable used before declared: %s", target.Ident())
	}

	// insert store operation into stack location
	storeOp := b.targetFunc.insertValueAfter(exprVal, OpStore, exprVal.Type, exprVal.Block)
	storeOp.Args = []*Value{exprVal, alloca}

	return nil
}

func (b *builder) genExpr(expr *ir.Node) (*Value, error) {
	switch expr.Op {
	case ir.OpInt:
		return b.genInt(expr)
	case ir.OpPlus, ir.OpMinus, ir.OpTimes, ir.OpDiv:
		return b.genBop(expr)
	case ir.OpIdent:
		return b.genIdent(expr)
	case ir.OpNegate:
		return b.genNegate(expr)
	default:
		return nil, diagnostic.NewError(expr.Pos, "unknown expression operation: %d", expr.Op)
	}
}

func (b *builder) genNegate(expr *ir.Node) (*Value, error) {
	if len(expr.List) != 1 {
		return nil, diagnostic.NewError(expr.Pos, "negation operator without one operand")
	}

	e, err := b.genExpr(expr.List[0])
	if err != nil {
		return nil, err
	}

	negateOp := b.targetFunc.appendValue(OpNegate, expr.Type, b.currentBlock)
	negateOp.Args = []*Value{e}

	return negateOp, nil
}

func (b *builder) genIdent(expr *ir.Node) (*Value, error) {
	alloca := b.vars[expr.Sym]
	if alloca == nil {
		return nil, diagnostic.NewError(expr.Pos, "variable used before declared: %s", expr.Ident())
	}

	loadOp := b.targetFunc.appendValue(OpLoad, expr.Type, b.currentBlock)
	loadOp.Args = []*Value{alloca}

	return loadOp, nil
}

func (b *builder) genInt(expr *ir.Node) (*Value, error) {
	if types.Equal(expr.Type, types.Int()) {
		v := b.targetFunc.appendValue(OpLiteral, types.Int(), b.currentBlock)
		v.Value = int32(expr.Val.(*big.Int).Int64())
		return v, nil
	}
	return nil, diagnostic.NewError(expr.Pos, "unknown integer type: %v", expr.Type)
}

func (b *builder) genBop(expr *ir.Node) (*Value, error) {
	if len(expr.List) != 2 {
		return nil, diagnostic.NewError(expr.Pos, "binary operator without two operands")
	}
	left := expr.List[0]
	right := expr.List[1]

	leftVal, err := b.genExpr(left)
	if err != nil {
		return nil, err
	}

	rightVal, err := b.genExpr(right)
	if err != nil {
		return nil, err
	}

	if expr.Type.IsConcreteNumeric() {
		v := b.targetFunc.appendValue(numericBopFrom(expr), expr.Type, b.currentBlock)
		v.Args = []*Value{leftVal, rightVal}
		return v, nil
	}

	return nil, diagnostic.NewError(expr.Pos, "cannot perform binary operation for type %v", expr.Type)
}

func numericBopFrom(n *ir.Node) Op {
	switch n.Op {
	case ir.OpPlus, ir.OpPlusEq:
		return OpAdd
	case ir.OpMinus, ir.OpMinusEq:
		return OpSubtract
	case ir.OpTimes, ir.OpTimesEq:
		return OpMultiply
	case ir.OpDiv, ir.OpDivEq:
		return OpDivide
	default:
		return OpUnknown
	}
}
