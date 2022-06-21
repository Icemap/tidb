// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// We implement 6 CastAsXXFunctionClass for `cast` built-in functions.
// XX means the return type of the `cast` built-in functions.
// XX contains the following 6 types:
// Int, decimal, Real, String, Time, Duration.

// We implement 6 CastYYAsXXSig built-in function signatures for every CastAsXXFunctionClass.
// builtinCastXXAsYYSig takes a argument of type XX and returns a value of type YY.

package expression

import (
	"math"
	"strconv"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/charset"
	"github.com/pingcap/tidb/parser/model"
	"github.com/pingcap/tidb/parser/mysql"
	"github.com/pingcap/tidb/parser/terror"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/types/json"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tipb/go-tipb"
)

var (
	_ functionClass = &castAsIntFunctionClass{}
	_ functionClass = &castAsRealFunctionClass{}
	_ functionClass = &castAsStringFunctionClass{}
	_ functionClass = &castAsDecimalFunctionClass{}
	_ functionClass = &castAsTimeFunctionClass{}
	_ functionClass = &castAsDurationFunctionClass{}
	_ functionClass = &castAsJSONFunctionClass{}
)

var (
	_ builtinFunc = &builtinCastIntAsIntSig{}
	_ builtinFunc = &builtinCastIntAsRealSig{}
	_ builtinFunc = &builtinCastIntAsStringSig{}
	_ builtinFunc = &builtinCastIntAsDecimalSig{}
	_ builtinFunc = &builtinCastIntAsTimeSig{}
	_ builtinFunc = &builtinCastIntAsDurationSig{}
	_ builtinFunc = &builtinCastIntAsJSONSig{}

	_ builtinFunc = &builtinCastRealAsIntSig{}
	_ builtinFunc = &builtinCastRealAsRealSig{}
	_ builtinFunc = &builtinCastRealAsStringSig{}
	_ builtinFunc = &builtinCastRealAsDecimalSig{}
	_ builtinFunc = &builtinCastRealAsTimeSig{}
	_ builtinFunc = &builtinCastRealAsDurationSig{}
	_ builtinFunc = &builtinCastRealAsJSONSig{}

	_ builtinFunc = &builtinCastDecimalAsIntSig{}
	_ builtinFunc = &builtinCastDecimalAsRealSig{}
	_ builtinFunc = &builtinCastDecimalAsStringSig{}
	_ builtinFunc = &builtinCastDecimalAsDecimalSig{}
	_ builtinFunc = &builtinCastDecimalAsTimeSig{}
	_ builtinFunc = &builtinCastDecimalAsDurationSig{}
	_ builtinFunc = &builtinCastDecimalAsJSONSig{}

	_ builtinFunc = &builtinCastStringAsIntSig{}
	_ builtinFunc = &builtinCastStringAsRealSig{}
	_ builtinFunc = &builtinCastStringAsStringSig{}
	_ builtinFunc = &builtinCastStringAsDecimalSig{}
	_ builtinFunc = &builtinCastStringAsTimeSig{}
	_ builtinFunc = &builtinCastStringAsDurationSig{}
	_ builtinFunc = &builtinCastStringAsJSONSig{}

	_ builtinFunc = &builtinCastTimeAsIntSig{}
	_ builtinFunc = &builtinCastTimeAsRealSig{}
	_ builtinFunc = &builtinCastTimeAsStringSig{}
	_ builtinFunc = &builtinCastTimeAsDecimalSig{}
	_ builtinFunc = &builtinCastTimeAsTimeSig{}
	_ builtinFunc = &builtinCastTimeAsDurationSig{}
	_ builtinFunc = &builtinCastTimeAsJSONSig{}

	_ builtinFunc = &builtinCastDurationAsIntSig{}
	_ builtinFunc = &builtinCastDurationAsRealSig{}
	_ builtinFunc = &builtinCastDurationAsStringSig{}
	_ builtinFunc = &builtinCastDurationAsDecimalSig{}
	_ builtinFunc = &builtinCastDurationAsTimeSig{}
	_ builtinFunc = &builtinCastDurationAsDurationSig{}
	_ builtinFunc = &builtinCastDurationAsJSONSig{}

	_ builtinFunc = &builtinCastJSONAsIntSig{}
	_ builtinFunc = &builtinCastJSONAsRealSig{}
	_ builtinFunc = &builtinCastJSONAsStringSig{}
	_ builtinFunc = &builtinCastJSONAsDecimalSig{}
	_ builtinFunc = &builtinCastJSONAsTimeSig{}
	_ builtinFunc = &builtinCastJSONAsDurationSig{}
	_ builtinFunc = &builtinCastJSONAsJSONSig{}
)

type castAsIntFunctionClass struct {
	baseFunctionClass

	tp *types.FieldType
}

func (c *castAsIntFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, err
	}
	b, err := newBaseBuiltinFunc(ctx, c.funcName, args, c.tp.EvalType())
	if err != nil {
		return nil, err
	}
	bf := newBaseBuiltinCastFunc(b, ctx.Value(inUnionCastContext) != nil)
	bf.tp = c.tp
	if args[0].GetType().Hybrid() || IsBinaryLiteral(args[0]) {
		sig = &builtinCastIntAsIntSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastIntAsInt)
		return sig, nil
	}
	argTp := args[0].GetType().EvalType()
	switch argTp {
	case types.ETInt:
		sig = &builtinCastIntAsIntSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastIntAsInt)
	case types.ETReal:
		sig = &builtinCastRealAsIntSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastRealAsInt)
	case types.ETDecimal:
		sig = &builtinCastDecimalAsIntSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDecimalAsInt)
	case types.ETDatetime, types.ETTimestamp:
		sig = &builtinCastTimeAsIntSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastTimeAsInt)
	case types.ETDuration:
		sig = &builtinCastDurationAsIntSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDurationAsInt)
	case types.ETJson:
		sig = &builtinCastJSONAsIntSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastJsonAsInt)
	case types.ETString:
		sig = &builtinCastStringAsIntSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastStringAsInt)
	default:
		panic("unsupported types.EvalType in castAsIntFunctionClass")
	}
	return sig, nil
}

type castAsRealFunctionClass struct {
	baseFunctionClass

	tp *types.FieldType
}

func (c *castAsRealFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, err
	}
	b, err := newBaseBuiltinFunc(ctx, c.funcName, args, c.tp.EvalType())
	if err != nil {
		return nil, err
	}
	bf := newBaseBuiltinCastFunc(b, ctx.Value(inUnionCastContext) != nil)
	bf.tp = c.tp
	if IsBinaryLiteral(args[0]) {
		sig = &builtinCastRealAsRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastRealAsReal)
		return sig, nil
	}
	var argTp types.EvalType
	if args[0].GetType().Hybrid() {
		argTp = types.ETInt
	} else {
		argTp = args[0].GetType().EvalType()
	}
	switch argTp {
	case types.ETInt:
		sig = &builtinCastIntAsRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastIntAsReal)
	case types.ETReal:
		sig = &builtinCastRealAsRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastRealAsReal)
	case types.ETDecimal:
		sig = &builtinCastDecimalAsRealSig{bf}
		PropagateType(types.ETReal, sig.getArgs()...)
		sig.setPbCode(tipb.ScalarFuncSig_CastDecimalAsReal)
	case types.ETDatetime, types.ETTimestamp:
		sig = &builtinCastTimeAsRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastTimeAsReal)
	case types.ETDuration:
		sig = &builtinCastDurationAsRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDurationAsReal)
	case types.ETJson:
		sig = &builtinCastJSONAsRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastJsonAsReal)
	case types.ETString:
		sig = &builtinCastStringAsRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastStringAsReal)
	default:
		panic("unsupported types.EvalType in castAsRealFunctionClass")
	}
	return sig, nil
}

type castAsDecimalFunctionClass struct {
	baseFunctionClass

	tp *types.FieldType
}

