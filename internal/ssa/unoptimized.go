package ssa

import (
	"math/big"
	"slices"

	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/types"
)

// buildFuncBody fills in the module's pre-created function for n.
func (m *Module) buildFuncBody(n *ir.Node) error {
	f := m.lookup(n.Signature.Label)
	if f == nil {
		return diagnostic.NewError(n.Pos, "could not find function in module")
	}

	b := &builder{
		targetFunc: f,
		module:     m,
		vars:       make(map[*ir.Sym]*Slot),
		captures:   make(map[*ir.Sym]*Slot),
	}

	entry := f.newBlock()
	f.Entry = entry
	b.currentBlock = entry

	b.bindCaptures(n)
	b.bindParams(n.Signature.Params)

	body := n.Body()
	for _, stmt := range body.Stmts() {
		if err := b.genStatement(stmt); err != nil {
			return err
		}
	}

	if v := body.Value(); v != nil {
		if err := b.genReturnValue(v); err != nil {
			return err
		}
	}

	// control reaching the end of the body returns implicitly.
	if b.currentBlock != nil && b.currentBlock.Kind == BlockUnset {
		b.currentBlock.Kind = BlockRet
	}

	return nil
}

// bindCaptures reads captured environment variables from the context register
// TODO: Switch over to lazy unpacking when there's more need for it
func (b *builder) bindCaptures(n *ir.Node) {
	captures := n.Captures()
	if len(captures) == 0 {
		return
	}

	envType := closureEnvType(n)
	env := b.targetFunc.appendValue(OpClosurePtr, types.Pointer(envType), b.currentBlock)

	for i, sym := range captures {
		field := addr{Ptr: env}.OffsetBy(envType.Offset(i))

		if capturedByValue(n, sym) {
			// if captured by value it's a regular local
			slot := b.targetFunc.newSlot(sym, sym.Type)
			b.genCopyFields(addr{Slot: slot}, field, sym.Type)
			b.vars[sym] = slot
			continue
		}

		box := b.genLoadFrom(field, types.Pointer(sym.Type))
		slot := b.targetFunc.newSlot(sym, types.Pointer(sym.Type))
		b.genStoreTo(addr{Slot: slot}, box)
		b.captures[sym] = slot
	}
}

// bindParams materializes incoming arguments at the top of the entry block.
func (b *builder) bindParams(params []*ir.Node) {
	// flatten the whole signature into ABI order
	var incomingTypes []*types.Type
	for _, p := range params {
		leafTypes := slices.Collect(iterutil.Second(p.Type.Leaves()))
		incomingTypes = append(incomingTypes, leafTypes...)
	}

	incomingValues := make([]*Value, len(incomingTypes))
	for i, t := range incomingTypes {
		v := b.targetFunc.appendValue(OpParam, t, b.currentBlock)
		v.Value = i
		incomingValues[i] = v
	}

	// reassemble each parameter from the leaves it owns into its own stack slot
	leaves := incomingValues
	for _, p := range params {
		slot := b.targetFunc.newSlot(p.Sym, p.Type)
		b.vars[p.Sym] = slot
		// update leaves to just the unused tail from implode
		leaves = b.implode(addr{Slot: slot}, p.Type, leaves)
	}
}

type breakTarget struct {
	head *Block // header of loop structure
	exit *Block // block being jumped to
	dest addr   // where to put value
}

type builder struct {
	targetFunc   *Func
	module       *Module
	currentBlock *Block
	vars         map[*ir.Sym]*Slot
	captures     map[*ir.Sym]*Slot
	target       breakTarget
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
	case ir.OpConditional:
		// the value is discarded, but the branches still need somewhere to write it
		return b.genConditional(addr{Slot: b.targetFunc.newSlot(nil, stmt.Type)}, stmt)
	case ir.OpLoop:
		// the value is discarded, but the branches still need somewhere to write it
		return b.genLoop(addr{Slot: b.targetFunc.newSlot(nil, stmt.Type)}, stmt)
	case ir.OpBreak:
		return b.genBreak(stmt)
	case ir.OpContinue:
		return b.genContinue(stmt)
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

	loadOp := b.genLoadFrom(dest, target.Type)

	exprVal, err := b.genExpr(n.List[1])
	if err != nil {
		return err
	}

	arithOp := b.targetFunc.appendValue(bopFrom(n), target.Type, b.currentBlock)
	arithOp.setArgs(loadOp, exprVal)

	b.genStoreTo(dest, arithOp)

	return nil
}

