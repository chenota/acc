package ssa

import (
	"math/big"

	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/types"
)

// buildFuncBody fills in the pre-created shell f from its AST node n.
func (m *Module) buildFuncBody(n *ir.Node) error {
	// look up function in module
	f := m.lookup(n.Sym.Name)
	if f == nil {
		return diagnostic.NewError(n.Pos, "could not find function in module")
	}

	b := &builder{targetFunc: f, module: m, vars: make(map[*ir.Sym]*Value)}

	entry := f.newBlock()
	f.Entry = entry
	b.currentBlock = entry

	b.bindParams(n.Signature.Params)

	for _, stmt := range n.List {
		if err := b.genStatement(stmt); err != nil {
			return err
		}
	}

	return nil
}

// bindParams materializes incoming arguments at the top of the entry block. Each
// parameter arrives in a fixed ABI register (an OpParam, pinned later in
// constraints); we immediately copy it into an unconstrained value so the body
// can hold it across clobbers like a call. The copy is bound through an alloca
// so it behaves as an ordinary mutable local (mem2reg promotes it away).
//
// All OpParams are defined first, before any copy, so every argument register
// stays reserved until its value is safely copied out -- otherwise an early
// copy's destination could reuse a register still holding a later argument.
func (b *builder) bindParams(params []*ir.Node) {
	incoming := make([]*Value, len(params))
	for i, p := range params {
		v := b.targetFunc.appendValue(OpParam, p.Type, b.currentBlock)
		v.Value = i
		incoming[i] = v
	}

	for i, p := range params {
		param := b.targetFunc.appendValue(OpCopy, p.Type, b.currentBlock)
		param.Args = []*Value{incoming[i]}

		alloca := b.targetFunc.newValue(OpAlloca, p.Type, b.currentBlock)
		b.vars[p.Sym] = alloca

		store := b.targetFunc.appendValue(OpStore, p.Type, b.currentBlock)
		store.Args = []*Value{param, alloca}
	}
}

type builder struct {
	targetFunc   *Func
	module       *Module
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
	case ir.OpCall:
		return b.genCall(expr)
	default:
		return nil, diagnostic.NewError(expr.Pos, "unknown expression operation: %d", expr.Op)
	}
}

func (b *builder) genCall(expr *ir.Node) (*Value, error) {
	if len(expr.List) < 1 {
		return nil, diagnostic.NewError(expr.Pos, "call without a callee")
	}

	// analyze the expression being called
	callee, err := b.genExpr(expr.List[0])
	if err != nil {
		return nil, err
	}

	args := expr.List[1:]
	var argVals []*Value
	for _, arg := range args {
		argVal, err := b.genExpr(arg)
		if err != nil {
			return nil, err
		}
		argVals = append(argVals, argVal)
	}

	v := b.targetFunc.appendValue(OpCall, expr.Type, b.currentBlock)
	v.Args = append([]*Value{callee}, argVals...)

	return v, nil
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
	switch expr.Sym.Kind {
	case ir.SymGlobal:
		callee := b.module.lookup(expr.Sym.Name)
		if callee == nil {
			return nil, diagnostic.NewError(expr.Pos, "reference to unknown function: %s", expr.Sym.Name)
		}
		v := b.targetFunc.appendValue(OpFuncRef, expr.Type, b.currentBlock)
		v.Value = callee
		return v, nil
	default:
		alloca := b.vars[expr.Sym]
		if alloca == nil {
			return nil, diagnostic.NewError(expr.Pos, "variable used before declared: %s", expr.Ident())
		}
		loadOp := b.targetFunc.appendValue(OpLoad, expr.Type, b.currentBlock)
		loadOp.Args = []*Value{alloca}
		return loadOp, nil
	}
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
