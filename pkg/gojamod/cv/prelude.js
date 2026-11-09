// cv prelude: the fluent JavaScript surface over the native binding.
//
// A node is a Proxy over { __id, __kind }. Method tables come from the Go
// registry (native.__registry()), so there is one source of truth. Rules:
//   - `then` makes nodes awaitable (Value → data, Image → summary); helpers in
//     this file never touch `then` on a node: they check isNode() and read __id.
//   - Symbol.toPrimitive / valueOf throw E_NOT_AWAITED (string hint: a label).
//   - Assigning, defining or deleting properties throws E_IMMUTABLE.
//   - On a Value or Regions node, a property that is not a method throws
//     E_NOT_AWAITED (stats().p99: await it first); Regions nodes have no
//     methods (regions are arguments of image methods); on an Image node an
//     unknown property throws E_UNKNOWN_METHOD with suggestions from Go.
//   - Options are literal values: a node passed as an option is E_ARG_TYPE.
// Evaluates to function (native) returning the module exports.
(function (native) {
  "use strict";

  const METHODS = native.__registry(); // { Image: ["contrast", "exposure", "mask.luminance", ...], ..., cv: ["compare", ...] }
  const MODULE_OPS = METHODS.cv; // module ops: cv.<name>(…) builds a node with no receiver
  delete METHODS.cv;
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

  // properties that generic JS machinery probes; they must not throw
  // (console formatting, JSON, frameworks).
  const PROBES = new Set(["constructor", "__proto__", "prototype", "inspect", "nodeType", "$$typeof", "asymmetricMatch"]);

  const nodeSet = new WeakSet();
  const proxies = new Map(); // id → proxy: hash-consed nodes are === in JS too

  // Awaited regions are a plain array of flat items ({ id, x, y, w, h, cx,
  // cy, area, … }). Each item carries its link [node, index] (the node it
  // came from and its position there) under an enumerable symbol key: object
  // spread copies it, JSON, Object.keys and the YAML result ignore it, and
  // filter/sort/slice keep the same objects. Passed back to an op, marshal
  // sends it as __cvref, so the item stands for its region's exact pixels.
  const ITEM = Symbol("cv.item");

  function linkItems(arr, id) {
    if (Array.isArray(arr)) arr.forEach((it, i) => { if (it !== null && typeof it === "object") it[ITEM] = [id, i]; });
    return arr;
  }

  function isNode(x) {
    return typeof x === "object" && x !== null && nodeSet.has(x);
  }

  // marshal converts call arguments for the native side: nodes become
  // { __cvnode: id }, linked items { __cvref: [node, index], …fields };
  // arrays and plain objects are walked (a node is accepted only where an op
  // takes an image, a mask or regions; elsewhere Go refuses it: options are
  // literal values).
  function marshal(x) {
    if (x === null || x === undefined) return x;
    if (isNode(x)) return { __cvnode: x.__id };
    if (Array.isArray(x)) return x.map(marshal);
    if (typeof x === "object") {
      const o = {};
      if (x[ITEM] !== undefined) o.__cvref = x[ITEM];
      for (const k of Object.keys(x)) o[k] = marshal(x[k]);
      return o;
    }
    return x;
  }

  function label(t) {
    const d = native.__explain(t.__id);
    return "[" + d.kind + " #" + d.id + " " + d.call + "]";
  }

  function callOp(t, op, args) {
    const h = native.__call(op, t ? t.__id : 0, args.map(marshal));
    return Array.isArray(h) ? h.map(nodeFor) : nodeFor(h); // a macro may build several nodes (split)
  }

  // Builder lambdas (within(mask, i => …)) run here, once, while the graph
  // is built: the function receives a placeholder node for the receiver and
  // its result travels to Go as { __cvbuilt: [result, placeholder] }; Go
  // substitutes the receiver for the placeholder (graph.Builder.Substitute).
  const BUILDERS = native.__builders(); // { "Image:within": [1] }

  function cvError(code, message, fix) {
    const e = new Error(message);
    e.name = e.code = code;
    e.cv = true;
    if (fix) e.fix = fix;
    return e;
  }

  function build(t, op, fn) {
    const hole = nodeFor(native.__placeholder(t.__id));
    const res = fn(hole);
    if (!isNode(res)) {
      const what = res !== null && typeof res === "object" && typeof res.then === "function" ? "a promise (the function must not be async)" : typeof res;
      throw cvError("E_ARG_TYPE", op + "'s function must return a node built from its argument; it returned " + what + ".", "img." + op + "(mask, i => i.exposure(-0.4))");
    }
    return { __cvbuilt: [res.__id, hole.__id] };
  }

  function method(t, op) {
    const builders = BUILDERS[t.__kind + ":" + op];
    return function () {
      const args = Array.prototype.slice.call(arguments);
      if (builders) {
        for (const i of builders) {
          if (typeof args[i] === "function") args[i] = build(t, op, args[i]);
        }
      }
      return callOp(t, op, args);
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

  function lookup(t, prop) {
    if (typeof prop === "symbol") {
      if (prop === Symbol.toPrimitive) {
        return function (hint) {
          return hint === "string" ? label(t) : native.__notAwaited(t.__id, null, hint === "default" ? "primitive" : hint);
        };
      }
      if (prop === Symbol.toStringTag) return "cv." + t.__kind;
      return undefined;
    }
    switch (prop) {
      case "then":
        // await: a Value or Regions node evaluates at scale 1 (Regions: an
        // array of linked items); an Image resolves to its static summary
        // (nothing is computed)
        return function (onFulfilled, onRejected) {
          const p = native.__evaluate(t.__id);
          return (t.__kind === "Regions" ? p.then((v) => linkItems(v, t.__id)) : p).then(onFulfilled, onRejected);
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
          return native.__notAwaited(t.__id, null, "number");
        };
      case "toJSON":
        return function () {
          return { node: label(t) };
        };
      case "look":
        // look(label | { label, zoom, … }) → a Go-backed look, queued: every
        // look created in an eval is delivered (up to four)
        return function (opts) {
          return native.__look(t.__id, opts === undefined ? null : marshal(opts));
        };
      case "save":
        // save(path, { depth }) → a promise of the saved file (under --out-root)
        return function (path, opts) {
          return native.__save(t.__id, path === undefined ? null : path, opts === undefined ? null : opts);
        };
    }
    const tab = tables[t.__kind];
    const entry = tab && tab[prop];
    if (entry) return entry.ns ? namespaceFor(t, prop, entry.ns) : method(t, entry.op);
    if (PROBES.has(prop)) return undefined;
    // lazy data: reading .p99 of stats() means "after awaiting it"
    if (t.__kind === "Value" || t.__kind === "Regions") return native.__notAwaited(t.__id, prop);
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
      return prop === "then" || prop === "__id" || prop === "__kind" || prop === "look" || prop === "save" || !!(tab && tab[prop]);
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
      e.cv = true;
      e.fix = where + "(img.gaussian(2))";
      throw e;
    }
    return x;
  }

  const cv = {
    version: native.__version,
    // open(path, { encoding, box, origin }) → Image
    open(path, opts) {
      return nodeFor(native.__open(path, opts === undefined ? null : marshal(opts)));
    },
    help(name) {
      return native.__help(name === undefined ? null : name);
    },
    // docs: the guides (pkg/cv/docs): list(), search(text, { limit }), read(id, { section })
    docs: Object.freeze({
      list() {
        return native.__docsList();
      },
      search(text, opts) {
        return native.__docsSearch(text, opts === undefined ? null : opts);
      },
      read(id, opts) {
        return native.__docsRead(id, opts === undefined ? null : opts);
      },
    }),
    // explain(node) → { id, kind, op, call, frame, encoding, space, path,
    // graph }: synchronous, evaluates nothing
    explain(node) {
      return native.__explain(requireNode(node, "cv.explain").__id);
    },
    // json(x, { name }) / csv(x, { name, columns }) → a Data handle: content
    // for programs (CLI: a file), or .save(path) under --out-root. Plain
    // data is passed as it is (not lifted back to its node).
    json(x, opts) {
      return native.__data("json", x, opts === undefined ? null : opts);
    },
    csv(x, opts) {
      return native.__data("csv", x, opts === undefined ? null : opts);
    },
  };
  for (const name of MODULE_OPS) {
    cv[name] = function () {
      return callOp(null, name, Array.prototype.slice.call(arguments));
    };
  }
  return cv;
})