func (c *castAsDecimalFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, err
	}
	b, err := newBaseBuiltinFunc(ctx, c.funcName, args, c.tp.EvalType())
	if err != nil {
		return nil, err
	}
	bf := newBaseBuiltinCastFunc(b, ctx.Value(inUnionCastContext) != nil)
	bf.tp = c.tp
	if IsBinaryLiteral(args[0]) {
		sig = &builtinCastDecimalAsDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDecimalAsDecimal)
		return sig, nil
	}
	var argTp types.EvalType
	if args[0].GetType().Hybrid() {
		argTp = types.ETInt
	} else {
		argTp = args[0].GetType().EvalType()
	}
	switch argTp {
	case types.ETInt:
		sig = &builtinCastIntAsDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastIntAsDecimal)
	case types.ETReal:
		sig = &builtinCastRealAsDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastRealAsDecimal)
	case types.ETDecimal:
		sig = &builtinCastDecimalAsDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDecimalAsDecimal)
	case types.ETDatetime, types.ETTimestamp:
		sig = &builtinCastTimeAsDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastTimeAsDecimal)
	case types.ETDuration:
		sig = &builtinCastDurationAsDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDurationAsDecimal)
	case types.ETJson:
		sig = &builtinCastJSONAsDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastJsonAsDecimal)
	case types.ETString:
		sig = &builtinCastStringAsDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastStringAsDecimal)
	default:
		panic("unsupported types.EvalType in castAsDecimalFunctionClass")
	}
	return sig, nil
}

type castAsStringFunctionClass struct {
	baseFunctionClass

	tp *types.FieldType
}

func (c *castAsStringFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, err
	}
	bf, err := newBaseBuiltinFunc(ctx, c.funcName, args, c.tp.EvalType())
	if err != nil {
		return nil, err
	}
	bf.tp = c.tp
	if args[0].GetType().Hybrid() {
		sig = &builtinCastStringAsStringSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastStringAsString)
		return sig, nil
	}
	argTp := args[0].GetType().EvalType()
	switch argTp {
	case types.ETInt:
		if bf.tp.GetFlen() == types.UnspecifiedLength {
			bf.tp.SetFlen(args[0].GetType().GetFlen())
		}
		sig = &builtinCastIntAsStringSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastIntAsString)
	case types.ETReal:
		sig = &builtinCastRealAsStringSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastRealAsString)
	case types.ETDecimal:
		sig = &builtinCastDecimalAsStringSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDecimalAsString)
	case types.ETDatetime, types.ETTimestamp:
		sig = &builtinCastTimeAsStringSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastTimeAsString)
	case types.ETDuration:
		sig = &builtinCastDurationAsStringSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDurationAsString)
	case types.ETJson:
		sig = &builtinCastJSONAsStringSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastJsonAsString)
	case types.ETString:
		// When cast from binary to some other charsets, we should check if the binary is valid or not.
		// so we build a from_binary function to do this check.
		bf.args[0] = HandleBinaryLiteral(ctx, args[0], &ExprCollation{Charset: c.tp.GetCharset(), Collation: c.tp.GetCollate()}, c.funcName)
		sig = &builtinCastStringAsStringSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastStringAsString)
	default:
		panic("unsupported types.EvalType in castAsStringFunctionClass")
	}
	return sig, nil
}

type castAsTimeFunctionClass struct {
	baseFunctionClass

	tp *types.FieldType
}

func (c *castAsTimeFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, err
	}
	bf, err := newBaseBuiltinFunc(ctx, c.funcName, args, c.tp.EvalType())
	if err != nil {
		return nil, err
	}
	bf.tp = c.tp
	argTp := args[0].GetType().EvalType()
	switch argTp {
	case types.ETInt:
		sig = &builtinCastIntAsTimeSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastIntAsTime)
	case types.ETReal:
		sig = &builtinCastRealAsTimeSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastRealAsTime)
	case types.ETDecimal:
		sig = &builtinCastDecimalAsTimeSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDecimalAsTime)
	case types.ETDatetime, types.ETTimestamp:
		sig = &builtinCastTimeAsTimeSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastTimeAsTime)
	case types.ETDuration:
		sig = &builtinCastDurationAsTimeSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDurationAsTime)
	case types.ETJson:
		sig = &builtinCastJSONAsTimeSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastJsonAsTime)
	case types.ETString:
		sig = &builtinCastStringAsTimeSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastStringAsTime)
	default:
		panic("unsupported types.EvalType in castAsTimeFunctionClass")
	}
	return sig, nil
}

type castAsDurationFunctionClass struct {
	baseFunctionClass

	tp *types.FieldType
}

func (c *castAsDurationFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, err
	}
	bf, err := newBaseBuiltinFunc(ctx, c.funcName, args, c.tp.EvalType())
	if err != nil {
		return nil, err
	}
	bf.tp = c.tp
	argTp := args[0].GetType().EvalType()
	switch argTp {
	case types.ETInt:
		sig = &builtinCastIntAsDurationSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastIntAsDuration)
	case types.ETReal:
		sig = &builtinCastRealAsDurationSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastRealAsDuration)
	case types.ETDecimal:
		sig = &builtinCastDecimalAsDurationSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDecimalAsDuration)
	case types.ETDatetime, types.ETTimestamp:
		sig = &builtinCastTimeAsDurationSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastTimeAsDuration)
	case types.ETDuration:
		sig = &builtinCastDurationAsDurationSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDurationAsDuration)
	case types.ETJson:
		sig = &builtinCastJSONAsDurationSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastJsonAsDuration)
	case types.ETString:
		sig = &builtinCastStringAsDurationSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastStringAsDuration)
	default:
		panic("unsupported types.EvalType in castAsDurationFunctionClass")
	}
	return sig, nil
}

type castAsJSONFunctionClass struct {
	baseFunctionClass

	tp *types.FieldType
}

func (c *castAsJSONFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, err
	}
	bf, err := newBaseBuiltinFunc(ctx, c.funcName, args, c.tp.EvalType())
	if err != nil {
		return nil, err
	}
	bf.tp = c.tp
	argTp := args[0].GetType().EvalType()
	switch argTp {
	case types.ETInt:
		sig = &builtinCastIntAsJSONSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastIntAsJson)
	case types.ETReal:
		sig = &builtinCastRealAsJSONSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastRealAsJson)
	case types.ETDecimal:
		sig = &builtinCastDecimalAsJSONSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDecimalAsJson)
	case types.ETDatetime, types.ETTimestamp:
		sig = &builtinCastTimeAsJSONSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastTimeAsJson)
	case types.ETDuration:
		sig = &builtinCastDurationAsJSONSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastDurationAsJson)
	case types.ETJson:
		sig = &builtinCastJSONAsJSONSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_CastJsonAsJson)
	case types.ETString:
		sig = &builtinCastStringAsJSONSig{bf}
		sig.getRetTp().AddFlag(mysql.ParseToJSONFlag)
		sig.setPbCode(tipb.ScalarFuncSig_CastStringAsJson)
	default:
		panic("unsupported types.EvalType in castAsJSONFunctionClass")
	}
	return sig, nil
}

type builtinCastIntAsIntSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastIntAsIntSig) Clone() builtinFunc {
	newSig := &builtinCastIntAsIntSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastIntAsIntSig) evalInt(row chunk.Row) (res int64, isNull bool, err error) {
	res, isNull, err = b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return
	}
	if b.inUnion && mysql.HasUnsignedFlag(b.tp.GetFlag()) && res < 0 {
		res = 0
	}
	return
}

type builtinCastIntAsRealSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastIntAsRealSig) Clone() builtinFunc {
	newSig := &builtinCastIntAsRealSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastIntAsRealSig) evalReal(row chunk.Row) (res float64, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	if unsignedArgs0 := mysql.HasUnsignedFlag(b.args[0].GetType().GetFlag()); !mysql.HasUnsignedFlag(b.tp.GetFlag()) && !unsignedArgs0 {
		res = float64(val)
	} else if b.inUnion && !unsignedArgs0 && val < 0 {
		// Round up to 0 if the value is negative but the expression eval type is unsigned in `UNION` statement
		// NOTE: the following expressions are equal (so choose the more efficient one):
		// `b.inUnion && mysql.HasUnsignedFlag(b.tp.GetFlag()) && !unsignedArgs0 && val < 0`
		// `b.inUnion && !unsignedArgs0 && val < 0`
		res = 0
	} else {
		// recall that, int to float is different from uint to float
		res = float64(uint64(val))
	}
	return res, false, err
}

type builtinCastIntAsDecimalSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastIntAsDecimalSig) Clone() builtinFunc {
	newSig := &builtinCastIntAsDecimalSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastIntAsDecimalSig) evalDecimal(row chunk.Row) (res *types.MyDecimal, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}

	if unsignedArgs0 := mysql.HasUnsignedFlag(b.args[0].GetType().GetFlag()); !mysql.HasUnsignedFlag(b.tp.GetFlag()) && !unsignedArgs0 {
		res = types.NewDecFromInt(val)
		// Round up to 0 if the value is negative but the expression eval type is unsigned in `UNION` statement
		// NOTE: the following expressions are equal (so choose the more efficient one):
		// `b.inUnion && mysql.HasUnsignedFlag(b.tp.GetFlag()) && !unsignedArgs0 && val < 0`
		// `b.inUnion && !unsignedArgs0 && val < 0`
	} else if b.inUnion && !unsignedArgs0 && val < 0 {
		res = &types.MyDecimal{}
	} else {
		res = types.NewDecFromUint(uint64(val))
	}
	res, err = types.ProduceDecWithSpecifiedTp(res, b.tp, b.ctx.GetSessionVars().StmtCtx)
	return res, isNull, err
}

