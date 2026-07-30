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

	b := &builder{targetFunc: f, module: m, vars: make(map[*ir.Sym]*Slot)}

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

// bindParams materializes incoming arguments at the top of the entry block.
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

		slot := b.targetFunc.newSlot(p.Sym, p.Type)
		b.vars[p.Sym] = slot

		b.genStoreTo(addr{Slot: slot}, param)
	}
}

type builder struct {
	targetFunc   *Func
	module       *Module
	currentBlock *Block
	vars         map[*ir.Sym]*Slot
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
	case ir.OpCall:
		// ignore the call's return value
		_, err := b.genCall(stmt)
		return err
	default:
		return diagnostic.NewError(stmt.Pos, "unknown statement operation: %d", stmt.Op)
	}
}

func (b *builder) genAssignOp(n *ir.Node) error {
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "assignment operator missing target or expression")
	}
	target := n.List[0]

	dest, err := b.genLValue(target)
	if err != nil {
		return err
	}

	// read the current value out of the destination
	loadOp := b.genLoadFrom(dest, target.Type)

	// generate expression value
	exprVal, err := b.genExpr(n.List[1])
	if err != nil {
		return err
	}

	// glue together with arithmetic bop
	arithOp := b.targetFunc.appendValue(numericBopFrom(n), target.Type, b.currentBlock)
	arithOp.Args = []*Value{loadOp, exprVal}

	// write the result back where it came from
	b.genStoreTo(dest, arithOp)

	return nil
}

func (b *builder) genReturn(n *ir.Node) error {
	b.currentBlock.Kind = BlockRet

	if len(n.List) == 0 {
		// no return value we're done
		return nil
	}

	retVal, err := b.genExpr(n.List[0])
	if err != nil {
		return err
	}
	b.currentBlock.Control = retVal

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

	// reserve a slot for the new variable
	slot := b.targetFunc.newSlot(n.Sym, exprVal.Type)
	b.vars[n.Sym] = slot

	b.genStoreTo(addr{Slot: slot}, exprVal)

	return nil
}

func (b *builder) genAssign(n *ir.Node) error {
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "variable assignment missing target or expression")
	}

	exprVal, err := b.genExpr(n.List[1])
	if err != nil {
		return err
	}

	dest, err := b.genLValue(n.List[0])
	if err != nil {
		return err
	}

	b.genStoreTo(dest, exprVal)

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
	case ir.OpRef:
		return b.genRef(expr)
	case ir.OpDeref:
		return b.genDeref(expr)
	default:
		return nil, diagnostic.NewError(expr.Pos, "unknown expression operation: %d", expr.Op)
	}
}

func (b *builder) genRef(expr *ir.Node) (*Value, error) {
	if len(expr.List) < 1 {
		return nil, diagnostic.NewError(expr.Pos, "invalid number of args in ref")
	}

	dest, err := b.genLValue(expr.List[0])
	if err != nil {
		return nil, err
	}

	// already returning the address nothing more to do
	if dest.Ptr != nil {
		return dest.Ptr, nil
	}

	// take the address of the stack slot
	v := b.targetFunc.appendValue(OpLocalAddr, expr.Type, b.currentBlock)
	v.Value = dest.Slot

	return v, nil
}

func (b *builder) genDeref(expr *ir.Node) (*Value, error) {
	if len(expr.List) < 1 {
		return nil, diagnostic.NewError(expr.Pos, "invalid number of args in deref")
	}

	ptr, err := b.genExpr(expr.List[0])
	if err != nil {
		return nil, err
	}

	v := b.targetFunc.appendValue(OpLoad, expr.Type, b.currentBlock)
	v.Args = []*Value{ptr}

	return v, nil
}

type addr struct {
	Slot *Slot  // frame slot referenced directly
	Ptr  *Value // an address computed at runtime
}

func (b *builder) genLValue(expr *ir.Node) (addr, error) {
	switch expr.Op {
	case ir.OpIdent:
		if slot, ok := b.vars[expr.Sym]; ok {
			return addr{Slot: slot}, nil
		}
		return addr{}, diagnostic.NewError(expr.Pos, "variable missing slot: %s", expr.Sym.Name)
	case ir.OpDeref:
		if len(expr.List) < 1 {
			return addr{}, diagnostic.NewError(expr.Pos, "deref missing argument")
		}
		// evaluating the expression should return a value containing the address we care about
		ptr, err := b.genExpr(expr.List[0])
		if err != nil {
			return addr{}, err
		}
		return addr{Ptr: ptr}, nil
	}
	return addr{}, diagnostic.NewError(expr.Pos, "invalid op for lvalue: %v", expr.Op)
}

// genLoadFrom reads the value of type t living at dest.
func (b *builder) genLoadFrom(dest addr, t *types.Type) *Value {
	if dest.Slot != nil {
		v := b.targetFunc.appendValue(OpStaticLoad, t, b.currentBlock)
		v.Value = dest.Slot
		return v
	}

	v := b.targetFunc.appendValue(OpLoad, t, b.currentBlock)
	v.Args = []*Value{dest.Ptr}
	return v
}

// genStoreTo writes val to dest.
func (b *builder) genStoreTo(dest addr, val *Value) *Value {
	if dest.Slot != nil {
		v := b.targetFunc.appendValue(OpStaticStore, val.Type, b.currentBlock)
		v.Args = []*Value{val}
		v.Value = dest.Slot
		return v
	}

	v := b.targetFunc.appendValue(OpStore, val.Type, b.currentBlock)
	v.Args = []*Value{val, dest.Ptr}
	return v
}

func (b *builder) genCall(expr *ir.Node) (*Value, error) {
	if len(expr.List) < 1 {
		return nil, diagnostic.NewError(expr.Pos, "call without a callee")
	}

	callee, err := b.resolveCallee(expr.List[0])
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

	v := b.targetFunc.appendValue(OpStaticCall, expr.Type, b.currentBlock)
	v.Value = callee
	v.Args = argVals

	return v, nil
}

// resolveCallee identifies the function a call targets.
func (b *builder) resolveCallee(callee *ir.Node) (*Func, error) {
	if callee.Op != ir.OpIdent || callee.Sym.Kind != ir.SymFunc {
		return nil, diagnostic.NewError(callee.Pos, "only calls to top-level functions are supported")
	}

	target := b.module.lookup(callee.Sym.Name)
	if target == nil {
		return nil, diagnostic.NewError(callee.Pos, "reference to unknown function: %s", callee.Sym.Name)
	}

	return target, nil
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
	case ir.SymFunc:
		// a function name is only meaningful as a call target until functions become values
		return nil, diagnostic.NewError(expr.Pos, "cannot use function as a value: %s", expr.Ident())
	case ir.SymParam, ir.SymLocal:
		slot := b.vars[expr.Sym]
		if slot == nil {
			return nil, diagnostic.NewError(expr.Pos, "no stack location for variable: %s", expr.Ident())
		}
		return b.genLoadFrom(addr{Slot: slot}, expr.Type), nil
	}
	return nil, diagnostic.NewError(expr.Pos, "unknown symbol kind: %v", expr.Sym.Kind)
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
