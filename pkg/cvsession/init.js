// cv session globals. Evaluates to function (native, cv) that defines the
// pre-bound globals of an eval session:
//   cv, P     the module and its params
//   mem       a plain object that persists between evals
//   show      show(node, opts) / show(look): queue a look and return it
//   img       the first named image (a getter/setter; throws E_NO_IMAGE when unset);
//             cv.open(path, { as: "ref" }) binds more names the same way
//   doc       the document layer (not available before M4: E_NO_DOCUMENT)
//   __cv_run  the eval wrapper used by cvsession.App.Evaluate
(function (native, cv) {
  "use strict";
  const g = globalThis;
  function define(name, value) {
    Object.defineProperty(g, name, { value: value, writable: true, configurable: true, enumerable: false });
  }

  // a name is a global backed by the session's name table
  function defineName(name) {
    Object.defineProperty(g, name, {
      configurable: true,
      enumerable: false,
      get() {
        return native.getName(name);
      },
      set(v) {
        native.setName(name, v);
      },
    });
  }

  // cv.open is restricted to the session's roots (when configured), and
  // takes { as } (or name, bind, alias) to bind the image to a global; the
  // module's open does not know as.
  const open = cv.open;
  cv.open = function (path, opts) {
    native.checkPath(path);
    let as;
    let rest = opts;
    if (opts !== null && typeof opts === "object" && !Array.isArray(opts) && !cv.isNode(opts)) {
      const key = native.asKey(Object.keys(opts));
      if (key !== null) {
        as = opts[key];
        rest = Object.assign({}, opts);
        delete rest[key];
        native.checkName(as);
      }
    }
    const node = open(path, rest);
    if (as !== undefined && native.bindName(as, node)) defineName(as);
    return node;
  };

  // cv and P give a structured unknown-name error (with did-you-mean) instead
  // of a raw TypeError when a script calls a member they do not have.
  const PROBES = new Set(["then", "toJSON", "constructor", "inspect", "__proto__", "prototype", "nodeType", "$$typeof", "asymmetricMatch"]);
  let guardedP;
  function guard(obj, name) {
    return new Proxy(obj, {
      get(t, p, r) {
        if (name === "cv" && p === "P") return guardedP;
        if (typeof p === "symbol" || p in t || PROBES.has(p)) return Reflect.get(t, p, r);
        return native.unknownMember(name, String(p), Object.keys(t));
      },
    });
  }
  guardedP = guard(cv.P, "P");

  define("cv", guard(cv, "cv"));
  define("P", guardedP);
  define("mem", {});
  define("show", function show(x, opts) {
    const l = cv.isNode(x) ? x.look(opts) : x;
    native.queue(l);
    return l;
  });

  defineName("img");

  define(
    "doc",
    new Proxy(Object.create(null), {
      get(_, prop) {
        if (typeof prop === "symbol" || prop === "then" || prop === "toJSON") return undefined;
        return native.noDoc(String(prop));
      },
      set(_, prop) {
        return native.noDoc(String(prop));
      },
    })
  );

  Object.defineProperty(g, "__cv_run", {
    configurable: false,
    enumerable: false,
    writable: false,
    // The body starts from a promise job: goja survives an interrupt
    // (timeout) in the synchronous part of an async function only when it was
    // started from a job.
    value: function (body) {
      return Promise.resolve()
        .then(function () {
          return body();
        })
        .then(undefined, function (e) {
          native.fail(e);
          throw e;
        });
    },
  });
})