type builtinCastIntAsStringSig struct {
	baseBuiltinFunc
}

func (b *builtinCastIntAsStringSig) Clone() builtinFunc {
	newSig := &builtinCastIntAsStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastIntAsStringSig) evalString(row chunk.Row) (res string, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	tp := b.args[0].GetType()
	if !mysql.HasUnsignedFlag(tp.GetFlag()) {
		res = strconv.FormatInt(val, 10)
	} else {
		res = strconv.FormatUint(uint64(val), 10)
	}
	if tp.GetType() == mysql.TypeYear && res == "0" {
		res = "0000"
	}
	res, err = types.ProduceStrWithSpecifiedTp(res, b.tp, b.ctx.GetSessionVars().StmtCtx, false)
	if err != nil {
		return res, false, err
	}
	return padZeroForBinaryType(res, b.tp, b.ctx)
}

type builtinCastIntAsTimeSig struct {
	baseBuiltinFunc
}

func (b *builtinCastIntAsTimeSig) Clone() builtinFunc {
	newSig := &builtinCastIntAsTimeSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastIntAsTimeSig) evalTime(row chunk.Row) (res types.Time, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}

	if b.args[0].GetType().GetType() == mysql.TypeYear {
		res, err = types.ParseTimeFromYear(b.ctx.GetSessionVars().StmtCtx, val)
	} else {
		res, err = types.ParseTimeFromNum(b.ctx.GetSessionVars().StmtCtx, val, b.tp.GetType(), b.tp.GetDecimal())
	}

	if err != nil {
		return types.ZeroTime, true, handleInvalidTimeError(b.ctx, err)
	}
	if b.tp.GetType() == mysql.TypeDate {
		// Truncate hh:mm:ss part if the type is Date.
		res.SetCoreTime(types.FromDate(res.Year(), res.Month(), res.Day(), 0, 0, 0, 0))
	}
	return res, false, nil
}

type builtinCastIntAsDurationSig struct {
	baseBuiltinFunc
}

func (b *builtinCastIntAsDurationSig) Clone() builtinFunc {
	newSig := &builtinCastIntAsDurationSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastIntAsDurationSig) evalDuration(row chunk.Row) (res types.Duration, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	dur, err := types.NumberToDuration(val, b.tp.GetDecimal())
	if err != nil {
		if types.ErrOverflow.Equal(err) {
			err = b.ctx.GetSessionVars().StmtCtx.HandleOverflow(err, err)
		}
		if types.ErrTruncatedWrongVal.Equal(err) {
			err = b.ctx.GetSessionVars().StmtCtx.HandleTruncate(err)
		}
		return res, true, err
	}
	return dur, false, err
}

type builtinCastIntAsJSONSig struct {
	baseBuiltinFunc
}

func (b *builtinCastIntAsJSONSig) Clone() builtinFunc {
	newSig := &builtinCastIntAsJSONSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastIntAsJSONSig) evalJSON(row chunk.Row) (res json.BinaryJSON, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	if mysql.HasIsBooleanFlag(b.args[0].GetType().GetFlag()) {
		res = json.CreateBinary(val != 0)
	} else if mysql.HasUnsignedFlag(b.args[0].GetType().GetFlag()) {
		res = json.CreateBinary(uint64(val))
	} else {
		res = json.CreateBinary(val)
	}
	return res, false, nil
}

type builtinCastRealAsJSONSig struct {
	baseBuiltinFunc
}

func (b *builtinCastRealAsJSONSig) Clone() builtinFunc {
	newSig := &builtinCastRealAsJSONSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastRealAsJSONSig) evalJSON(row chunk.Row) (res json.BinaryJSON, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalReal(b.ctx, row)
	// FIXME: `select json_type(cast(1111.11 as json))` should return `DECIMAL`, we return `DOUBLE` now.
	return json.CreateBinary(val), isNull, err
}

type builtinCastDecimalAsJSONSig struct {
	baseBuiltinFunc
}

func (b *builtinCastDecimalAsJSONSig) Clone() builtinFunc {
	newSig := &builtinCastDecimalAsJSONSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastDecimalAsJSONSig) evalJSON(row chunk.Row) (json.BinaryJSON, bool, error) {
	val, isNull, err := b.args[0].EvalDecimal(b.ctx, row)
	if isNull || err != nil {
		return json.BinaryJSON{}, true, err
	}
	// FIXME: `select json_type(cast(1111.11 as json))` should return `DECIMAL`, we return `DOUBLE` now.
	f64, err := val.ToFloat64()
	if err != nil {
		return json.BinaryJSON{}, true, err
	}
	return json.CreateBinary(f64), isNull, err
}

type builtinCastStringAsJSONSig struct {
	baseBuiltinFunc
}

func (b *builtinCastStringAsJSONSig) Clone() builtinFunc {
	newSig := &builtinCastStringAsJSONSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastStringAsJSONSig) evalJSON(row chunk.Row) (res json.BinaryJSON, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	if mysql.HasParseToJSONFlag(b.tp.GetFlag()) {
		res, err = json.ParseBinaryFromString(val)
	} else {
		res = json.CreateBinary(val)
	}
	return res, false, err
}

type builtinCastDurationAsJSONSig struct {
	baseBuiltinFunc
}

