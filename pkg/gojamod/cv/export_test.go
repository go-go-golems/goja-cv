package cvmod

// SetPanicHook makes every wrapped native function call f first (tests of
// the panic boundary); nil removes it.
func SetPanicHook(f func(name string)) { panicHook = f }
