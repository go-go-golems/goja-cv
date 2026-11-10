package main

import "github.com/pkg/errors"

// onceString is a string flag that may be given only once. --root names the
// one read-write workspace; a second --root used to replace the first
// silently, which turned a read-only data directory into the workspace.
type onceString struct {
	p    *string
	name string
	set  bool
}

func (o *onceString) String() string {
	if o.p == nil {
		return ""
	}
	return *o.p
}

func (o *onceString) Set(v string) error {
	if o.set {
		return errors.Errorf("--%s takes one directory (the workspace, read-write); add read-only directories with --read DIR", o.name)
	}
	*o.p, o.set = v, true
	return nil
}

func (o *onceString) Type() string { return "string" }