func (b *builtinCastDurationAsJSONSig) Clone() builtinFunc {
	newSig := &builtinCastDurationAsJSONSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastDurationAsJSONSig) evalJSON(row chunk.Row) (res json.BinaryJSON, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDuration(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	val.Fsp = types.MaxFsp
	return json.CreateBinary(val.String()), false, nil
}

type builtinCastTimeAsJSONSig struct {
	baseBuiltinFunc
}

func (b *builtinCastTimeAsJSONSig) Clone() builtinFunc {
	newSig := &builtinCastTimeAsJSONSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastTimeAsJSONSig) evalJSON(row chunk.Row) (res json.BinaryJSON, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalTime(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	if val.Type() == mysql.TypeDatetime || val.Type() == mysql.TypeTimestamp {
		val.SetFsp(types.MaxFsp)
	}
	return json.CreateBinary(val.String()), false, nil
}

type builtinCastRealAsRealSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastRealAsRealSig) Clone() builtinFunc {
	newSig := &builtinCastRealAsRealSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastRealAsRealSig) evalReal(row chunk.Row) (res float64, isNull bool, err error) {
	res, isNull, err = b.args[0].EvalReal(b.ctx, row)
	if b.inUnion && mysql.HasUnsignedFlag(b.tp.GetFlag()) && res < 0 {
		res = 0
	}
	return
}

type builtinCastRealAsIntSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastRealAsIntSig) Clone() builtinFunc {
	newSig := &builtinCastRealAsIntSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastRealAsIntSig) evalInt(row chunk.Row) (res int64, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalReal(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	if !mysql.HasUnsignedFlag(b.tp.GetFlag()) {
		res, err = types.ConvertFloatToInt(val, types.IntergerSignedLowerBound(mysql.TypeLonglong), types.IntergerSignedUpperBound(mysql.TypeLonglong), mysql.TypeLonglong)
	} else if b.inUnion && val < 0 {
		res = 0
	} else {
		var uintVal uint64
		sc := b.ctx.GetSessionVars().StmtCtx
		uintVal, err = types.ConvertFloatToUint(sc, val, types.IntergerUnsignedUpperBound(mysql.TypeLonglong), mysql.TypeLonglong)
		res = int64(uintVal)
	}
	if types.ErrOverflow.Equal(err) {
		err = b.ctx.GetSessionVars().StmtCtx.HandleOverflow(err, err)
	}
	return res, isNull, err
}

type builtinCastRealAsDecimalSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastRealAsDecimalSig) Clone() builtinFunc {
	newSig := &builtinCastRealAsDecimalSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastRealAsDecimalSig) evalDecimal(row chunk.Row) (res *types.MyDecimal, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalReal(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	res = new(types.MyDecimal)
	if !b.inUnion || val >= 0 {
		err = res.FromFloat64(val)
		if types.ErrOverflow.Equal(err) {
			warnErr := types.ErrTruncatedWrongVal.GenWithStackByArgs("DECIMAL", b.args[0])
			err = b.ctx.GetSessionVars().StmtCtx.HandleOverflow(err, warnErr)
		} else if types.ErrTruncated.Equal(err) {
			// This behavior is consistent with MySQL.
			err = nil
		}
		if err != nil {
			return res, false, err
		}
	}
	res, err = types.ProduceDecWithSpecifiedTp(res, b.tp, b.ctx.GetSessionVars().StmtCtx)
	return res, false, err
}

type builtinCastRealAsStringSig struct {
	baseBuiltinFunc
}

func (b *builtinCastRealAsStringSig) Clone() builtinFunc {
	newSig := &builtinCastRealAsStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastRealAsStringSig) evalString(row chunk.Row) (res string, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalReal(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}

	bits := 64
	if b.args[0].GetType().GetType() == mysql.TypeFloat {
		// b.args[0].EvalReal() casts the value from float32 to float64, for example:
		// float32(208.867) is cast to float64(208.86700439)
		// If we strconv.FormatFloat the value with 64bits, the result is incorrect!
		bits = 32
	}
	res, err = types.ProduceStrWithSpecifiedTp(strconv.FormatFloat(val, 'f', -1, bits), b.tp, b.ctx.GetSessionVars().StmtCtx, false)
	if err != nil {
		return res, false, err
	}
	return padZeroForBinaryType(res, b.tp, b.ctx)
}

type builtinCastRealAsTimeSig struct {
	baseBuiltinFunc
}

func (b *builtinCastRealAsTimeSig) Clone() builtinFunc {
	newSig := &builtinCastRealAsTimeSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastRealAsTimeSig) evalTime(row chunk.Row) (types.Time, bool, error) {
	val, isNull, err := b.args[0].EvalReal(b.ctx, row)
	if isNull || err != nil {
		return types.ZeroTime, true, err
	}
	// MySQL compatibility: 0 should not be converted to null, see #11203
	fv := strconv.FormatFloat(val, 'f', -1, 64)
	if fv == "0" {
		return types.ZeroTime, false, nil
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err := types.ParseTime(sc, fv, b.tp.GetType(), b.tp.GetDecimal())
	if err != nil {
		return types.ZeroTime, true, handleInvalidTimeError(b.ctx, err)
	}
	if b.tp.GetType() == mysql.TypeDate {
		// Truncate hh:mm:ss part if the type is Date.
		res.SetCoreTime(types.FromDate(res.Year(), res.Month(), res.Day(), 0, 0, 0, 0))
	}
	return res, false, nil
}

type builtinCastRealAsDurationSig struct {
	baseBuiltinFunc
}

func (b *builtinCastRealAsDurationSig) Clone() builtinFunc {
	newSig := &builtinCastRealAsDurationSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastRealAsDurationSig) evalDuration(row chunk.Row) (res types.Duration, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalReal(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	res, err = types.ParseDuration(b.ctx.GetSessionVars().StmtCtx, strconv.FormatFloat(val, 'f', -1, 64), b.tp.GetDecimal())
	if err != nil {
		if types.ErrTruncatedWrongVal.Equal(err) {
			err = b.ctx.GetSessionVars().StmtCtx.HandleTruncate(err)
			// ErrTruncatedWrongVal needs to be considered NULL.
			return res, true, err
		}
	}
	return res, false, err
}

type builtinCastDecimalAsDecimalSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastDecimalAsDecimalSig) Clone() builtinFunc {
	newSig := &builtinCastDecimalAsDecimalSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastDecimalAsDecimalSig) evalDecimal(row chunk.Row) (res *types.MyDecimal, isNull bool, err error) {
	evalDecimal, isNull, err := b.args[0].EvalDecimal(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	res = &types.MyDecimal{}
	if !(b.inUnion && mysql.HasUnsignedFlag(b.tp.GetFlag()) && evalDecimal.IsNegative()) {
		*res = *evalDecimal
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ProduceDecWithSpecifiedTp(res, b.tp, sc)
	return res, false, err
}

type builtinCastDecimalAsIntSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastDecimalAsIntSig) Clone() builtinFunc {
	newSig := &builtinCastDecimalAsIntSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastDecimalAsIntSig) evalInt(row chunk.Row) (res int64, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDecimal(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}

	// Round is needed for both unsigned and signed.
	var to types.MyDecimal
	err = val.Round(&to, 0, types.ModeHalfUp)
	if err != nil {
		return 0, true, err
	}

	if !mysql.HasUnsignedFlag(b.tp.GetFlag()) {
		res, err = to.ToInt()
	} else if b.inUnion && to.IsNegative() {
		res = 0
	} else {
		var uintRes uint64
		uintRes, err = to.ToUint()
		res = int64(uintRes)
	}

	if types.ErrOverflow.Equal(err) {
		warnErr := types.ErrTruncatedWrongVal.GenWithStackByArgs("DECIMAL", val)
		err = b.ctx.GetSessionVars().StmtCtx.HandleOverflow(err, warnErr)
	}

	return res, false, err
}

type builtinCastDecimalAsStringSig struct {
	baseBuiltinFunc
}

func (b *builtinCastDecimalAsStringSig) Clone() builtinFunc {
	newSig := &builtinCastDecimalAsStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastDecimalAsStringSig) evalString(row chunk.Row) (res string, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDecimal(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ProduceStrWithSpecifiedTp(string(val.ToString()), b.tp, sc, false)
	if err != nil {
		return res, false, err
	}
	return padZeroForBinaryType(res, b.tp, b.ctx)
}

type builtinCastDecimalAsRealSig struct {
	baseBuiltinCastFunc
}

func setDataTypeDouble(srcDecimal int) (flen, decimal int) {
	decimal = mysql.NotFixedDec
	flen = floatLength(srcDecimal, decimal)
	return
}

func floatLength(srcDecimal int, decimalPar int) int {
	const dblDIG = 15
	if srcDecimal != mysql.NotFixedDec {
		return dblDIG + 2 + decimalPar
	}
	return dblDIG + 8
}

func (b *builtinCastDecimalAsRealSig) Clone() builtinFunc {
	newSig := &builtinCastDecimalAsRealSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastDecimalAsRealSig) evalReal(row chunk.Row) (res float64, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDecimal(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	if b.inUnion && mysql.HasUnsignedFlag(b.tp.GetFlag()) && val.IsNegative() {
		res = 0
	} else {
		res, err = val.ToFloat64()
	}
	return res, false, err
}

type builtinCastDecimalAsTimeSig struct {
	baseBuiltinFunc
}

func (b *builtinCastDecimalAsTimeSig) Clone() builtinFunc {
	newSig := &builtinCastDecimalAsTimeSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastDecimalAsTimeSig) evalTime(row chunk.Row) (res types.Time, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDecimal(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ParseTimeFromFloatString(sc, string(val.ToString()), b.tp.GetType(), b.tp.GetDecimal())
	if err != nil {
		return types.ZeroTime, true, handleInvalidTimeError(b.ctx, err)
	}
	if b.tp.GetType() == mysql.TypeDate {
		// Truncate hh:mm:ss part if the type is Date.
		res.SetCoreTime(types.FromDate(res.Year(), res.Month(), res.Day(), 0, 0, 0, 0))
	}
	return res, false, err
}

type builtinCastDecimalAsDurationSig struct {
	baseBuiltinFunc
}

func (b *builtinCastDecimalAsDurationSig) Clone() builtinFunc {
	newSig := &builtinCastDecimalAsDurationSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastDecimalAsDurationSig) evalDuration(row chunk.Row) (res types.Duration, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDecimal(b.ctx, row)
	if isNull || err != nil {
		return res, true, err
	}
	res, err = types.ParseDuration(b.ctx.GetSessionVars().StmtCtx, string(val.ToString()), b.tp.GetDecimal())
	if types.ErrTruncatedWrongVal.Equal(err) {
		err = b.ctx.GetSessionVars().StmtCtx.HandleTruncate(err)
		// ErrTruncatedWrongVal needs to be considered NULL.
		return res, true, err
	}
	return res, false, err
}

type builtinCastStringAsStringSig struct {
	baseBuiltinFunc
}

func (b *builtinCastStringAsStringSig) Clone() builtinFunc {
	newSig := &builtinCastStringAsStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastStringAsStringSig) evalString(row chunk.Row) (res string, isNull bool, err error) {
	res, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ProduceStrWithSpecifiedTp(res, b.tp, sc, false)
	if err != nil {
		return res, false, err
	}
	return padZeroForBinaryType(res, b.tp, b.ctx)
}

type builtinCastStringAsIntSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastStringAsIntSig) Clone() builtinFunc {
	newSig := &builtinCastStringAsIntSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

// handleOverflow handles the overflow caused by cast string as int,
// see https://dev.mysql.com/doc/refman/5.7/en/out-of-range-and-overflow.html.
// When an out-of-range value is assigned to an integer column, MySQL stores the value representing the corresponding endpoint of the column data type range. If it is in select statement, it will return the
// endpoint value with a warning.
func (b *builtinCastStringAsIntSig) handleOverflow(origRes int64, origStr string, origErr error, isNegative bool) (res int64, err error) {
	res, err = origRes, origErr
	if err == nil {
		return
	}

	sc := b.ctx.GetSessionVars().StmtCtx
	if types.ErrOverflow.Equal(origErr) {
		if isNegative {
			res = math.MinInt64
		} else {
			uval := uint64(math.MaxUint64)
			res = int64(uval)
		}
		warnErr := types.ErrTruncatedWrongVal.GenWithStackByArgs("INTEGER", origStr)
		err = sc.HandleOverflow(origErr, warnErr)
	}
	return
}

func (b *builtinCastStringAsIntSig) evalInt(row chunk.Row) (res int64, isNull bool, err error) {
	if b.args[0].GetType().Hybrid() || IsBinaryLiteral(b.args[0]) {
		return b.args[0].EvalInt(b.ctx, row)
	}

	// Take the implicit evalInt path if possible.
	if CanImplicitEvalInt(b.args[0]) {
		return b.args[0].EvalInt(b.ctx, row)
	}

	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}

	val = strings.TrimSpace(val)
	isNegative := false
	if len(val) > 1 && val[0] == '-' { // negative number
		isNegative = true
	}

	var ures uint64
	sc := b.ctx.GetSessionVars().StmtCtx
	if !isNegative {
		ures, err = types.StrToUint(sc, val, true)
		res = int64(ures)

		if err == nil && !mysql.HasUnsignedFlag(b.tp.GetFlag()) && ures > uint64(math.MaxInt64) {
			sc.AppendWarning(types.ErrCastAsSignedOverflow)
		}
	} else if b.inUnion && mysql.HasUnsignedFlag(b.tp.GetFlag()) {
		res = 0
	} else {
		res, err = types.StrToInt(sc, val, true)
		if err == nil && mysql.HasUnsignedFlag(b.tp.GetFlag()) {
			// If overflow, don't append this warnings
			sc.AppendWarning(types.ErrCastNegIntAsUnsigned)
		}
	}

	res, err = b.handleOverflow(res, val, err, isNegative)
	return res, false, err
}

type builtinCastStringAsRealSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastStringAsRealSig) Clone() builtinFunc {
	newSig := &builtinCastStringAsRealSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastStringAsRealSig) evalReal(row chunk.Row) (res float64, isNull bool, err error) {
	if IsBinaryLiteral(b.args[0]) {
		return b.args[0].EvalReal(b.ctx, row)
	}

	// Take the implicit evalReal path if possible.
	if CanImplicitEvalReal(b.args[0]) {
		return b.args[0].EvalReal(b.ctx, row)
	}

	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.StrToFloat(sc, val, true)
	if err != nil {
		return 0, false, err
	}
	if b.inUnion && mysql.HasUnsignedFlag(b.tp.GetFlag()) && res < 0 {
		res = 0
	}
	res, err = types.ProduceFloatWithSpecifiedTp(res, b.tp, sc)
	return res, false, err
}

type builtinCastStringAsDecimalSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastStringAsDecimalSig) Clone() builtinFunc {
	newSig := &builtinCastStringAsDecimalSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastStringAsDecimalSig) evalDecimal(row chunk.Row) (res *types.MyDecimal, isNull bool, err error) {
	if IsBinaryLiteral(b.args[0]) {
		return b.args[0].EvalDecimal(b.ctx, row)
	}
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	val = strings.TrimSpace(val)
	isNegative := len(val) > 1 && val[0] == '-'
	res = new(types.MyDecimal)
	sc := b.ctx.GetSessionVars().StmtCtx
	if !(b.inUnion && mysql.HasUnsignedFlag(b.tp.GetFlag()) && isNegative) {
		err = sc.HandleTruncate(res.FromString([]byte(val)))
		if err != nil {
			return res, false, err
		}
	}
	res, err = types.ProduceDecWithSpecifiedTp(res, b.tp, sc)
	return res, false, err
}

type builtinCastStringAsTimeSig struct {
	baseBuiltinFunc
}

func (b *builtinCastStringAsTimeSig) Clone() builtinFunc {
	newSig := &builtinCastStringAsTimeSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastStringAsTimeSig) evalTime(row chunk.Row) (res types.Time, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ParseTime(sc, val, b.tp.GetType(), b.tp.GetDecimal())
	if err != nil {
		return types.ZeroTime, true, handleInvalidTimeError(b.ctx, err)
	}
	if res.IsZero() && b.ctx.GetSessionVars().SQLMode.HasNoZeroDateMode() {
		return types.ZeroTime, true, handleInvalidTimeError(b.ctx, types.ErrWrongValue.GenWithStackByArgs(types.DateTimeStr, res.String()))
	}
	if b.tp.GetType() == mysql.TypeDate {
		// Truncate hh:mm:ss part if the type is Date.
		res.SetCoreTime(types.FromDate(res.Year(), res.Month(), res.Day(), 0, 0, 0, 0))
	}
	return res, false, nil
}

type builtinCastStringAsDurationSig struct {
	baseBuiltinFunc
}

func (b *builtinCastStringAsDurationSig) Clone() builtinFunc {
	newSig := &builtinCastStringAsDurationSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastStringAsDurationSig) evalDuration(row chunk.Row) (res types.Duration, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	res, err = types.ParseDuration(b.ctx.GetSessionVars().StmtCtx, val, b.tp.GetDecimal())
	if types.ErrTruncatedWrongVal.Equal(err) {
		sc := b.ctx.GetSessionVars().StmtCtx
		err = sc.HandleTruncate(err)
		// ZeroDuration of error ErrTruncatedWrongVal needs to be considered NULL.
		if res == types.ZeroDuration {
			return res, true, err
		}
	}
	return res, false, err
}

type builtinCastTimeAsTimeSig struct {
	baseBuiltinFunc
}

func (b *builtinCastTimeAsTimeSig) Clone() builtinFunc {
	newSig := &builtinCastTimeAsTimeSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastTimeAsTimeSig) evalTime(row chunk.Row) (res types.Time, isNull bool, err error) {
	res, isNull, err = b.args[0].EvalTime(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}

	sc := b.ctx.GetSessionVars().StmtCtx
	if res, err = res.Convert(sc, b.tp.GetType()); err != nil {
		return types.ZeroTime, true, handleInvalidTimeError(b.ctx, err)
	}
	res, err = res.RoundFrac(sc, b.tp.GetDecimal())
	if b.tp.GetType() == mysql.TypeDate {
		// Truncate hh:mm:ss part if the type is Date.
		res.SetCoreTime(types.FromDate(res.Year(), res.Month(), res.Day(), 0, 0, 0, 0))
		res.SetType(b.tp.GetType())
	}
	return res, false, err
}

type builtinCastTimeAsIntSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastTimeAsIntSig) Clone() builtinFunc {
	newSig := &builtinCastTimeAsIntSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastTimeAsIntSig) evalInt(row chunk.Row) (res int64, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalTime(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	t, err := val.RoundFrac(sc, types.DefaultFsp)
	if err != nil {
		return res, false, err
	}
	res, err = t.ToNumber().ToInt()
	return res, false, err
}

type builtinCastTimeAsRealSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastTimeAsRealSig) Clone() builtinFunc {
	newSig := &builtinCastTimeAsRealSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastTimeAsRealSig) evalReal(row chunk.Row) (res float64, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalTime(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	res, err = val.ToNumber().ToFloat64()
	return res, false, err
}

type builtinCastTimeAsDecimalSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastTimeAsDecimalSig) Clone() builtinFunc {
	newSig := &builtinCastTimeAsDecimalSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastTimeAsDecimalSig) evalDecimal(row chunk.Row) (res *types.MyDecimal, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalTime(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ProduceDecWithSpecifiedTp(val.ToNumber(), b.tp, sc)
	return res, false, err
}

type builtinCastTimeAsStringSig struct {
	baseBuiltinFunc
}

func (b *builtinCastTimeAsStringSig) Clone() builtinFunc {
	newSig := &builtinCastTimeAsStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastTimeAsStringSig) evalString(row chunk.Row) (res string, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalTime(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ProduceStrWithSpecifiedTp(val.String(), b.tp, sc, false)
	if err != nil {
		return res, false, err
	}
	return padZeroForBinaryType(res, b.tp, b.ctx)
}

type builtinCastTimeAsDurationSig struct {
	baseBuiltinFunc
}

func (b *builtinCastTimeAsDurationSig) Clone() builtinFunc {
	newSig := &builtinCastTimeAsDurationSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastTimeAsDurationSig) evalDuration(row chunk.Row) (res types.Duration, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalTime(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	res, err = val.ConvertToDuration()
	if err != nil {
		return res, false, err
	}
	res, err = res.RoundFrac(b.tp.GetDecimal(), b.ctx.GetSessionVars().Location())
	return res, false, err
}

type builtinCastDurationAsDurationSig struct {
	baseBuiltinFunc
}

func (b *builtinCastDurationAsDurationSig) Clone() builtinFunc {
	newSig := &builtinCastDurationAsDurationSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastDurationAsDurationSig) evalDuration(row chunk.Row) (res types.Duration, isNull bool, err error) {
	res, isNull, err = b.args[0].EvalDuration(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	res, err = res.RoundFrac(b.tp.GetDecimal(), b.ctx.GetSessionVars().Location())
	return res, false, err
}

type builtinCastDurationAsIntSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastDurationAsIntSig) Clone() builtinFunc {
	newSig := &builtinCastDurationAsIntSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastDurationAsIntSig) evalInt(row chunk.Row) (res int64, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDuration(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	dur, err := val.RoundFrac(types.DefaultFsp, b.ctx.GetSessionVars().Location())
	if err != nil {
		return res, false, err
	}
	res, err = dur.ToNumber().ToInt()
	return res, false, err
}

type builtinCastDurationAsRealSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastDurationAsRealSig) Clone() builtinFunc {
	newSig := &builtinCastDurationAsRealSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastDurationAsRealSig) evalReal(row chunk.Row) (res float64, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDuration(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	if val.Fsp, err = types.CheckFsp(val.Fsp); err != nil {
		return res, false, err
	}
	res, err = val.ToNumber().ToFloat64()
	return res, false, err
}

type builtinCastDurationAsDecimalSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastDurationAsDecimalSig) Clone() builtinFunc {
	newSig := &builtinCastDurationAsDecimalSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastDurationAsDecimalSig) evalDecimal(row chunk.Row) (res *types.MyDecimal, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDuration(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	if val.Fsp, err = types.CheckFsp(val.Fsp); err != nil {
		return res, false, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ProduceDecWithSpecifiedTp(val.ToNumber(), b.tp, sc)
	return res, false, err
}

type builtinCastDurationAsStringSig struct {
	baseBuiltinFunc
}

func (b *builtinCastDurationAsStringSig) Clone() builtinFunc {
	newSig := &builtinCastDurationAsStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastDurationAsStringSig) evalString(row chunk.Row) (res string, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDuration(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ProduceStrWithSpecifiedTp(val.String(), b.tp, sc, false)
	if err != nil {
		return res, false, err
	}
	return padZeroForBinaryType(res, b.tp, b.ctx)
}

func padZeroForBinaryType(s string, tp *types.FieldType, ctx sessionctx.Context) (string, bool, error) {
	flen := tp.GetFlen()
	if tp.GetType() == mysql.TypeString && types.IsBinaryStr(tp) && len(s) < flen {
		valStr, _ := ctx.GetSessionVars().GetSystemVar(variable.MaxAllowedPacket)
		maxAllowedPacket, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			return "", false, errors.Trace(err)
		}
		if uint64(flen) > maxAllowedPacket {
			return "", true, handleAllowedPacketOverflowed(ctx, "cast_as_binary", maxAllowedPacket)
		}
		padding := make([]byte, flen-len(s))
		s = string(append([]byte(s), padding...))
	}
	return s, false, nil
}

type builtinCastDurationAsTimeSig struct {
	baseBuiltinFunc
}

func (b *builtinCastDurationAsTimeSig) Clone() builtinFunc {
	newSig := &builtinCastDurationAsTimeSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastDurationAsTimeSig) evalTime(row chunk.Row) (res types.Time, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalDuration(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = val.ConvertToTime(sc, b.tp.GetType())
	if err != nil {
		return types.ZeroTime, true, handleInvalidTimeError(b.ctx, err)
	}
	res, err = res.RoundFrac(sc, b.tp.GetDecimal())
	return res, false, err
}

type builtinCastJSONAsJSONSig struct {
	baseBuiltinFunc
}

func (b *builtinCastJSONAsJSONSig) Clone() builtinFunc {
	newSig := &builtinCastJSONAsJSONSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastJSONAsJSONSig) evalJSON(row chunk.Row) (res json.BinaryJSON, isNull bool, err error) {
	return b.args[0].EvalJSON(b.ctx, row)
}

type builtinCastJSONAsIntSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastJSONAsIntSig) Clone() builtinFunc {
	newSig := &builtinCastJSONAsIntSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastJSONAsIntSig) evalInt(row chunk.Row) (res int64, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalJSON(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ConvertJSONToInt64(sc, val, mysql.HasUnsignedFlag(b.tp.GetFlag()))
	return
}

type builtinCastJSONAsRealSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastJSONAsRealSig) Clone() builtinFunc {
	newSig := &builtinCastJSONAsRealSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastJSONAsRealSig) evalReal(row chunk.Row) (res float64, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalJSON(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ConvertJSONToFloat(sc, val)
	return
}

type builtinCastJSONAsDecimalSig struct {
	baseBuiltinCastFunc
}

func (b *builtinCastJSONAsDecimalSig) Clone() builtinFunc {
	newSig := &builtinCastJSONAsDecimalSig{}
	newSig.cloneFrom(&b.baseBuiltinCastFunc)
	return newSig
}

func (b *builtinCastJSONAsDecimalSig) evalDecimal(row chunk.Row) (res *types.MyDecimal, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalJSON(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ConvertJSONToDecimal(sc, val)
	if err != nil {
		return res, false, err
	}
	res, err = types.ProduceDecWithSpecifiedTp(res, b.tp, sc)
	return res, false, err
}

type builtinCastJSONAsStringSig struct {
	baseBuiltinFunc
}

func (b *builtinCastJSONAsStringSig) Clone() builtinFunc {
	newSig := &builtinCastJSONAsStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastJSONAsStringSig) evalString(row chunk.Row) (res string, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalJSON(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	return val.String(), false, nil
}

type builtinCastJSONAsTimeSig struct {
	baseBuiltinFunc
}

func (b *builtinCastJSONAsTimeSig) Clone() builtinFunc {
	newSig := &builtinCastJSONAsTimeSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastJSONAsTimeSig) evalTime(row chunk.Row) (res types.Time, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalJSON(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	s, err := val.Unquote()
	if err != nil {
		return res, false, err
	}
	sc := b.ctx.GetSessionVars().StmtCtx
	res, err = types.ParseTime(sc, s, b.tp.GetType(), b.tp.GetDecimal())
	if err != nil {
		return types.ZeroTime, true, handleInvalidTimeError(b.ctx, err)
	}
	if b.tp.GetType() == mysql.TypeDate {
		// Truncate hh:mm:ss part if the type is Date.
		res.SetCoreTime(types.FromDate(res.Year(), res.Month(), res.Day(), 0, 0, 0, 0))
	}
	return
}

type builtinCastJSONAsDurationSig struct {
	baseBuiltinFunc
}

func (b *builtinCastJSONAsDurationSig) Clone() builtinFunc {
	newSig := &builtinCastJSONAsDurationSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCastJSONAsDurationSig) evalDuration(row chunk.Row) (res types.Duration, isNull bool, err error) {
	val, isNull, err := b.args[0].EvalJSON(b.ctx, row)
	if isNull || err != nil {
		return res, isNull, err
	}
	s, err := val.Unquote()
	if err != nil {
		return res, false, err
	}
	res, err = types.ParseDuration(b.ctx.GetSessionVars().StmtCtx, s, b.tp.GetDecimal())
	if types.ErrTruncatedWrongVal.Equal(err) {
		sc := b.ctx.GetSessionVars().StmtCtx
		err = sc.HandleTruncate(err)
	}
	return
}

// inCastContext is session key type that indicates whether executing
// in special cast context that negative unsigned num will be zero.
type inCastContext int

func (i inCastContext) String() string {
	return "__cast_ctx"
}

// inUnionCastContext is session key value that indicates whether executing in
// union cast context.
// @see BuildCastFunction4Union
const inUnionCastContext inCastContext = 0

// CanImplicitEvalInt represents the builtin functions that have an implicit path to evaluate as integer,
// regardless of the type that type inference decides it to be.
// This is a nasty way to match the weird behavior of MySQL functions like `dayname()` being implicitly evaluated as integer.
// See https://github.com/mysql/mysql-server/blob/ee4455a33b10f1b1886044322e4893f587b319ed/sql/item_timefunc.h#L423 for details.
func CanImplicitEvalInt(expr Expression) bool {
	switch f := expr.(type) {
	case *ScalarFunction:
		switch f.FuncName.L {
		case ast.DayName:
			return true
		}
	}
	return false
}

// CanImplicitEvalReal represents the builtin functions that have an implicit path to evaluate as real,
// regardless of the type that type inference decides it to be.
// This is a nasty way to match the weird behavior of MySQL functions like `dayname()` being implicitly evaluated as real.
// See https://github.com/mysql/mysql-server/blob/ee4455a33b10f1b1886044322e4893f587b319ed/sql/item_timefunc.h#L423 for details.
func CanImplicitEvalReal(expr Expression) bool {
	switch f := expr.(type) {
	case *ScalarFunction:
		switch f.FuncName.L {
		case ast.DayName:
			return true
		}
	}
	return false
}

// BuildCastFunction4Union build a implicitly CAST ScalarFunction from the Union
// Expression.
func BuildCastFunction4Union(ctx sessionctx.Context, expr Expression, tp *types.FieldType) (res Expression) {
	ctx.SetValue(inUnionCastContext, struct{}{})
	defer func() {
		ctx.SetValue(inUnionCastContext, nil)
	}()
	return BuildCastFunction(ctx, expr, tp)
}

// BuildCastCollationFunction builds a ScalarFunction which casts the collation.
func BuildCastCollationFunction(ctx sessionctx.Context, expr Expression, ec *ExprCollation, enumOrSetRealTypeIsStr bool) Expression {
	if expr.GetType().EvalType() != types.ETString {
		return expr
	}
	if expr.GetType().GetCollate() == ec.Collation {
		return expr
	}
	tp := expr.GetType().Clone()
	if expr.GetType().Hybrid() {
		if enumOrSetRealTypeIsStr {
			tp = types.NewFieldType(mysql.TypeVarString)
		} else {
			return expr
		}
	} else if ec.Charset == charset.CharsetBin {
		// When cast character string to binary string, if we still use fixed length representation,
		// then 0 padding will be used, which can affect later execution.
		// e.g. https://github.com/pingcap/tidb/issues/34823.
		// On the other hand, we can not directly return origin expr back,
		// since we need binary collation to do string comparison later.
		// e.g. https://github.com/pingcap/tidb/pull/35053#discussion_r894155052
		// Here we use VarString type of cast, i.e `cast(a as binary)`, to avoid this problem.
		tp.SetType(mysql.TypeVarString)
	}
	tp.SetCharset(ec.Charset)
	tp.SetCollate(ec.Collation)
	newExpr := BuildCastFunction(ctx, expr, tp)
	return newExpr
}

// BuildCastFunction builds a CAST ScalarFunction from the Expression.
func BuildCastFunction(ctx sessionctx.Context, expr Expression, tp *types.FieldType) (res Expression) {
	argType := expr.GetType()
	// If source argument's nullable, then target type should be nullable
	if !mysql.HasNotNullFlag(argType.GetFlag()) {
		tp.DelFlag(mysql.NotNullFlag)
	}
	expr = TryPushCastIntoControlFunctionForHybridType(ctx, expr, tp)
	var fc functionClass
	switch tp.EvalType() {
	case types.ETInt:
		fc = &castAsIntFunctionClass{baseFunctionClass{ast.Cast, 1, 1}, tp}
	case types.ETDecimal:
		fc = &castAsDecimalFunctionClass{baseFunctionClass{ast.Cast, 1, 1}, tp}
	case types.ETReal:
		fc = &castAsRealFunctionClass{baseFunctionClass{ast.Cast, 1, 1}, tp}
	case types.ETDatetime, types.ETTimestamp:
		fc = &castAsTimeFunctionClass{baseFunctionClass{ast.Cast, 1, 1}, tp}
	case types.ETDuration:
		fc = &castAsDurationFunctionClass{baseFunctionClass{ast.Cast, 1, 1}, tp}
	case types.ETJson:
		fc = &castAsJSONFunctionClass{baseFunctionClass{ast.Cast, 1, 1}, tp}
	case types.ETString:
		fc = &castAsStringFunctionClass{baseFunctionClass{ast.Cast, 1, 1}, tp}
		if expr.GetType().GetType() == mysql.TypeBit {
			tp.SetFlen((expr.GetType().GetFlen() + 7) / 8)
		}
	}
	f, err := fc.getFunction(ctx, []Expression{expr})
	terror.Log(err)
	res = &ScalarFunction{
		FuncName: model.NewCIStr(ast.Cast),
		RetType:  tp,
		Function: f,
	}
	// We do not fold CAST if the eval type of this scalar function is ETJson
	// since we may reset the flag of the field type of CastAsJson later which
	// would affect the evaluation of it.
	if tp.EvalType() != types.ETJson {
		res = FoldConstant(res)
	}
	return res
}

// WrapWithCastAsInt wraps `expr` with `cast` if the return type of expr is not
// type int, otherwise, returns `expr` directly.
func WrapWithCastAsInt(ctx sessionctx.Context, expr Expression) Expression {
	if expr.GetType().GetType() == mysql.TypeEnum {
		if col, ok := expr.(*Column); ok {
			col = col.Clone().(*Column)
			col.RetType = col.RetType.Clone()
			expr = col
		}
		expr.GetType().AddFlag(mysql.EnumSetAsIntFlag)
	}
	if expr.GetType().EvalType() == types.ETInt {
		return expr
	}
	tp := types.NewFieldType(mysql.TypeLonglong)
	tp.SetFlen(expr.GetType().GetFlen())
	tp.SetDecimal(0)
	types.SetBinChsClnFlag(tp)
	tp.AddFlag(expr.GetType().GetFlag() & mysql.UnsignedFlag)
	return BuildCastFunction(ctx, expr, tp)
}

// WrapWithCastAsReal wraps `expr` with `cast` if the return type of expr is not
// type real, otherwise, returns `expr` directly.
func WrapWithCastAsReal(ctx sessionctx.Context, expr Expression) Expression {
	if expr.GetType().EvalType() == types.ETReal {
		return expr
	}
	tp := types.NewFieldType(mysql.TypeDouble)
	tp.SetFlen(mysql.MaxRealWidth)
	tp.SetDecimal(types.UnspecifiedLength)
	types.SetBinChsClnFlag(tp)
	tp.AddFlag(expr.GetType().GetFlag() & mysql.UnsignedFlag)
	return BuildCastFunction(ctx, expr, tp)
}

// WrapWithCastAsDecimal wraps `expr` with `cast` if the return type of expr is
// not type decimal, otherwise, returns `expr` directly.
func WrapWithCastAsDecimal(ctx sessionctx.Context, expr Expression) Expression {
	if expr.GetType().EvalType() == types.ETDecimal {
		return expr
	}
	tp := types.NewFieldType(mysql.TypeNewDecimal)
	tp.SetFlenUnderLimit(expr.GetType().GetFlen())
	tp.SetDecimalUnderLimit(expr.GetType().GetDecimal())

	if expr.GetType().EvalType() == types.ETInt {
		tp.SetFlen(mysql.MaxIntWidth)
	}
	if tp.GetFlen() == types.UnspecifiedLength || tp.GetFlen() > mysql.MaxDecimalWidth {
		tp.SetFlen(mysql.MaxDecimalWidth)
	}
	types.SetBinChsClnFlag(tp)
	tp.AddFlag(expr.GetType().GetFlag() & mysql.UnsignedFlag)
	castExpr := BuildCastFunction(ctx, expr, tp)
	// For const item, we can use find-grained precision and scale by the result.
	if castExpr.ConstItem(ctx.GetSessionVars().StmtCtx) {
		val, isnull, err := castExpr.EvalDecimal(ctx, chunk.Row{})
		if !isnull && err == nil {
			precision, frac := val.PrecisionAndFrac()
			castTp := castExpr.GetType()
			castTp.SetDecimalUnderLimit(frac)
			castTp.SetFlenUnderLimit(precision)
		}
	}
	return castExpr
}

// WrapWithCastAsString wraps `expr` with `cast` if the return type of expr is
// not type string, otherwise, returns `expr` directly.
func WrapWithCastAsString(ctx sessionctx.Context, expr Expression) Expression {
	exprTp := expr.GetType()
	if exprTp.EvalType() == types.ETString {
		return expr
	}
	argLen := exprTp.GetFlen()
	// If expr is decimal, we should take the decimal point ,negative sign and the leading zero(0.xxx)
	// into consideration, so we set `expr.GetType().GetFlen() + 3` as the `argLen`.
	// Since the length of float and double is not accurate, we do not handle
	// them.
	if exprTp.GetType() == mysql.TypeNewDecimal && argLen != types.UnspecifiedFsp {
		argLen += 3
	}

	if exprTp.EvalType() == types.ETInt {
		argLen = mysql.MaxIntWidth
		// For TypeBit, castAsString will make length as int(( bit_len + 7 ) / 8) bytes due to
		// TiKV needs the bit's real len during calculating, eg: ascii(bit).
		if exprTp.GetType() == mysql.TypeBit {
			argLen = (exprTp.GetFlen() + 7) / 8
		}
	}

	// Because we can't control the length of cast(float as char) for now, we can't determine the argLen.
	if exprTp.GetType() == mysql.TypeFloat || exprTp.GetType() == mysql.TypeDouble {
		argLen = -1
	}
	tp := types.NewFieldType(mysql.TypeVarString)
	if expr.Coercibility() == CoercibilityExplicit {
		charset, collate := expr.CharsetAndCollation()
		tp.SetCharset(charset)
		tp.SetCollate(collate)
	} else {
		charset, collate := ctx.GetSessionVars().GetCharsetInfo()
		tp.SetCharset(charset)
		tp.SetCollate(collate)
	}
	tp.SetFlen(argLen)
	tp.SetDecimal(types.UnspecifiedLength)
	return BuildCastFunction(ctx, expr, tp)
}

// WrapWithCastAsTime wraps `expr` with `cast` if the return type of expr is not
// same as type of the specified `tp` , otherwise, returns `expr` directly.
func WrapWithCastAsTime(ctx sessionctx.Context, expr Expression, tp *types.FieldType) Expression {
	exprTp := expr.GetType().GetType()
	if tp.GetType() == exprTp {
		return expr
	} else if (exprTp == mysql.TypeDate || exprTp == mysql.TypeTimestamp) && tp.GetType() == mysql.TypeDatetime {
		return expr
	}
	switch x := expr.GetType().EvalType(); x {
	case types.ETInt:
		tp.SetDecimal(types.MinFsp)
	case types.ETString, types.ETReal, types.ETJson:
		tp.SetDecimal(types.MaxFsp)
	case types.ETDatetime, types.ETTimestamp, types.ETDuration:
		tp.SetDecimal(expr.GetType().GetDecimal())
	case types.ETDecimal:
		tp.SetDecimal(expr.GetType().GetDecimal())
		if tp.GetDecimal() > types.MaxFsp {
			tp.SetDecimal(types.MaxFsp)
		}
	default:
	}
	switch tp.GetType() {
	case mysql.TypeDate:
		tp.SetFlen(mysql.MaxDateWidth)
	case mysql.TypeDatetime, mysql.TypeTimestamp:
		tp.SetFlen(mysql.MaxDatetimeWidthNoFsp)
		if tp.GetDecimal() > 0 {
			tp.SetFlen(tp.GetFlen() + 1 + tp.GetDecimal())
		}
	}
	types.SetBinChsClnFlag(tp)
	return BuildCastFunction(ctx, expr, tp)
}

// WrapWithCastAsDuration wraps `expr` with `cast` if the return type of expr is
// not type duration, otherwise, returns `expr` directly.
func WrapWithCastAsDuration(ctx sessionctx.Context, expr Expression) Expression {
	if expr.GetType().GetType() == mysql.TypeDuration {
		return expr
	}
	tp := types.NewFieldType(mysql.TypeDuration)
	switch x := expr.GetType(); x.GetType() {
	case mysql.TypeDatetime, mysql.TypeTimestamp, mysql.TypeDate:
		tp.SetDecimal(x.GetDecimal())
	default:
		tp.SetDecimal(types.MaxFsp)
	}
	tp.SetFlen(mysql.MaxDurationWidthNoFsp)
	if tp.GetDecimal() > 0 {
		tp.SetFlen(tp.GetFlen() + 1 + tp.GetDecimal())
	}
	return BuildCastFunction(ctx, expr, tp)
}

// WrapWithCastAsJSON wraps `expr` with `cast` if the return type of expr is not
// type json, otherwise, returns `expr` directly.
func WrapWithCastAsJSON(ctx sessionctx.Context, expr Expression) Expression {
	if expr.GetType().GetType() == mysql.TypeJSON && !mysql.HasParseToJSONFlag(expr.GetType().GetFlag()) {
		return expr
	}
	tp := types.NewFieldTypeBuilder().SetType(mysql.TypeJSON).SetFlag(mysql.BinaryFlag).SetFlen(12582912).SetCharset(mysql.DefaultCharset).SetCollate(mysql.DefaultCollationName).BuildP()
	return BuildCastFunction(ctx, expr, tp)
}

// TryPushCastIntoControlFunctionForHybridType try to push cast into control function for Hybrid Type.
// If necessary, it will rebuild control function using changed args.
// When a hybrid type is the output of a control function, the result may be as a numeric type to subsequent calculation
// We should perform the `Cast` operation early to avoid using the wrong type for calculation
// For example, the condition `if(1, e, 'a') = 1`, `if` function will output `e` and compare with `1`.
// If the evaltype is ETString, it will get wrong result. So we can rewrite the condition to
// `IfInt(1, cast(e as int), cast('a' as int)) = 1` to get the correct result.
func TryPushCastIntoControlFunctionForHybridType(ctx sessionctx.Context, expr Expression, tp *types.FieldType) (res Expression) {
	sf, ok := expr.(*ScalarFunction)
	if !ok {
		return expr
	}

	var wrapCastFunc func(ctx sessionctx.Context, expr Expression) Expression
	switch tp.EvalType() {
	case types.ETInt:
		wrapCastFunc = WrapWithCastAsInt
	case types.ETReal:
		wrapCastFunc = WrapWithCastAsReal
	default:
		return expr
	}

	isHybrid := func(ft *types.FieldType) bool {
		// todo: compatible with mysql control function using bit type. issue 24725
		return ft.Hybrid() && ft.GetType() != mysql.TypeBit
	}

	args := sf.GetArgs()
	switch sf.FuncName.L {
	case ast.If:
		if isHybrid(args[1].GetType()) || isHybrid(args[2].GetType()) {
			args[1] = wrapCastFunc(ctx, args[1])
			args[2] = wrapCastFunc(ctx, args[2])
			f, err := funcs[ast.If].getFunction(ctx, args)
			if err != nil {
				return expr
			}
			sf.RetType, sf.Function = f.getRetTp(), f
			return sf
		}
	case ast.Case:
		hasHybrid := false
		for i := 0; i < len(args)-1; i += 2 {
			hasHybrid = hasHybrid || isHybrid(args[i+1].GetType())
		}
		if len(args)%2 == 1 {
			hasHybrid = hasHybrid || isHybrid(args[len(args)-1].GetType())
		}
		if !hasHybrid {
			return expr
		}

		for i := 0; i < len(args)-1; i += 2 {
			args[i+1] = wrapCastFunc(ctx, args[i+1])
		}
		if len(args)%2 == 1 {
			args[len(args)-1] = wrapCastFunc(ctx, args[len(args)-1])
		}
		f, err := funcs[ast.Case].getFunction(ctx, args)
		if err != nil {
			return expr
		}
		sf.RetType, sf.Function = f.getRetTp(), f
		return sf
	case ast.Elt:
		hasHybrid := false
		for i := 1; i < len(args); i++ {
			hasHybrid = hasHybrid || isHybrid(args[i].GetType())
		}
		if !hasHybrid {
			return expr
		}

		for i := 1; i < len(args); i++ {
			args[i] = wrapCastFunc(ctx, args[i])
		}
		f, err := funcs[ast.Elt].getFunction(ctx, args)
		if err != nil {
			return expr
		}
		sf.RetType, sf.Function = f.getRetTp(), f
		return sf
	default:
		return expr
	}
	return expr
}
