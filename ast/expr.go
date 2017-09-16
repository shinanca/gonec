package ast

import (
	"reflect"
	"strings"

	"github.com/covrom/gonec/bincode/binstmt"
	"github.com/covrom/gonec/builtins"
	"github.com/covrom/gonec/pos"
)

// Expr provides all of interfaces for expression.
type Expr interface {
	pos.Pos
	expr()
	Simplify() Expr
	BinTo(*binstmt.BinStmts, int, *int, bool)
}

type CanLetExpr interface {
	Expr
	BinLetTo(*binstmt.BinStmts, int, *int)
}

// ExprImpl provide commonly implementations for Expr.
type ExprImpl struct {
	pos.PosImpl // ExprImpl provide Pos() function.
}

// expr provide restraint interface.
func (x *ExprImpl) expr() {}

// отсутствующее выражение, используется для пропущенных значений в диапазонах
type NoneExpr struct {
	ExprImpl
}

func (x *NoneExpr) Simplify() Expr { return x }
func (e *NoneExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	bins.Append(binstmt.NewBinLOAD(reg, nil, false, e))
}

// NumberExpr provide Number expression.
type NumberExpr struct {
	ExprImpl
	Lit string
}

func (x *NumberExpr) Simplify() Expr {
	var rv core.VMValuer
	if strings.ContainsAny(x.Lit, ".eE") {
		v := &core.VMDecimal{}
		if err := v.Parse(x.Lit); err != nil {
			return x
		}
		rv = *v
	} else {
		v := new(core.VMInt)
		if err := v.Parse(x.Lit); err != nil {
			return x
		}
		rv = *v
	}
	return &NativeExpr{Value: rv}
}

func (e *NumberExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	// команда на загрузку строки в регистр и ее преобразование в число, в регистр reg
	bins.Append(binstmt.NewBinLOAD(reg, core.VMString(e.Lit), false, e))
	bins.Append(binstmt.NewBinCASTNUM(reg, e))
}

// StringExpr provide String expression.
type StringExpr struct {
	ExprImpl
	Lit string
}

func (x *StringExpr) Simplify() Expr {
	return &NativeExpr{Value: core.VMString(x.Lit)}
}

func (e *StringExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	bins.Append(binstmt.NewBinLOAD(reg, core.VMString(e.Lit), false, e))
}

// ArrayExpr provide Array expression.
type ArrayExpr struct {
	ExprImpl
	Exprs []Expr
}

func (x *ArrayExpr) Simplify() Expr {
	waserrors := false
	a := make(core.VMSlice, len(x.Exprs))
	for i := range x.Exprs {
		x.Exprs[i] = x.Exprs[i].Simplify()
		if v, ok := x.Exprs[i].(*NativeExpr); ok {
			a[i] = v.Value
		} else {
			waserrors = true
		}
	}
	if waserrors {
		return x
	} else {
		return &NativeExpr{Value: a}
	}
}

func (e *ArrayExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	// создание слайса
	bins.Append(binstmt.NewBinMAKESLICE(reg, len(e.Exprs), len(e.Exprs), e))

	for i, ee := range e.Exprs {
		// каждое выражение сохраняем в следующем по номеру регистре (относительно регистра слайса)
		ee.BinTo(bins, reg+1, lid, false)
		bins.Append(binstmt.NewBinSETIDX(reg, i, reg+1, ee))
	}
}

// PairExpr provide one of Map key/value pair.
type PairExpr struct {
	ExprImpl
	Key   string
	Value Expr
}

func (x *PairExpr) Simplify() Expr {
	x.Value = x.Value.Simplify()
	return x
}