func (b *builder) genReturn(n *ir.Node) error {
	if len(n.List) == 0 {
		b.currentBlock.Kind = BlockRet
		return nil
	}

	return b.genReturnValue(n.List[0])
}

func (b *builder) genReturnValue(e *ir.Node) error {
	vals, err := b.genArg(e)
	if err != nil {
		return err
	}

	b.currentBlock.Kind = BlockRet

	return b.genResults(e, vals)
}

func (b *builder) genResults(n *ir.Node, vals []*Value) error {
	var results []*Value
	for i, val := range vals {
		// results store their index in the value slot, the same way parameters do
		res := b.targetFunc.appendValue(OpResult, val.Type, b.currentBlock)
		res.Value = i
		res.setArgs(val)
		results = append(results, res)
	}

	b.currentBlock.setControl(results...)

	return nil
}

func (b *builder) genDecl(n *ir.Node) error {
	if len(n.List) != 3 {
		return diagnostic.NewError(n.Pos, "variable declaration missing type or expression")
	}

	if _, ok := b.vars[n.Sym]; ok {
		return diagnostic.NewError(n.Pos, "variable already allocated: %s", n.List[0].Ident())
	}

	// reserve a slot for the new variable, visible before the initializer so a let rec closure can capture it
	slot := b.targetFunc.newSlot(n.Sym, n.Sym.Type)
	b.vars[n.Sym] = slot

	return b.genExprInto(addr{Slot: slot}, n.List[2])
}

func (b *builder) genAssign(n *ir.Node) error {
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "variable assignment missing target or expression")
	}

	dest, err := b.genLValue(n.List[0])
	if err != nil {
		return err
	}

	// build-then-copy tuples (and conditionals and loops, whose branches and breaks may build tuples) to get around self-assign issues
	rhs := n.List[1]
	if rhs.Op == ir.OpTuple || rhs.Op == ir.OpConditional || rhs.Op == ir.OpLoop {
		tmp := addr{Slot: b.targetFunc.newSlot(nil, rhs.Type)}
		if err := b.genExprInto(tmp, rhs); err != nil {
			return err
		}
		b.genCopyFields(dest, tmp, rhs.Type)
		return nil
	}

	return b.genExprInto(dest, rhs)
}

func (b *builder) genExpr(expr *ir.Node) (*Value, error) {
	switch expr.Op {
	case ir.OpInt:
		return b.genInt(expr)
	case ir.OpBool:
		return b.genBool(expr)
	case ir.OpPlus, ir.OpMinus, ir.OpTimes, ir.OpDiv, ir.OpEqual, ir.OpNotEqual, ir.OpLess, ir.OpLessEqual, ir.OpGreater, ir.OpGreaterEqual:
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
	case ir.OpNil:
		return b.genNil(expr)
	case ir.OpNot:
		return b.genNot(expr)
	case ir.OpConditional, ir.OpLoop, ir.OpAnd, ir.OpOr:
		// the branches (or breaks) write into a temporary that is read back once control rejoins
		place, err := b.genPlace(expr)
		if err != nil {
			return nil, err
		}
		return b.genLoadFrom(place, expr.Type), nil
	default:
		return nil, diagnostic.NewError(expr.Pos, "unknown expression operation: %d", expr.Op)
	}
}

func (b *builder) genNot(e *ir.Node) (*Value, error) {
	sub, err := b.genExpr(e.List[0])
	if err != nil {
		return nil, err
	}

	notVal := b.targetFunc.appendValue(OpNot, e.Type, b.currentBlock)
	notVal.setArgs(sub)

	return notVal, nil
}

func (b *builder) genBool(e *ir.Node) (*Value, error) {
	v, ok := e.Val.(bool)
	if !ok {
		return nil, diagnostic.NewError(e.Pos, "unable to convert node value to boolean")
	}

	return b.genBoolLiteral(v), nil
}

