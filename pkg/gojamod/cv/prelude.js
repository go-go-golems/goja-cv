// cv prelude: the fluent JavaScript surface over the native binding.
//
// A node is a Proxy over { __id, __kind }. Method tables come from the Go
// registry (native.__registry()), so there is one source of truth. Rules:
//   - `then` makes nodes awaitable (Value → data, Image → summary); helpers in
//     this file never touch `then` on a node: they check isNode() and read __id.
//   - Symbol.toPrimitive / valueOf throw E_LAZY_COMPARE (string hint: a label).
//   - Assigning, defining or deleting properties throws E_IMMUTABLE.
//   - Unknown properties on Value nodes are derived fields (stats().meanL);
//     on other kinds they throw E_UNKNOWN_METHOD with suggestions from Go.
// Evaluates to function (native) returning the module exports.
(function (native) {
  "use strict";

  const METHODS = native.__registry(); // { Image: ["contrast", "exposure", "mask.luminance", ...], ... }
  const tables = Object.create(null);
  for (const kind of Object.keys(METHODS)) {
    const t = Object.create(null);
    for (const name of METHODS[kind]) {
      const dot = name.indexOf(".");
      if (dot < 0) {
        t[name] = { op: name };
        continue;
      }
      const ns = name.slice(0, dot);
      if (!t[ns] || !t[ns].ns) t[ns] = { ns: Object.create(null) };
      t[ns].ns[name.slice(dot + 1)] = name;
    }
    tables[kind] = t;
  }

  // properties that generic JS machinery probes; they must not derive fields
  // or throw (console formatting, JSON, frameworks).
  const PROBES = new Set(["constructor", "__proto__", "prototype", "inspect", "nodeType", "$$typeof", "asymmetricMatch"]);

  const nodeSet = new WeakSet();
  const proxies = new Map(); // id → proxy: hash-consed nodes are === in JS too

  function isNode(x) {
    return typeof x === "object" && x !== null && nodeSet.has(x);
  }

  // data lambdas: one native id per function object, so the same function
  // passed twice gives the same (hash-consed) node.
  const funcIds = new WeakMap();
  function funcId(fn) {
    let id = funcIds.get(fn);
    if (id === undefined) {
      id = native.__func(fn);
      funcIds.set(fn, id);
    }
    return id;
  }

  // marshal converts call arguments for the native side: nodes become
  // { __cvnode: id }, functions { __cvfunc: id }; arrays and plain objects
  // are walked (nodes inside options objects are lifted by Go).
  function marshal(x) {
    if (x === null || x === undefined) return x;
    if (isNode(x)) return { __cvnode: x.__id };
    if (typeof x === "function") return { __cvfunc: funcId(x) };
    if (Array.isArray(x)) return x.map(marshal);
    if (typeof x === "object") {
      const o = {};
      for (const k of Object.keys(x)) o[k] = marshal(x[k]);
      return o;
    }
    return x;
  }

  function label(t) {
    const d = native.__describe(t.__id);
    return "[" + d.kind + " #" + d.id + " " + d.call + "]";
  }

  function callOp(t, op, args) {
    return nodeFor(native.__call(op, t.__id, args.map(marshal)));
  }

  function method(t, op) {
    return function () {
      return callOp(t, op, Array.prototype.slice.call(arguments));
    };
  }

  function namespaceFor(t, nsName, members) {
    return new Proxy(Object.create(null), {
      get(_, prop) {
        if (typeof prop === "symbol" || prop === "then" || PROBES.has(prop)) return undefined;
        const op = members[prop];
        if (op) return method(t, op);
        return native.__unknown(t.__kind, nsName + "." + prop, t.__id);
      },
      set(_, prop) {
        return native.__immutable(t.__id, nsName + "." + String(prop));
      },
    });
  }

  function paramProp(t, prop) {
    switch (prop) {
      case "value":
        return native.__paramInfo(t.__id).value;
      case "touched":
        return native.__paramInfo(t.__id).touched;
      case "name":
        return native.__paramInfo(t.__id).name;
      case "def":
        return native.__paramInfo(t.__id);
    }
    return undefined;
  }

  function lookup(t, prop) {
    if (typeof prop === "symbol") {
      if (prop === Symbol.toPrimitive) {
        return function (hint) {
          return hint === "string" ? label(t) : native.__lazy(t.__id, hint === "default" ? "primitive" : hint);
        };
      }
      if (prop === Symbol.toStringTag) return "cv." + t.__kind;
      return undefined;
    }
    switch (prop) {
      case "then":
        return function (onFulfilled, onRejected) {
          return native.__evaluate(t.__id, null).then(onFulfilled, onRejected);
        };
      case "__id":
        return t.__id;
      case "__kind":
        return t.__kind;
      case "toString":
        return function () {
          return label(t);
        };
      case "valueOf":
        return function () {
          return native.__lazy(t.__id, "number");
        };
      case "toJSON":
        return function () {
          return { node: label(t) };
        };
    }
    if (PROBES.has(prop)) return undefined;
    if (t.__kind === "Param") {
      const v = paramProp(t, prop);
      if (v !== undefined) return v;
    }
    const tab = tables[t.__kind];
    const entry = tab && tab[prop];
    if (entry) return entry.ns ? namespaceFor(t, prop, entry.ns) : method(t, entry.op);
    if (t.__kind === "Value") return callOp(t, "field", [prop]);
    return native.__unknown(t.__kind, prop, t.__id);
  }

  const handler = {
    get(t, prop) {
      return lookup(t, prop);
    },
    set(t, prop) {
      return native.__immutable(t.__id, String(prop));
    },
    defineProperty(t, prop) {
      return native.__immutable(t.__id, String(prop));
    },
    deleteProperty(t, prop) {
      return native.__immutable(t.__id, String(prop));
    },
    has(t, prop) {
      if (typeof prop !== "string") return false;
      const tab = tables[t.__kind];
      return prop === "then" || prop === "__id" || prop === "__kind" || !!(tab && tab[prop]);
    },
    ownKeys() {
      return [];
    },
    getOwnPropertyDescriptor() {
      return undefined;
    },
  };

  function nodeFor(h) {
    let p = proxies.get(h.id);
    if (p) return p;
    p = new Proxy({ __id: h.id, __kind: h.kind }, handler);
    nodeSet.add(p);
    proxies.set(h.id, p);
    return p;
  }

  function requireNode(x, where) {
    if (!isNode(x)) {
      const e = new Error(where + " needs a cv node.");
      e.name = e.code = "E_ARG_TYPE";
      throw e;
    }
    return x;
  }

  const P = {
    slider(name, opts) {
      return nodeFor(native.__param("slider", name, marshal(opts || {})));
    },
    toggle(name, opts) {
      return nodeFor(native.__param("toggle", name, marshal(opts || {})));
    },
    choice(name, opts) {
      return nodeFor(native.__param("choice", name, marshal(opts || {})));
    },
    // set moves a param as the person would (tests, hosts).
    set(name, value) {
      native.__paramSet(isNode(name) ? name.name : name, value);
    },
    list() {
      return native.__params();
    },
  };

  const cv = {
    version: native.__version,
    kinds: ["Image", "Mask", "Regions", "Value", "Param"],
    P: P,
    open(path) {
      return nodeFor(native.__open(path));
    },
    help(name) {
      return native.__help(name === undefined ? null : name);
    },
    // evaluate(node, { scale }) evaluates at an explicit scale: a number in
    // (0, 1], "preview" (default for await), "look" or "full" (1.0).
    evaluate(node, opts) {
      requireNode(node, "cv.evaluate");
      const scale = opts && opts.scale !== undefined ? opts.scale : null;
      return native.__evaluate(node.__id, scale);
    },
    describe(node) {
      return native.__describe(requireNode(node, "cv.describe").__id);
    },
    isNode: isNode,
    // warnings({ clear }) lists RANGE warnings collected while building nodes.
    warnings(opts) {
      return native.__warnings(!!(opts && opts.clear));
    },
  };
  return cv;
})
