package interpreter

import (
	"fmt"
	"reflect"
	"strconv"
)

var (
	interpreterType = reflect.TypeOf((*Interpreter)(nil))
	errorType       = reflect.TypeOf((*error)(nil)).Elem()
)

// WrapFunc wraps a typed Go function into a Function (func(args []any) (any, error)).
func WrapFunc(fn any) (Function, error) {
	return WrapFuncWithInterpreter(nil, fn)
}

// MustWrapFunc wraps a typed Go function into a Function, panicking on error.
func MustWrapFunc(fn any) Function {
	return MustWrapFuncWithInterpreter(nil, fn)
}

// MustWrapFuncWithInterpreter wraps a typed Go function into a Function with an interpreter context, panicking on error.
func MustWrapFuncWithInterpreter(interp *Interpreter, fn any) Function {
	f, err := WrapFuncWithInterpreter(interp, fn)
	if err != nil {
		panic("interpreter: " + err.Error())
	}
	return f
}

// WrapFuncWithInterpreter wraps a typed Go function into a Function with optional Interpreter binding.
func WrapFuncWithInterpreter(interp *Interpreter, fn any) (Function, error) {
	if fn == nil {
		return nil, fmt.Errorf("cannot wrap nil function")
	}
	if f, ok := fn.(Function); ok {
		return f, nil
	}
	if f, ok := fn.(func([]any) (any, error)); ok {
		return Function(f), nil
	}

	fnVal := reflect.ValueOf(fn)
	if fnVal.Kind() != reflect.Func {
		return nil, fmt.Errorf("expected a function, got %T", fn)
	}

	fnType := fnVal.Type()
	numIn := fnType.NumIn()
	isVariadic := fnType.IsVariadic()

	hasInterpParam := numIn > 0 && fnType.In(0) == interpreterType

	return func(args []any) (any, error) {
		argIdx := 0
		var callArgs []reflect.Value

		if hasInterpParam {
			if interp != nil {
				callArgs = append(callArgs, reflect.ValueOf(interp))
			} else if len(args) > 0 {
				if interpArg, ok := args[0].(*Interpreter); ok {
					callArgs = append(callArgs, reflect.ValueOf(interpArg))
					argIdx++
				} else {
					return nil, fmt.Errorf("function expects *Interpreter as first argument, got %T", args[0])
				}
			} else {
				return nil, fmt.Errorf("function expects *Interpreter as first argument, but no arguments provided")
			}
		}

		remainingArgs := args[argIdx:]
		numNonInterpIn := numIn
		if hasInterpParam {
			numNonInterpIn--
		}

		minArgs := numNonInterpIn
		if isVariadic {
			minArgs--
		}

		if len(remainingArgs) < minArgs {
			return nil, fmt.Errorf("function expects at least %d arguments, got %d", minArgs, len(remainingArgs))
		}
		if !isVariadic && len(remainingArgs) > numNonInterpIn {
			return nil, fmt.Errorf("function expects %d arguments, got %d", numNonInterpIn, len(remainingArgs))
		}

		normalParamCount := numNonInterpIn
		if isVariadic {
			normalParamCount--
		}

		paramStartIdx := 0
		if hasInterpParam {
			paramStartIdx = 1
		}

		for p := 0; p < normalParamCount; p++ {
			paramType := fnType.In(paramStartIdx + p)
			val, err := convertValue(remainingArgs[p], paramType)
			if err != nil {
				return nil, fmt.Errorf("argument %d: %w", p, err)
			}
			callArgs = append(callArgs, val)
		}

		if isVariadic {
			varSliceType := fnType.In(numIn - 1)
			elemType := varSliceType.Elem()
			varArgs := remainingArgs[normalParamCount:]
			for p, varArg := range varArgs {
				val, err := convertValue(varArg, elemType)
				if err != nil {
					return nil, fmt.Errorf("variadic argument %d: %w", p, err)
				}
				callArgs = append(callArgs, val)
			}
		}

		out := fnVal.Call(callArgs)
		return processReturns(out)
	}, nil
}

func processReturns(out []reflect.Value) (any, error) {
	n := len(out)
	if n == 0 {
		return nil, nil
	}

	lastVal := out[n-1]
	var err error
	hasErrorRet := lastVal.Type().Implements(errorType)
	if hasErrorRet {
		switch lastVal.Kind() {
		case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
			if !lastVal.IsNil() {
				err = lastVal.Interface().(error)
			}
		default:
			err = lastVal.Interface().(error)
		}
	}

	if n == 1 {
		if hasErrorRet {
			return nil, err
		}
		return normalizeReturnValue(out[0].Interface()), nil
	}

	if n == 2 && hasErrorRet {
		if err != nil {
			return nil, err
		}
		return normalizeReturnValue(out[0].Interface()), nil
	}

	limit := n
	if hasErrorRet {
		if err != nil {
			return nil, err
		}
		limit = n - 1
	}

	res := make([]any, limit)
	for i := 0; i < limit; i++ {
		res[i] = normalizeReturnValue(out[i].Interface())
	}
	return res, nil
}

