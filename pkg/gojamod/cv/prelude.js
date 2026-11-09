// cv prelude: builds the public `cv` object over the native binding.
// Evaluates to a function(native) returning the exports object.
(function (native) {
  "use strict";
  const cv = {};
  cv.version = "0.0.0-m0";
  cv.help = function (name) {
    return native.__help(name);
  };
  return cv;
})
