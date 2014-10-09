package main

import (
    "bufio"
    //"io"
    "fmt"
    "os"
    "strings"
    "errors"
)

import (
    . "types"
    "reader"
    "printer"
    . "env"
    "core"
)

// read
func READ(str string) (MalType, error) {
    return reader.Read_str(str)
}

// eval
func is_pair(x MalType) bool {
    slc, e := GetSlice(x)
    if e != nil { return false }
    return len(slc) > 0
}

func quasiquote(ast MalType) MalType {
    if !is_pair(ast) {
        return List{[]MalType{Symbol{"quote"}, ast}}
    } else {
        slc, _ := GetSlice(ast)
        a0 := slc[0]
        if Symbol_Q(a0) && (a0.(Symbol).Val == "unquote") {
            return slc[1]
        } else if is_pair(a0) {
            slc0, _ := GetSlice(a0)
            a00 := slc0[0]
            if Symbol_Q(a00) && (a00.(Symbol).Val == "splice-unquote") {
                return List{[]MalType{Symbol{"concat"},
                                      slc0[1],
                                      quasiquote(List{slc[1:]})}}
            }
        }
        return List{[]MalType{Symbol{"cons"},
                              quasiquote(a0),
                              quasiquote(List{slc[1:]})}}
    }
}

func is_macro_call(ast MalType, env EnvType) bool {
    if List_Q(ast) {
        slc, _ := GetSlice(ast)
        a0 := slc[0]
        if Symbol_Q(a0) && env.Find(a0.(Symbol).Val) != nil {
            mac, e := env.Get(a0.(Symbol).Val)
            if e != nil { return false }
            if MalFunc_Q(mac) {
                return mac.(MalFunc).GetMacro()
            }
        }
    }
    return false
}

func macroexpand(ast MalType, env EnvType)  (MalType, error) {
    var mac MalType
    var e error
    for ; is_macro_call(ast, env) ; {
        slc, _ := GetSlice(ast)
        a0 := slc[0]
        mac, e = env.Get(a0.(Symbol).Val); if e != nil { return nil, e }
        fn := mac.(MalFunc)
        ast, e = Apply(fn, slc[1:]); if e != nil { return nil, e }
    }
    return ast, nil
}

func eval_ast(ast MalType, env EnvType) (MalType, error) {
    //fmt.Printf("eval_ast: %#v\n", ast)
    if Symbol_Q(ast) {
        return env.Get(ast.(Symbol).Val)
    } else if List_Q(ast) {
        lst := []MalType{}
        for _, a := range ast.(List).Val {
            exp, e := EVAL(a, env)
            if e != nil { return nil, e }
            lst = append(lst, exp)
        }
        return List{lst}, nil
    } else if Vector_Q(ast) {
        lst := []MalType{}
        for _, a := range ast.(Vector).Val {
            exp, e := EVAL(a, env)
            if e != nil { return nil, e }
            lst = append(lst, exp)
        }
        return Vector{lst}, nil
    } else if Hash_Map_Q(ast) {
        m := ast.(map[string]MalType)
        new_hm := map[string]MalType{}
        for k, v := range m {
            ke, e1 := EVAL(k, env)
            if e1 != nil { return nil, e1 }
            if _, ok := ke.(string); !ok {
                return nil, errors.New("non string hash-map key")
            }
            kv, e2 := EVAL(v, env)
            if e2 != nil { return nil, e2 }
            new_hm[ke.(string)] = kv
        }
        return new_hm, nil
    } else {
        return ast, nil
    }
}