func normalizeReturnValue(v any) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int16:
		return int64(x)
	case int8:
		return int64(x)
	case uint:
		return int64(x)
	case uint64:
		return int64(x)
	case uint32:
		return int64(x)
	case uint16:
		return int64(x)
	case uint8:
		return int64(x)
	case float32:
		return float64(x)
	case bool:
		return boolInt(x)
	default:
		return v
	}
}

func convertValue(v any, targetType reflect.Type) (reflect.Value, error) {
	if addr, ok := v.(address); ok {
		if targetType != reflect.TypeOf(address{}) {
			v = addr.get()
		}
	}

	if v == nil {
		return reflect.Zero(targetType), nil
	}

	vVal := reflect.ValueOf(v)

	if vVal.Type().AssignableTo(targetType) {
		return vVal, nil
	}

	if targetType.Kind() == reflect.Interface {
		return vVal, nil
	}

	targetKind := targetType.Kind()

	switch targetKind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := integer(v)
		if err != nil {
			if s, ok := v.(string); ok {
				if parsed, parseErr := strconv.ParseInt(s, 0, 64); parseErr == nil {
					return reflect.ValueOf(parsed).Convert(targetType), nil
				}
			}
			return reflect.Value{}, fmt.Errorf("cannot convert %T (%v) to %s", v, v, targetType)
		}
		return reflect.ValueOf(n).Convert(targetType), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := integer(v)
		if err != nil {
			if s, ok := v.(string); ok {
				if parsed, parseErr := strconv.ParseUint(s, 0, 64); parseErr == nil {
					return reflect.ValueOf(parsed).Convert(targetType), nil
				}
			}
			return reflect.Value{}, fmt.Errorf("cannot convert %T (%v) to %s", v, v, targetType)
		}
		if n < 0 {
			return reflect.Value{}, fmt.Errorf("cannot convert negative value %d to %s", n, targetType)
		}
		return reflect.ValueOf(uint64(n)).Convert(targetType), nil

	case reflect.Float32, reflect.Float64:
		f, ok := asFloat(v)
		if !ok {
			if s, ok := v.(string); ok {
				if parsed, parseErr := strconv.ParseFloat(s, 64); parseErr == nil {
					return reflect.ValueOf(parsed).Convert(targetType), nil
				}
			}
			return reflect.Value{}, fmt.Errorf("cannot convert %T (%v) to %s", v, v, targetType)
		}
		return reflect.ValueOf(f).Convert(targetType), nil

	case reflect.Bool:
		return reflect.ValueOf(truth(v)), nil

	case reflect.String:
		if s, ok := v.(string); ok {
			return reflect.ValueOf(s), nil
		}
		return reflect.ValueOf(fmt.Sprint(v)), nil

	case reflect.Slice:
		if vVal.Kind() == reflect.Slice || vVal.Kind() == reflect.Array {
			l := vVal.Len()
			res := reflect.MakeSlice(targetType, l, l)
			elemType := targetType.Elem()
			for i := 0; i < l; i++ {
				elem, err := convertValue(vVal.Index(i).Interface(), elemType)
				if err != nil {
					return reflect.Value{}, fmt.Errorf("index %d: %w", i, err)
				}
				res.Index(i).Set(elem)
			}
			return res, nil
		}
		return reflect.Value{}, fmt.Errorf("cannot convert %T to %s", v, targetType)

	case reflect.Map:
		if vVal.Kind() == reflect.Map {
			res := reflect.MakeMap(targetType)
			keyType := targetType.Key()
			elemType := targetType.Elem()
			for _, k := range vVal.MapKeys() {
				convKey, err := convertValue(k.Interface(), keyType)
				if err != nil {
					return reflect.Value{}, fmt.Errorf("map key %v: %w", k, err)
				}
				convVal, err := convertValue(vVal.MapIndex(k).Interface(), elemType)
				if err != nil {
					return reflect.Value{}, fmt.Errorf("map key %v value: %w", k, err)
				}
				res.SetMapIndex(convKey, convVal)
			}
			return res, nil
		}
		return reflect.Value{}, fmt.Errorf("cannot convert %T to %s", v, targetType)

	case reflect.Ptr:
		if vVal.Kind() == reflect.Ptr {
			if vVal.Type().AssignableTo(targetType) {
				return vVal, nil
			}
		}
		elemType := targetType.Elem()
		elemVal, err := convertValue(v, elemType)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("pointer element: %w", err)
		}
		ptr := reflect.New(elemType)
		ptr.Elem().Set(elemVal)
		return ptr, nil
	}

	if vVal.Type().ConvertibleTo(targetType) {
		return vVal.Convert(targetType), nil
	}

	return reflect.Value{}, fmt.Errorf("cannot convert %T (%v) to %s", v, v, targetType)
}