func (b *builder) genBoolLiteral(v bool) *Value {
	boolVal := b.targetFunc.appendValue(OpLiteral, types.Bool(), b.currentBlock)
	boolVal.Value = v
	return boolVal
}

func closureEnvType(lambda *ir.Node) *types.Type {
	captures := lambda.Captures()
	elems := make([]*types.Type, 0, len(captures))
	for _, sym := range captures {
		if capturedByValue(lambda, sym) {
			elems = append(elems, sym.Type)
			continue
		}
		elems = append(elems, types.Pointer(sym.Type))
	}
	return types.Tuple(elems)
}

// TODO: raise this once captures are unpacked lazily and the per-call copy goes away
const maxByValueLeaves = 4

func capturedByValue(lambda *ir.Node, sym *ir.Sym) bool {
	if sym.Observed(ir.SOMutated) || sym == lambda.RecSym() {
		return false
	}

	// large aggregates are cheaper to share than to copy
	return sym.Type.LeafCount() <= maxByValueLeaves
}

// genClosure writes the (code, env) pair for code closing over lambda's captures into dest
func (b *builder) genClosure(dest addr, expr *ir.Node, code *Func, lambda *ir.Node) error {
	codeRef := b.targetFunc.appendValue(OpLabelAddr, types.UnitPointer(), b.currentBlock)
	// TODO: I hate this OpLabelAddr must be made more generic
	codeRef.Value = code

	envRef, err := b.genEnv(expr, lambda)
	if err != nil {
		return err
	}

	b.implode(dest, expr.Type, []*Value{codeRef, envRef})

	return nil
}

// genZero is a zero literal of the register-sized type t
func (b *builder) genZero(t *types.Type) *Value {
	v := b.targetFunc.appendValue(OpLiteral, t, b.currentBlock)
	// TODO: This needs to be an int64(0) when there's better plumbing for this sort of thing
	v.Value = int32(0)
	return v
}

func (b *builder) genEnv(expr *ir.Node, lambda *ir.Node) (*Value, error) {
	captures := lambda.Captures()
	if len(captures) == 0 {
		// nothing to close over, so the callee never reads its environment
		return b.genZero(types.UnitPointer()), nil
	}

	envType := closureEnvType(lambda)
	env := addr{Slot: b.targetFunc.newSlot(nil, envType)}

	// write captured values (or pointers to them) to the environment
	for i, sym := range captures {
		field := env.OffsetBy(envType.Offset(i))

		if capturedByValue(lambda, sym) {
			src, err := b.genSymPlace(expr, sym)
			if err != nil {
				return nil, err
			}
			b.genCopyFields(field, src, sym.Type)
			continue
		}

		box, err := b.genBoxAddr(expr, sym)
		if err != nil {
			return nil, err
		}
		b.genStoreTo(field, box)
	}

	envRef := b.targetFunc.appendValue(OpLocalAddr, types.UnitPointer(), b.currentBlock)
	envRef.Value = env.Slot

	return envRef, nil
}

func (b *builder) genBoxAddr(expr *ir.Node, sym *ir.Sym) (*Value, error) {
	if slot, ok := b.vars[sym]; ok {
		// shallow capture (variable lives in frame this closure is defined in), so grab from the stack
		v := b.targetFunc.appendValue(OpLocalAddr, types.Pointer(sym.Type), b.currentBlock)
		v.Value = slot
		return v, nil
	}

	if slot, ok := b.captures[sym]; ok {
		// nested capture (variable lives outside the frame this closure is defined in), so grab from captures list
		return b.genLoadFrom(addr{Slot: slot}, types.Pointer(sym.Type)), nil
	}

	return nil, diagnostic.NewError(expr.Pos, "no storage for captured variable: %s", sym.Name)
}

// genNil lowers nil to a zero occupying a whole pointer, so it needs no operator of its own.
func (b *builder) genNil(expr *ir.Node) (*Value, error) {
	if !expr.Type.IsPointer() {
		return nil, diagnostic.NewError(expr.Pos, "unknown nil type: %v", expr.Type)
	}

	return b.genZero(types.Int64()), nil
}

// genPlace returns an addressable bucket for expr
func (b *builder) genPlace(expr *ir.Node) (addr, error) {
	if isPlace(expr) {
		return b.genLValue(expr)
	}

	a := addr{
		Slot: b.targetFunc.newSlot(nil, expr.Type),
	}
	if err := b.genExprInto(a, expr); err != nil {
		return addr{}, err
	}
	return a, nil
}

