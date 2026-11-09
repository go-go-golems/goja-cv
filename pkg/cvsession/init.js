// cv session globals. Evaluates to function (native, cv) that defines the
// pre-bound globals of an eval session:
//   cv        the module
//   mem       a plain object that persists between evals
//   img       the first named image (a getter/setter; throws E_NO_IMAGE when unset);
//             cv.open(path, { name: "ref" }) binds more names the same way
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

  // cv.open reads in the session's workspace and read directories, and
  // takes { name } to bind the image to a global; the module's open does not
  // know name.
  const open = cv.open;
  cv.open = function (path, opts) {
    let name;
    let rest = opts;
    if (opts !== null && typeof opts === "object" && !Array.isArray(opts) && "name" in opts) {
      name = opts.name;
      rest = Object.assign({}, opts);
      delete rest.name;
      native.checkName(name);
    }
    const node = open(path, rest);
    if (name !== undefined && native.bindName(name, node)) defineName(name);
    return node;
  };

  // cv gives a structured unknown-name error (with did-you-mean) instead of a
  // raw TypeError when a script calls a member it does not have.
  const PROBES = new Set(["then", "toJSON", "constructor", "inspect", "__proto__", "prototype", "nodeType", "$$typeof", "asymmetricMatch"]);
  function guard(obj, name) {
    return new Proxy(obj, {
      get(t, p, r) {
        if (typeof p === "symbol" || p in t || PROBES.has(p)) return Reflect.get(t, p, r);
        return native.unknownMember(name, String(p), Object.keys(t));
      },
    });
  }

  define("cv", guard(cv, "cv"));
  define("mem", {});

  defineName("img");

  Object.defineProperty(g, "__cv_run", {
    configurable: false,
    enumerable: false,
    writable: false,
    // The body starts from a promise job: goja survives an interrupt
    // (timeout) in the synchronous part of an async function only when it was
    // started from a job. Every cv.load and cv.save the eval started settles
    // before it returns, awaited or not (also those a pending one started);
    // an unawaited save that failed fails the eval, any other rejection
    // nobody handled is an UNHANDLED warning.
    value: function (body) {
      function saves(seen) {
        const ps = native.pending();
        if (ps.length > seen) return Promise.allSettled(ps).then(function () { return saves(ps.length); });
        const e = native.settled();
        if (e !== null) throw e;
        return Promise.resolve();
      }
      return Promise.resolve()
        .then(function () {
          return body();
        })
        .then(
          function (v) {
            return saves(0).then(function () {
              return v;
            });
          },
          function (e) {
            return saves(0).then(
              function () {
                throw e;
              },
              function () {
                throw e;
              },
            );
          },
        )
        .then(undefined, function (e) {
          native.fail(e);
          throw e;
        });
    },
  });
})
