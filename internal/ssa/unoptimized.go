package ssa

import (
	"math/big"
	"slices"

	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/types"
)

// buildFuncBody fills in the pre-created shell f from its AST node n.
func (m *Module) buildFuncBody(n *ir.Node) error {
	// look up function in module
	f := m.lookup(n.Signature.Label)
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

	// control reaching the end of the body returns implicitly.
	if b.currentBlock != nil && b.currentBlock.Kind == BlockUnset {
		b.currentBlock.Kind = BlockRet
	}

	return nil
}

// bindParams materializes incoming arguments at the top of the entry block.
func (b *builder) bindParams(params []*ir.Node) {
	// flatten the whole signature into ABI order
	var incomingTypes []*types.Type
	for _, p := range params {
		leafTypes := slices.Collect(iterutil.Second(p.Type.Leaves()))
		incomingTypes = append(incomingTypes, leafTypes...)
	}

	// append every incomingValues leaf as a restricted param value
	incomingValues := make([]*Value, len(incomingTypes))
	for i, t := range incomingTypes {
		v := b.targetFunc.appendValue(OpParam, t, b.currentBlock)
		v.Value = i
		incomingValues[i] = v
	}

	// copy each leaf into a non-restricted slot
	copies := make([]*Value, len(incomingValues))
	for i, v := range incomingValues {
		c := b.targetFunc.appendValue(OpCopy, v.Type, b.currentBlock)
		c.Args = []*Value{v}
		copies[i] = c
	}

	// reassemble each parameter from the leaves it owns into its own stack slot
	for _, p := range params {
		slot := b.targetFunc.newSlot(p.Sym, p.Type)
		b.vars[p.Sym] = slot
		// update copies to just the unused tail from implode
		copies = b.implode(addr{Slot: slot}, p.Type, copies)
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

	vals, err := b.genArg(n.List[0])
	if err != nil {
		return err
	}

	return b.genResults(n, vals)
}

// genResults hands vals back to the caller, one result value per value returned.
func (b *builder) genResults(n *ir.Node, vals []*Value) error {
	if len(vals) > 0 && !results.fits(len(vals)-1) {
		return diagnostic.NewError(n.Pos, "cannot return %d values: only %d fit in the result registers", len(vals), len(results.regs))
	}

	var results []*Value
	for i, val := range vals {
		// results store their index in the value slot, the same way parameters do
		res := b.targetFunc.appendValue(OpResult, val.Type, b.currentBlock)
		res.Value = i
		res.Args = []*Value{val}
		results = append(results, res)
	}

	// a second return in the same block hands back its own values, not the earlier ones
	b.currentBlock.Control = results

	return nil
}

func (b *builder) genDecl(n *ir.Node) error {
	if len(n.List) != 3 {
		return diagnostic.NewError(n.Pos, "variable declaration missing type or expression")
	}

	if _, ok := b.vars[n.Sym]; ok {
		return diagnostic.NewError(n.Pos, "variable already allocated: %s", n.List[0].Ident())
	}

	// reserve a slot for the new variable
	slot := b.targetFunc.newSlot(n.Sym, n.Sym.Type)

	// generate n into the slot
	if err := b.genExprInto(addr{Slot: slot}, n.List[2]); err != nil {
		return err
	}

	b.vars[n.Sym] = slot
	return nil
}

func (b *builder) genAssign(n *ir.Node) error {
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "variable assignment missing target or expression")
	}

	// figure out destination we're assigning to
	dest, err := b.genLValue(n.List[0])
	if err != nil {
		return err
	}

	// generate new value into destination
	return b.genExprInto(dest, n.List[1])
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
		return b.genCallValue(expr)
	case ir.OpRef:
		return b.genRef(expr)
	case ir.OpDeref:
		return b.genDeref(expr)
	case ir.OpUnit:
		return b.genUnit(expr)
	case ir.OpDot:
		place, err := b.genPlace(expr)
		if err != nil {
			return nil, err
		}
		return b.genLoadFrom(place, expr.Type), nil
	default:
		return nil, diagnostic.NewError(expr.Pos, "unknown expression operation: %d", expr.Op)
	}
}