func (b *builder) genExprInto(dest addr, expr *ir.Node) error {
	switch {
	case expr.Op == ir.OpTuple:
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
	case expr.Op == ir.OpCall:
		vals, err := b.genCallResults(expr)
		if err != nil {
			return err
		}
		b.implode(dest, expr.Type, vals)
	case expr.Op == ir.OpConditional:
		return b.genConditional(dest, expr)
	case expr.Op == ir.OpLoop:
		return b.genLoop(dest, expr)
	case expr.Op == ir.OpAnd || expr.Op == ir.OpOr:
		return b.genLogical(dest, expr)
	case expr.Op == ir.OpFunction:
		code := b.module.lookup(expr.Signature.Label)
		if code == nil {
			return diagnostic.NewError(expr.Pos, "reference to unknown function: %s", expr.Signature.Label)
		}
		return b.genClosure(dest, expr, code, expr)
	case expr.Op == ir.OpIdent && expr.Sym.Kind == ir.SymFunc:
		// global functions are closures that don't capture anything
		code := b.module.lookup(expr.Sym.Name)
		if code == nil {
			return diagnostic.NewError(expr.Pos, "reference to unknown function: %s", expr.Sym.Name)
		}
		return b.genClosure(dest, expr, code, nil)
	case isPlace(expr) && expr.Type.IsAggregate():
		// existing places must be copied
		src, err := b.genLValue(expr)
		if err != nil {
			return err
		}
		b.genCopyFields(dest, src, expr.Type)
		return nil
	default:
		v, err := b.genExpr(expr)
		if err != nil {
			return err
		}
		b.genStoreTo(dest, v)
	}

	return nil
}

func (b *builder) genConditional(dest addr, expr *ir.Node) error {
	if len(expr.List) != 3 {
		return diagnostic.NewError(expr.Pos, "conditional without three arguments")
	}

	predicate := expr.List[0]
	consequent := expr.List[1]
	alternative := expr.List[2]

	cond, err := b.genExpr(predicate)
	if err != nil {
		return err
	}

	return b.genBranch(cond,
		func() error { return b.genArm(dest, consequent) },
		func() error { return b.genArm(dest, alternative) },
	)
}

// genArm writes the value of one arm of a conditional into dest
func (b *builder) genArm(dest addr, arm *ir.Node) error {
	if arm == nil {
		return nil
	}

	switch arm.Op {
	case ir.OpConditional:
		// elif
		return b.genExprInto(dest, arm)
	case ir.OpBlock:
		for _, s := range arm.Stmts() {
			if err := b.genStatement(s); err != nil {
				return err
			}
		}
		if v := arm.Value(); v != nil {
			return b.genExprInto(dest, v)
		}
		return nil
	default:
		return diagnostic.NewError(arm.Pos, "conditional arm does not have required op")
	}
}

// genLogical writes a short-circuiting && or || into dest
func (b *builder) genLogical(dest addr, expr *ir.Node) error {
	if len(expr.List) != 2 {
		return diagnostic.NewError(expr.Pos, "logical operator without two operands")
	}

	cond, err := b.genExpr(expr.List[0])
	if err != nil {
		return err
	}

	right := func() error { return b.genExprInto(dest, expr.List[1]) }
	// a false left settles &&, and a true left settles ||
	settled := func() error {
		b.genStoreTo(dest, b.genBoolLiteral(expr.Op == ir.OpOr))
		return nil
	}

	if expr.Op == ir.OpAnd {
		return b.genBranch(cond, right, settled)
	}
	return b.genBranch(cond, settled, right)
}

// genBranch splits control on cond between then and els, leaving the block where they rejoin current
func (b *builder) genBranch(cond *Value, then, els func() error) error {
	branchBlock := b.currentBlock
	branchBlock.Kind = BlockIf
	branchBlock.setControl(cond)

	thenBlock := b.targetFunc.newBlock()
	addEdge(branchBlock, thenBlock)

	elseBlock := b.targetFunc.newBlock()
	addEdge(branchBlock, elseBlock)

	mergeBlock := b.targetFunc.newBlock()

	b.currentBlock = thenBlock
	if err := then(); err != nil {
		return err
	}
	if b.currentBlock.Kind == BlockUnset {
		b.jump(mergeBlock)
	}

	b.currentBlock = elseBlock
	if err := els(); err != nil {
		return err
	}
	if b.currentBlock.Kind == BlockUnset {
		b.jump(mergeBlock)
	}

	b.currentBlock = mergeBlock

	return nil
}