func (e *PairExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {}

// MapExpr provide Map expression.
type MapExpr struct {
	ExprImpl
	MapExpr map[string]Expr
}

func (x *MapExpr) Simplify() Expr {
	waserrors := false
	m := make(core.VMStringMap)
	for k, v := range x.MapExpr {
		vv := v.Simplify()
		x.MapExpr[k] = vv
		if arg, ok := vv.(*NativeExpr); ok {
			m[k] = arg.Value
		} else {
			waserrors = true
		}
	}
	if waserrors {
		return x
	} else {
		return &NativeExpr{Value: m}
	}
}

func (e *MapExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	// создание мапы
	bins.Append(binstmt.NewBinMAKEMAP(reg, len(e.MapExpr), e))

	for k, ee := range e.MapExpr {
		// каждое выражение сохраняем в следующем по номеру регистре (относительно регистра слайса)
		ee.BinTo(bins, reg+1, lid, false)
		bins.Append(binstmt.NewBinSETKEY(reg, reg+1, k, ee))
	}
}

// IdentExpr provide identity expression.
type IdentExpr struct {
	ExprImpl
	Lit string
	Id  int
}

func (x *IdentExpr) Simplify() Expr { return x }

func (e *IdentExpr) BinLetTo(bins *binstmt.BinStmts, reg int, lid *int) {
	bins.Append(binstmt.NewBinSET(reg, e.Id, e))
}

func (e *IdentExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	bins.Append(binstmt.NewBinGET(reg, e.Id, e))
}

// UnaryExpr provide unary minus expression. ex: -1, ^1, ~1.
type UnaryExpr struct {
	ExprImpl
	Operator string
	Expr     Expr
}

func (x *UnaryExpr) Simplify() Expr {
	x.Expr = x.Expr.Simplify()
	if v, ok := x.Expr.(*NativeExpr); ok {
		if vv := v.Value.(core.VMUnarer); ok {
			oper := rune(x.Operator[0])
			rv, err := vv.EvalUnOp(oper)
			if err == nil {
				return &NativeExpr{Value: rv}
			}
		}
	}
	return x
}

func (e *UnaryExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	e.Expr.BinTo(bins, reg, lid, false)
	bins.Append(binstmt.NewBinUNARY(reg, rune(e.Operator[0]), e))
}

// AddrExpr provide referencing address expression.
type AddrExpr struct {
	ExprImpl
	Expr Expr
}

func (x *AddrExpr) Simplify() Expr {
	x.Expr = x.Expr.Simplify()
	return x
}

func (e *AddrExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	switch ee := e.Expr.(type) {
	case *IdentExpr:
		bins.Append(binstmt.NewBinADDRID(reg, ee.Id, e))
	case *MemberExpr:
		ee.Expr.BinTo(bins, reg, lid, false)
		bins.Append(binstmt.NewBinADDRMBR(reg, ee.Name, e))
	default:
		panic(binstmt.NewStringError(e, "Неверная операция над значением"))
	}
}

// DerefExpr provide dereferencing address expression.
type DerefExpr struct {
	ExprImpl
	Expr Expr
}

func (x *DerefExpr) Simplify() Expr {
	x.Expr = x.Expr.Simplify()
	return x
}

func (e *DerefExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	switch ee := e.Expr.(type) {
	case *IdentExpr:
		bins.Append(binstmt.NewBinUNREFID(reg, ee.Id, e))
	case *MemberExpr:
		ee.Expr.BinTo(bins, reg, lid, false)
		bins.Append(binstmt.NewBinUNREFMBR(reg, ee.Name, e))
	default:
		panic(binstmt.NewStringError(e, "Неверная операция над значением"))
	}
}

// ParenExpr provide parent block expression.
type ParenExpr struct {
	ExprImpl
	SubExpr Expr
}

func (x *ParenExpr) Simplify() Expr {
	x.SubExpr = x.SubExpr.Simplify()
	if arg, ok := x.SubExpr.(*NativeExpr); ok {
		return arg
	}
	return x
}

func (e *ParenExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	e.SubExpr.BinTo(bins, reg, lid, false)
}

// BinOpExpr provide binary operator expression.
type BinOpExpr struct {
	ExprImpl
	Lhss     []Expr
	Operator string
	Rhss     []Expr
}

func (x *BinOpExpr) Simplify() Expr {
	allnative := true
	for i := range x.Lhss {
		x.Lhss[i] = x.Lhss[i].Simplify()
		if _, ok := x.Lhss[i].(*NativeExpr); !ok {
			allnative = false
		}
	}
	for i := range x.Rhss {
		x.Rhss[i] = x.Rhss[i].Simplify()
		if _, ok := x.Rhss[i].(*NativeExpr); !ok {
			allnative = false
		}
	}
	if len(x.Lhss) == 1 && len(x.Rhss) == 1 && allnative {
		if x1, ok := x.Lhss[0].(*NativeExpr).Value.(core.VMOperationer); ok {
			if x2, ok := x.Rhss[0].(*NativeExpr).Value.(core.VMOperationer); ok {
				oper := core.OperMap[x.Operator]
				rv, err := x1.EvalBinOp(oper, x2)
				if err == nil {
					return &NativeExpr{Value: rv}
				}
			}
		}
	}
	return x
}

func (e *BinOpExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {

	oper := core.OperMap[e.Operator]
	// если это равенство в контексте исполнения блока кода, то это присваивание, а не вычисление выражения
	if inStmt && oper == core.EQL {
		(&LetsStmt{
			Lhss:     e.Lhss,
			Operator: "=",
			Rhss:     e.Rhss,
		}).BinTo(bins, reg, lid)
		return
	}
	if len(e.Lhss) != 1 || len(e.Rhss) != 1 {
		panic(binstmt.NewStringError(e, "С каждой стороны операции может быть только одно выражение"))
	}
	// сначала вычисляем левую часть
	e.Lhss[0].BinTo(bins, reg, lid, false)
	switch oper {
	case core.LOR:
		*lid++
		lab := *lid
		// вставляем проверку на истину слева и возвращаем ее, не вычисляя правую часть, иначе возвращаем правую часть
		bins.Append(binstmt.NewBinJTRUE(reg, lab, e))
		e.Rhss[0].BinTo(bins, reg, lid, false)
		bins.Append(binstmt.NewBinLABEL(lab, e))
	case core.LAND:
		*lid++
		lab := *lid
		// вставляем проверку на ложь слева и возвращаем ее, не вычисляя правую часть, иначе возвращаем правую часть
		bins.Append(binstmt.NewBinJFALSE(reg, lab, e))
		e.Rhss[0].BinTo(bins, reg, lid, false)
		bins.Append(binstmt.NewBinLABEL(lab, e))
	default:
		e.Rhss[0].BinTo(bins, reg+1, lid, false)
		bins.Append(binstmt.NewBinOPER(reg, reg+1, oper, e))
	}
}

type TernaryOpExpr struct {
	ExprImpl
	Expr Expr
	Lhs  Expr
	Rhs  Expr
}

func (x *TernaryOpExpr) Simplify() Expr {
	x.Expr = x.Expr.Simplify()
	x.Lhs = x.Expr.Simplify()
	x.Rhs = x.Expr.Simplify()
	if v, ok := x.Expr.(*NativeExpr); ok {
		if b, ok := v.Value.(core.VMBooler); ok {
			if b.Bool() {
				return x.Lhs
			} else {
				return x.Rhs
			}
		}
	}
	return x
}

func (e *TernaryOpExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	e.Expr.BinTo(bins, reg, lid, false)
	*lid++
	lab := *lid
	bins.Append(binstmt.NewBinJFALSE(reg, lab, e))
	// если истина - берем левое выражение
	e.Lhs.BinTo(bins, reg, lid, false)
	// прыгаем в конец
	*lid++
	lend := *lid
	bins.Append(binstmt.NewBinJMP(lend, e))
	// правое выражение
	bins.Append(binstmt.NewBinLABEL(lab, e))
	e.Rhs.BinTo(bins, reg, lid, false)
	bins.Append(binstmt.NewBinLABEL(lend, e))
}

// CallExpr provide calling expression.
type CallExpr struct {
	ExprImpl
	Func     interface{}
	Name     int //string
	SubExprs []Expr
	VarArg   bool
	Go       bool
}

func (x *CallExpr) Simplify() Expr {
	for i := range x.SubExprs {
		x.SubExprs[i] = x.SubExprs[i].Simplify()
	}
	return x
}

func (e *CallExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	// если это анонимный вызов, то в reg сама функция, значит, параметры записываем в reg+1, иначе в reg
	var regoff int
	if e.Name == 0 {
		regoff = 1
	}

	// помещаем аргументы в массив аргументов в reg, если их >1
	var sliceoff int
	if len(e.SubExprs) > 1 {
		bins.Append(binstmt.NewBinMAKESLICE(reg+regoff, len(e.SubExprs), len(e.SubExprs), e))
		sliceoff = 1
	}

	for i, ee := range e.SubExprs {
		// каждое выражение сохраняем в следующем по номеру регистре (относительно регистра слайса)
		ee.BinTo(bins, reg+sliceoff+regoff, lid, false)
		if sliceoff == 1 {
			bins.Append(binstmt.NewBinSETIDX(reg+regoff, i, reg+sliceoff+regoff, ee))
		}
	}

	// для анонимных (Name==0) - в reg будет функция, иначе первый аргумент (см. выше) или слайс аргументов
	bins.Append(binstmt.NewBinCALL(e.Name, len(e.SubExprs), reg, reg, e.VarArg, e.Go, e))
}

// AnonCallExpr provide anonymous calling expression. ex: func(){}().
type AnonCallExpr struct {
	ExprImpl
	Expr     Expr
	SubExprs []Expr
	VarArg   bool
	Go       bool
}

func (x *AnonCallExpr) Simplify() Expr {
	x.Expr = x.Expr.Simplify()
	for i := range x.SubExprs {
		x.SubExprs[i] = x.SubExprs[i].Simplify()
	}
	return x
}

func (e *AnonCallExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	// помещаем в регистр значение функции (тип func, или ссылку на него, или интерфейс с ним)
	e.Expr.BinTo(bins, reg, lid, false)
	// далее аргументы, как при вызове обычной функции
	(&CallExpr{
		Name:     0,
		SubExprs: e.SubExprs,
		VarArg:   e.VarArg,
		Go:       e.Go,
	}).BinTo(bins, reg, lid, false) // передаем именно reg, т.к. он для Name==0 означает функцию, которую надо вызвать в BinCALL
}

// MemberExpr provide expression to refer menber.
type MemberExpr struct {
	ExprImpl
	Expr Expr
	Name int //string
}

func (x *MemberExpr) Simplify() Expr {
	x.Expr = x.Expr.Simplify()
	return x
}

func (e *MemberExpr) BinLetTo(bins *binstmt.BinStmts, reg int, lid *int) {
	e.Expr.BinTo(bins, reg+1, lid, false)
	bins.Append(binstmt.NewBinSETMEMBER(reg+1, e.Name, reg, e))
}

func (e *MemberExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	e.Expr.BinTo(bins, reg, lid, false)
	bins.Append(binstmt.NewBinGETMEMBER(reg, e.Name, e))
}

// ItemExpr provide expression to refer Map/Array item.
type ItemExpr struct {
	ExprImpl
	Value Expr
	Index Expr
}

func (x *ItemExpr) Simplify() Expr {
	x.Value = x.Value.Simplify()
	x.Index = x.Index.Simplify()
	if v, ok := x.Value.(*NativeExpr); ok {
		if i, ok := x.Index.(*NativeExpr); ok {
			if vv, ok := v.Value.(core.VMSlicer); ok {
				if ii, ok := i.Value.(core.VMInt); ok {
					return &NativeExpr{Value: vv.Slice()[ii.Int()]}
				}
			}
			if vv, ok := v.Value.(core.VMStringMaper); ok {
				if ii, ok := i.Value.(core.VMString); ok {
					return &NativeExpr{Value: vv.StringMap()[ii.String()]}
				}
			}
		}
	}
	return x
}

func (e *ItemExpr) BinLetTo(bins *binstmt.BinStmts, reg int, lid *int) {

	*lid++
	lend := *lid
	e.Value.BinTo(bins, reg+1, lid, false)
	e.Index.BinTo(bins, reg+2, lid, false)
	bins.Append(binstmt.NewBinSETITEM(reg+1, reg+2, reg, reg+3, e))
	bins.Append(binstmt.NewBinJFALSE(reg+3, lend, e))
	ee := e.Value.(CanLetExpr)
	ee.BinLetTo(bins, reg+1, lid)
	bins.Append(binstmt.NewBinLABEL(lend, e))
}

func (e *ItemExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	e.Value.BinTo(bins, reg, lid, false)
	e.Index.BinTo(bins, reg+1, lid, false)
	bins.Append(binstmt.NewBinGETIDX(reg, reg+1, e))
}

// SliceExpr provide expression to refer slice of Array.
type SliceExpr struct {
	ExprImpl
	Value Expr
	Begin Expr
	End   Expr
}

func (x *SliceExpr) Simplify() Expr {
	x.Value = x.Value.Simplify()
	x.Begin = x.Begin.Simplify()
	x.End = x.End.Simplify()
	if v, ok := x.Value.(*NativeExpr); ok {
		if ib, ok := x.Begin.(*NativeExpr); ok {
			if ie, ok := x.End.(*NativeExpr); ok {
				if vv, ok := v.Value.(core.VMSlicer); ok {
					if iib, ok := ib.Value.(core.VMInt); ok {
						if iie, ok := ie.Value.(core.VMInt); ok {
							return &NativeExpr{Value: vv.Slice()[iib.Int():iie.Int()]}
						}
					}
				}
			}
		}
	}
	return x
}

func (e *SliceExpr) BinLetTo(bins *binstmt.BinStmts, reg int, lid *int) {
	*lid++
	lend := *lid
	e.Value.BinTo(bins, reg+1, lid, false)
	e.Begin.BinTo(bins, reg+2, lid, false)
	e.End.BinTo(bins, reg+3, lid, false)
	bins.Append(binstmt.NewBinSETSLICE(reg+1, reg+2, reg+3, reg, reg+4, e))

	bins.Append(binstmt.NewBinJFALSE(reg+4, lend, e))
	ee := e.Value.(CanLetExpr)
	ee.BinLetTo(bins, reg+1, lid)
	bins.Append(binstmt.NewBinLABEL(lend, e))
}

func (e *SliceExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	e.Value.BinTo(bins, reg, lid, false)
	e.Begin.BinTo(bins, reg+1, lid, false)
	e.End.BinTo(bins, reg+2, lid, false)
	bins.Append(binstmt.NewBinGETSUBSLICE(reg, reg+1, reg+2, e))
}

// FuncExpr provide function expression.
type FuncExpr struct {
	ExprImpl
	Name   int //string
	Stmts  Stmts
	Args   []int //string
	VarArg bool
}

func (x *FuncExpr) Simplify() Expr {
	for i := range x.Stmts {
		x.Stmts[i].Simplify()
	}
	return x
}

func (e *FuncExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	*lid++
	lstart := *lid
	*lid++
	lend := *lid
	bins.Append(binstmt.NewBinFUNC(reg, e.Name, e.Args, e.VarArg, lstart, lend, e))
	bins.Append(binstmt.NewBinLABEL(lstart, e))
	e.Stmts.BinTo(bins, reg, lid)
	bins.Append(binstmt.NewBinLABEL(lend, e))
}

// LetExpr provide expression to let variable.
type LetExpr struct {
	ExprImpl
	Lhs Expr
	Rhs Expr
}

func (x *LetExpr) Simplify() Expr {
	x.Lhs = x.Lhs.Simplify()
	x.Rhs = x.Rhs.Simplify()
	return x
}

func (e *LetExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	e.Rhs.BinTo(bins, reg, lid, false)
	e.Lhs.(CanLetExpr).BinLetTo(bins, reg, lid)
}

// LetsExpr provide multiple expression of let.
// type LetsExpr struct {
// 	ExprImpl
// 	Lhss     []Expr
// 	Operator string
// 	Rhss     []Expr
// }

// AssocExpr provide expression to assoc operation.
type AssocExpr struct {
	ExprImpl
	Lhs      Expr
	Operator string
	Rhs      Expr
}

func (x *AssocExpr) Simplify() Expr {
	x.Lhs = x.Lhs.Simplify()
	x.Rhs = x.Rhs.Simplify()
	return x
}

// NewExpr provide expression to make new instance.
// type NewExpr struct {
// 	ExprImpl
// 	Type int //string
// }

// ConstExpr provide expression for constant variable.
type ConstExpr struct {
	ExprImpl
	Value string
}

func (x *ConstExpr) Simplify() Expr {
	switch strings.ToLower(x.Value) {
	case "истина", "true":
		return &NativeExpr{Value: core.VMBool(true)}
	case "ложь", "false":
		return &NativeExpr{Value: core.VMBool(false)}
	case "null":
		return &NativeExpr{Value: core.VMNullVar}
	}
	return x
}

func (e *ConstExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	var v core.VMValuer

	switch strings.ToLower(e.Value) {
	case "истина", "true":
		v = core.VMBool(true)
	case "ложь", "false":
		v = core.VMBool(false)
	case "null":
		v = core.VMNullVar
	default:
		v = core.VMNil
	}

	bins.Append(binstmt.NewBinLOAD(reg, v, false, e))
}

type ChanExpr struct {
	ExprImpl
	Lhs Expr
	Rhs Expr
}

func (x *ChanExpr) Simplify() Expr {
	x.Lhs = x.Lhs.Simplify()
	x.Rhs = x.Rhs.Simplify()
	return x
}

func (e *ChanExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	// определяем значение справа
	e.Rhs.BinTo(bins, reg+1, lid, false)
	if e.Lhs == nil {
		// слева нет значения - это временное чтение из канала без сохранения значения в переменной
		bins.Append(binstmt.NewBinCHANRECV(reg+1, reg, e))
	} else {
		// значение слева
		e.Lhs.BinTo(bins, reg+2, lid, false)
		bins.Append(binstmt.NewBinMV(reg+2, reg+3, e))
		// слева канал - пишем в него правое
		bins.Append(binstmt.NewBinISKIND(reg+3, reflect.Chan, e))
		*lid++
		li := *lid
		bins.Append(binstmt.NewBinJFALSE(reg+3, li, e))
		bins.Append(binstmt.NewBinCHANSEND(reg+2, reg+1, e))
		bins.Append(binstmt.NewBinLOAD(reg, core.VMBool(true), false, e))

		*lid++
		li2 := *lid

		bins.Append(binstmt.NewBinJMP(li2, e))

		// иначе справа канал, а слева переменная (установим, если прочитали из канала)
		bins.Append(binstmt.NewBinLABEL(li, e))
		bins.Append(binstmt.NewBinCHANRECV(reg+1, reg, e))
		e.Lhs.(CanLetExpr).BinLetTo(bins, reg, lid)

		bins.Append(binstmt.NewBinLABEL(li2, e))
	}
}

type Type struct {
	Name int //string
}

type TypeCast struct {
	ExprImpl
	Type     int
	TypeExpr Expr // должен быть строкой
	CastExpr Expr
}

func (x *TypeCast) Simplify() Expr {
	x.TypeExpr = x.TypeExpr.Simplify()
	x.CastExpr = x.CastExpr.Simplify()
	return x
}

func (e *TypeCast) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	e.CastExpr.BinTo(bins, reg, lid, false)
	if e.TypeExpr == nil {
		bins.Append(binstmt.NewBinLOAD(reg+1, core.VMInt(e.Type), true, e))
	} else {
		e.TypeExpr.BinTo(bins, reg+1, lid, false)
		bins.Append(binstmt.NewBinSETNAME(reg+1, e))
	}
	bins.Append(binstmt.NewBinCASTTYPE(reg, reg+1, e))
}

type MakeExpr struct {
	ExprImpl
	Type     int  //string
	TypeExpr Expr // должен быть строкой
}

func (x *MakeExpr) Simplify() Expr {
	x.TypeExpr = x.TypeExpr.Simplify()
	return x
}

func (e *MakeExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	if e.TypeExpr == nil {
		bins.Append(binstmt.NewBinLOAD(reg, core.VMInt(e.Type), true, e))
	} else {
		e.TypeExpr.BinTo(bins, reg, lid, false)
		bins.Append(binstmt.NewBinSETNAME(reg, e))
	}
	bins.Append(binstmt.NewBinMAKE(reg, e))
}

type MakeChanExpr struct {
	ExprImpl
	// Type     int //string
	SizeExpr Expr
}

func (x *MakeChanExpr) Simplify() Expr {
	x.SizeExpr = x.SizeExpr.Simplify()
	return x
}

func (e *MakeChanExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	if e.SizeExpr == nil {
		bins.Append(binstmt.NewBinLOAD(reg, core.VMInt(0), false, e))
	} else {
		e.SizeExpr.BinTo(bins, reg, lid, false)
	}
	bins.Append(binstmt.NewBinMAKECHAN(reg, e))
}

type MakeArrayExpr struct {
	ExprImpl
	// Type    int //string
	LenExpr Expr
	CapExpr Expr
}

func (x *MakeArrayExpr) Simplify() Expr {
	x.LenExpr = x.LenExpr.Simplify()
	x.CapExpr = x.CapExpr.Simplify()
	return x
}

func (e *MakeArrayExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	e.LenExpr.BinTo(bins, reg, lid, false)
	if e.CapExpr == nil {
		bins.Append(binstmt.NewBinMV(reg, reg+1, e))
	} else {
		e.CapExpr.BinTo(bins, reg+1, lid, false)
	}
	bins.Append(binstmt.NewBinMAKEARR(reg, reg+1, e))
}

// хранит реальное значение, рассчитанное на этапе оптимизации AST
type NativeExpr struct {
	ExprImpl
	Value core.VMValuer
}

func (x *NativeExpr) Simplify() Expr {
	return x
}

func (e *NativeExpr) BinTo(bins *binstmt.BinStmts, reg int, lid *int, inStmt bool) {
	bins.Append(binstmt.NewBinLOAD(reg, e.Value, false, e))
}