// genPlace returns an addressable bucket for expr
func (b *builder) genPlace(expr *ir.Node) (addr, error) {
	if isPlace(expr) {
		// idents, derefs, dots already refer to addresable locations so grab that address
		return b.genLValue(expr)
	}

	// all others need a new address
	a := addr{
		Slot: b.targetFunc.newSlot(nil, expr.Type),
	}
	if err := b.genExprInto(a, expr); err != nil {
		return addr{}, err
	}
	return a, nil
}

// genExprInto generates an expression value into an area of memory
func (b *builder) genExprInto(dest addr, expr *ir.Node) error {
	switch expr.Op {
	case ir.OpTuple:
		params := expr.Type.Params()

		if len(expr.List) != len(params) {
			return diagnostic.NewError(expr.Pos, "tuple has %d elements but its type has %d", len(expr.List), len(params))
		}

		// each element goes directly into its field in the tuple
		for i := range params {
			if err := b.genExprInto(dest.OffsetBy(expr.Type.Offset(i)), expr.List[i]); err != nil {
				return err
			}
		}
	case ir.OpCall:
		vals, err := b.genCallResults(expr)
		if err != nil {
			return err
		}
		b.implode(dest, expr.Type, vals)
	default:
		// existing tuples must be copied
		if expr.Type.IsTuple() && isPlace(expr) {
			src, err := b.genLValue(expr)
			if err != nil {
				return err
			}
			b.genCopyFields(dest, src, expr.Type)
			return nil
		}

		v, err := b.genExpr(expr)
		if err != nil {
			return err
		}
		b.genStoreTo(dest, v)
	}

	return nil
}

// isPlace reports whether expr denotes an existing addressable location.
func isPlace(expr *ir.Node) bool {
	switch expr.Op {
	case ir.OpIdent, ir.OpDeref, ir.OpDot:
		return true
	}
	return false
}

// genCopyFields copies src into dest one atomic field at a time
func (b *builder) genCopyFields(dest, src addr, t *types.Type) {
	for offset, leaf := range t.Leaves() {
		b.genStoreTo(dest.OffsetBy(offset), b.genLoadFrom(src.OffsetBy(offset), leaf))
	}
}

func (b *builder) genUnit(*ir.Node) (*Value, error) {
	return b.targetFunc.appendValue(OpUnit, types.Unit(), b.currentBlock), nil
}

func (b *builder) genRef(expr *ir.Node) (*Value, error) {
	if len(expr.List) < 1 {
		return nil, diagnostic.NewError(expr.Pos, "invalid number of args in ref")
	}

	dest, err := b.genLValue(expr.List[0])
	if err != nil {
		return nil, err
	}

	// the base address already exists, so the field is a walk forward from it
	if dest.Ptr != nil {
		if dest.Offset == 0 {
			return dest.Ptr, nil
		}

		v := b.targetFunc.appendValue(OpFieldAddr, expr.Type, b.currentBlock)
		v.Args = []*Value{dest.Ptr}
		v.Offset = dest.Offset

		return v, nil
	}

	// take the address of the stack slot
	v := b.targetFunc.appendValue(OpLocalAddr, expr.Type, b.currentBlock)
	v.Value = dest.Slot
	v.Offset = dest.Offset

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

	return b.genLoadFrom(addr{Ptr: ptr}, expr.Type), nil
}

type addr struct {
	Slot   *Slot  // frame slot referenced directly
	Ptr    *Value // an address computed at runtime
	Offset int    // offset from address
}

func (a addr) OffsetBy(bytes int) addr {
	a.Offset += bytes
	return a
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
	case ir.OpDot:
		// get the address of the dot'ed tuple
		base, err := b.genPlace(expr.List[0])
		if err != nil {
			return addr{}, err
		}
		// dot field stored as a big Int we need to convert it (gross)
		idx := int(expr.List[1].Val.(*big.Int).Int64())
		return base.OffsetBy(expr.List[0].Type.Offset(idx)), nil
	}
	return addr{}, diagnostic.NewError(expr.Pos, "invalid op for lvalue: %v", expr.Op)
}