func (b *builder) genLoop(dest addr, expr *ir.Node) error {
	body := expr.Body()
	if body == nil {
		return diagnostic.NewError(expr.Pos, "loop missing body node")
	}

	header := b.targetFunc.newBlock()
	exit := b.targetFunc.newBlock()

	b.jump(header)

	oldBreakTo := b.target
	defer func() { b.target = oldBreakTo }()
	b.target = breakTarget{
		head: header,
		exit: exit,
		dest: dest,
	}

	b.currentBlock = header
	for _, s := range body.Stmts() {
		if err := b.genStatement(s); err != nil {
			return err
		}
	}
	if v := body.Value(); v != nil {
		// the body's value is unit and thrown away, but evaluating it may still have effects
		if err := b.genExprInto(addr{Slot: b.targetFunc.newSlot(nil, v.Type)}, v); err != nil {
			return err
		}
	}

	if b.currentBlock.Kind == BlockUnset {
		b.jump(header)
	}

	b.currentBlock = exit

	return nil
}

func (b *builder) genBreak(n *ir.Node) error {
	if len(n.List) == 1 {
		if err := b.genExprInto(b.target.dest, n.List[0]); err != nil {
			return err
		}
	}
	b.jump(b.target.exit)
	return nil
}

func (b *builder) genContinue(n *ir.Node) error {
	b.jump(b.target.head)
	return nil
}

func isPlace(expr *ir.Node) bool {
	switch expr.Op {
	case ir.OpIdent:
		// a global function has no storage of its own, so its name is a value rather than a place
		return expr.Sym.Kind != ir.SymFunc
	case ir.OpDeref, ir.OpDot:
		return true
	}
	return false
}

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
		v.setArgs(dest.Ptr)
		v.Offset = dest.Offset

		return v, nil
	}

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
	Offset int
}

func (a addr) OffsetBy(bytes int) addr {
	a.Offset += bytes
	return a
}