func EVAL(ast MalType, env EnvType) (MalType, error) {
    var e error
    for {

    //fmt.Printf("EVAL: %v\n", printer.Pr_str(ast, true))
    switch ast.(type) {
    case List: // continue
    default:   return eval_ast(ast, env)
    }

    // apply list
    ast, e = macroexpand(ast, env); if e != nil { return nil, e }
    if (!List_Q(ast)) { return ast, nil }

    a0 := ast.(List).Val[0]
    var a1 MalType = nil; var a2 MalType = nil
    switch len(ast.(List).Val) {
    case 1:
        a1 = nil; a2 = nil
    case 2:
        a1 = ast.(List).Val[1]; a2 = nil
    default:
        a1 = ast.(List).Val[1]; a2 = ast.(List).Val[2]
    }
    a0sym := "__<*fn*>__"
    if Symbol_Q(a0) { a0sym = a0.(Symbol).Val } 
    switch a0sym {
    case "def!":
        res, e := EVAL(a2, env)
        if e != nil { return nil, e }
        return env.Set(a1.(Symbol).Val, res), nil
    case "let*":
        let_env, e := NewEnv(env, nil, nil)
        if e != nil { return nil, e }
        arr1, e := GetSlice(a1)
        if e != nil { return nil, e }
        for i := 0; i < len(arr1); i+=2 {
            if !Symbol_Q(arr1[i]) {
                return nil, errors.New("non-symbol bind value")
            }
            exp, e := EVAL(arr1[i+1], let_env)
            if e != nil { return nil, e }
            let_env.Set(arr1[i].(Symbol).Val, exp)
        }
        ast = a2
        env = let_env
    case "quote":
        return a1, nil
    case "quasiquote":
        ast = quasiquote(a1)
    case "defmacro!":
        fn, e := EVAL(a2, env)
        fn = fn.(MalFunc).SetMacro()
        if e != nil { return nil, e }
        return env.Set(a1.(Symbol).Val, fn), nil
    case "macroexpand":
        return macroexpand(a1, env)
    case "do":
        lst := ast.(List).Val
        _, e := eval_ast(List{lst[1:len(lst)-1]}, env) 
        if e != nil { return nil, e }
        if len(lst) == 1 { return nil, nil }
        ast = lst[len(lst)-1]
    case "if":
        cond, e := EVAL(a1, env)
        if e != nil { return nil, e }
        if cond == nil || cond == false {
            if len(ast.(List).Val) >= 4 {
                ast = ast.(List).Val[3]
            } else {
                return nil, nil
            }
        } else {
            ast = a2
        }
    case "fn*":
        fn := MalFunc{EVAL, a2, env, a1, false, NewEnv}
        return fn, nil
    default:
        el, e := eval_ast(ast, env)
        if e != nil { return nil, e }
        f := el.(List).Val[0]
        if MalFunc_Q(f) {
            fn := f.(MalFunc)
            ast = fn.Exp
            env, e = NewEnv(fn.Env, fn.Params, List{el.(List).Val[1:]})
            if e != nil { return nil, e }
        } else {
            fn, ok := f.(func([]MalType)(MalType, error))
            if !ok { return nil, errors.New("attempt to call non-function") }
            return fn(el.(List).Val[1:])
        }
    }

    } // TCO loop
}

// print
func PRINT(exp MalType) (string, error) {
    return printer.Pr_str(exp, true), nil
}


var repl_env, _ = NewEnv(nil, nil, nil)

// repl
func rep(str string) (MalType, error) {
    var exp MalType
    var res string
    var e error
    if exp, e = READ(str); e != nil { return nil, e }
    if exp, e = EVAL(exp, repl_env); e != nil { return nil, e }
    if res, e = PRINT(exp); e != nil { return nil, e }
    return res, nil
}

func main() {
    // core.go: defined using go
    for k, v := range core.NS {
        repl_env.Set(k, v)
    }
    repl_env.Set("eval", func(a []MalType) (MalType, error) {
        return EVAL(a[0], repl_env) })
    repl_env.Set("*ARGV*", List{})

    // core.mal: defined using the language itself
    rep("(def! not (fn* (a) (if a false true)))")
    rep("(def! load-file (fn* (f) (eval (read-string (str \"(do \" (slurp f) \")\")))))")

    // called with mal script to load and eval
    if len(os.Args) > 1 {
        args := make([]MalType, 0, len(os.Args)-2)
        for _,a := range os.Args[2:] {
            args = append(args, a)
        }
        repl_env.Set("*ARGV*", List{args})
        rep("(load-file \"" + os.Args[1] + "\")")
        os.Exit(0)
    }

    rdr := bufio.NewReader(os.Stdin);
    // repl loop
    for {
        fmt.Print("user> ");
        text, err := rdr.ReadString('\n');
        text = strings.TrimRight(text, "\n");
        if (err != nil) {
            return
        }
        var out MalType
        var e error
        if out, e = rep(text); e != nil {
            if e.Error() == "<empty line>" { continue }
            fmt.Printf("Error: %v\n", e)
            continue
        }
        fmt.Printf("%v\n", out)
    }
}