// genLoadFrom reads the value of type t living at dest.
func (b *builder) genLoadFrom(dest addr, t *types.Type) *Value {
	// a singleton's value is already known from its type, so there is nothing to read
	if t.IsSingleton() {
		return b.targetFunc.appendValue(OpUnit, t, b.currentBlock)
	}

	if dest.Slot != nil {
		v := b.targetFunc.appendValue(OpStaticLoad, t, b.currentBlock)
		v.Value = dest.Slot
		v.Offset = dest.Offset
		return v
	}

	v := b.targetFunc.appendValue(OpLoad, t, b.currentBlock)
	v.Args = []*Value{dest.Ptr}
	v.Offset = dest.Offset
	return v
}

// genStoreTo writes val to dest. A singleton carries no information, so it is
// evaluated for its effects and then dropped rather than written anywhere.
func (b *builder) genStoreTo(dest addr, val *Value) {
	if val.Type.IsSingleton() {
		return
	}

	if dest.Slot != nil {
		v := b.targetFunc.appendValue(OpStaticStore, val.Type, b.currentBlock)
		v.Args = []*Value{val}
		v.Value = dest.Slot
		v.Offset = dest.Offset
		return
	}

	v := b.targetFunc.appendValue(OpStore, val.Type, b.currentBlock)
	v.Args = []*Value{val, dest.Ptr}
	v.Offset = dest.Offset
}

// genArg evaluates arg into a flat list of atomic values
func (b *builder) genArg(arg *ir.Node) ([]*Value, error) {
	place, err := b.genPlace(arg)
	if err != nil {
		return nil, err
	}
	return b.explode(place, arg.Type), nil
}

// explode reads the atomic leaf values of t into a list of values
func (b *builder) explode(a addr, t *types.Type) []*Value {
	var vals []*Value
	for offset, leaf := range t.Leaves() {
		vals = append(vals, b.genLoadFrom(a.OffsetBy(offset), leaf))
	}
	return vals
}

// implode writes the head of vals back into addr of type t and returns an unconsumed tail
func (b *builder) implode(a addr, t *types.Type, vals []*Value) []*Value {
	for offset := range t.Leaves() {
		b.genStoreTo(a.OffsetBy(offset), vals[0])
		vals = vals[1:]
	}
	return vals
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
		vals, err := b.genArg(arg)
		if err != nil {
			return nil, err
		}
		argVals = append(argVals, vals...)
	}

	v := b.targetFunc.appendValue(OpStaticCall, expr.Type, b.currentBlock)
	v.Value = callee
	v.Args = argVals

	return v, nil
}

// genCallResults emits a call and reads back the returned values
func (b *builder) genCallResults(expr *ir.Node) ([]*Value, error) {
	call, err := b.genCall(expr)
	if err != nil {
		return nil, err
	}

	var vals []*Value
	for i, leaf := range iterutil.Enumerate(iterutil.Second(expr.Type.Leaves())) {
		// the callee cannot have placed a value the sequence has no home for
		if !results.fits(i) {
			return nil, diagnostic.NewError(expr.Pos, "cannot read back a call returning more than %d values", len(results.regs))
		}

		// results store their index in the value slot, the same way parameters do
		res := b.targetFunc.appendValue(OpCallResult, leaf, b.currentBlock)
		res.Value = i
		res.Args = []*Value{call}
		vals = append(vals, res)
	}

	return vals, nil
}

// genCallValue evaluates a call standing where a single value is expected.
func (b *builder) genCallValue(expr *ir.Node) (*Value, error) {
	vals, err := b.genCallResults(expr)
	if err != nil {
		return nil, err
	}

	switch len(vals) {
	case 0:
		// a singleton's value is known from its type, so the call hands back nothing to read
		return b.targetFunc.appendValue(OpUnit, expr.Type, b.currentBlock), nil
	case 1:
		return vals[0], nil
	}

	return nil, diagnostic.NewError(expr.Pos, "call returning %d values used where one is expected", len(vals))
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

	v := b.targetFunc.appendValue(numericBopFrom(expr), expr.Type, b.currentBlock)
	v.Args = []*Value{leftVal, rightVal}
	return v, nil
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