func (b *builder) genLValue(expr *ir.Node) (addr, error) {
	switch expr.Op {
	case ir.OpIdent:
		return b.genSymPlace(expr, expr.Sym)
	case ir.OpDeref:
		if len(expr.List) < 1 {
			return addr{}, diagnostic.NewError(expr.Pos, "deref missing argument")
		}
		ptr, err := b.genExpr(expr.List[0])
		if err != nil {
			return addr{}, err
		}
		return addr{Ptr: ptr}, nil
	case ir.OpDot:
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

// genSymPlace is where sym's value lives from inside the function being built
func (b *builder) genSymPlace(expr *ir.Node, sym *ir.Sym) (addr, error) {
	if slot, ok := b.vars[sym]; ok {
		return addr{Slot: slot}, nil
	}
	if slot, ok := b.captures[sym]; ok {
		// a captured variable lives in a box
		return addr{Ptr: b.genLoadFrom(addr{Slot: slot}, types.Pointer(sym.Type))}, nil
	}
	return addr{}, diagnostic.NewError(expr.Pos, "variable missing slot: %s", sym.Name)
}

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
	v.setArgs(dest.Ptr)
	v.Offset = dest.Offset
	return v
}

func (b *builder) genStoreTo(dest addr, val *Value) {
	if val.Type.IsSingleton() {
		return
	}

	if dest.Slot != nil {
		v := b.targetFunc.appendValue(OpStaticStore, val.Type, b.currentBlock)
		v.setArgs(val)
		v.Value = dest.Slot
		v.Offset = dest.Offset
		return
	}

	v := b.targetFunc.appendValue(OpStore, val.Type, b.currentBlock)
	v.setArgs(val, dest.Ptr)
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

	// naming a global means this function is statically known
	callee := expr.List[0]
	if callee.Op == ir.OpIdent && callee.Sym.Kind == ir.SymFunc {
		return b.genStaticCall(expr, callee)
	}

	return b.genClosureCall(expr, callee)
}

func (b *builder) genStaticCall(expr *ir.Node, callee *ir.Node) (*Value, error) {
	target := b.module.lookup(callee.Sym.Name)
	if target == nil {
		return nil, diagnostic.NewError(callee.Pos, "reference to unknown function: %s", callee.Sym.Name)
	}

	argVals, err := b.genCallArgs(expr)
	if err != nil {
		return nil, err
	}

	v := b.targetFunc.appendValue(OpStaticCall, expr.Type, b.currentBlock)
	v.Value = target
	v.setArgs(argVals...)

	return v, nil
}

// genClosureCall calls the code pointer a closure carries, handing it the closure's environment
func (b *builder) genClosureCall(expr *ir.Node, callee *ir.Node) (*Value, error) {
	closure, err := b.genArg(callee)
	if err != nil {
		return nil, err
	}

	argVals, err := b.genCallArgs(expr)
	if err != nil {
		return nil, err
	}

	v := b.targetFunc.appendValue(OpClosureCall, expr.Type, b.currentBlock)
	args := make([]*Value, 2+len(argVals))
	args[0] = closure[0]
	args[1] = closure[1]
	copy(args[2:], argVals)
	v.setArgs(args...)

	return v, nil
}

// genCallArgs flattens every argument of a call into ABI order.
func (b *builder) genCallArgs(expr *ir.Node) ([]*Value, error) {
	var argVals []*Value
	for _, arg := range expr.List[1:] {
		vals, err := b.genArg(arg)
		if err != nil {
			return nil, err
		}
		argVals = append(argVals, vals...)
	}

	return argVals, nil
}

func (b *builder) genCallResults(expr *ir.Node) ([]*Value, error) {
	call, err := b.genCall(expr)
	if err != nil {
		return nil, err
	}

	var vals []*Value
	for i, leaf := range iterutil.Enumerate(iterutil.Second(expr.Type.Leaves())) {
		// results store their index in the value slot
		res := b.targetFunc.appendValue(OpCallResult, leaf, b.currentBlock)
		res.Value = i
		res.setArgs(call)
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

func (b *builder) genNegate(expr *ir.Node) (*Value, error) {
	if len(expr.List) != 1 {
		return nil, diagnostic.NewError(expr.Pos, "negation operator without one operand")
	}

	e, err := b.genExpr(expr.List[0])
	if err != nil {
		return nil, err
	}

	negateOp := b.targetFunc.appendValue(OpNegate, expr.Type, b.currentBlock)
	negateOp.setArgs(e)

	return negateOp, nil
}

func (b *builder) genIdent(expr *ir.Node) (*Value, error) {
	switch expr.Sym.Kind {
	case ir.SymParam, ir.SymLocal:
		place, err := b.genLValue(expr)
		if err != nil {
			return nil, err
		}
		return b.genLoadFrom(place, expr.Type), nil
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

	v := b.targetFunc.appendValue(bopFrom(expr), expr.Type, b.currentBlock)
	v.setArgs(leftVal, rightVal)
	return v, nil
}

func bopFrom(n *ir.Node) Op {
	switch n.Op {
	case ir.OpPlus, ir.OpPlusEq:
		return OpAdd
	case ir.OpMinus, ir.OpMinusEq:
		return OpSubtract
	case ir.OpTimes, ir.OpTimesEq:
		return OpMultiply
	case ir.OpDiv, ir.OpDivEq:
		return OpDivide
	case ir.OpEqual:
		return OpEqual
	case ir.OpNotEqual:
		return OpNotEqual
	case ir.OpLess:
		return OpLess
	case ir.OpLessEqual:
		return OpLessEqual
	case ir.OpGreater:
		return OpGreater
	case ir.OpGreaterEqual:
		return OpGreaterEqual
	default:
		return OpUnknown
	}
}

func (b *builder) jump(to *Block) {
	addEdge(b.currentBlock, to)
	b.currentBlock.Kind = BlockPlain
}

func addEdge(from *Block, to *Block) {
	from.Successors = append(from.Successors, to)
	to.Predecessors = append(to.Predecessors, from)
}
