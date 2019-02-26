// https://github.com/jsverify/jsverify/blob/master/helpers/jasmineHelpers2.js
import jsc from 'jsverify';

beforeEach(function () {
  "use strict";

  function message(r) {
    return "Expected property to hold. Counterexample found: " + r.counterexamplestr;
  }

  jasmine.addMatchers({
    toHold: function () {
      return {
        compare: function (actual, done) {
          var r = jsc.check(actual);
          if (done) {
            Promise.resolve().then(function () { return r; }).then(function (v) {
              // TODO: update jsverify after the fix is merged: https://github.com/jsverify/jsverify/pull/283
              // @ts-ignore
              if (v === true) {
                done();
              } else {
                done.fail(message(v));
              }
            });
            return {
              pass: true,
            };
          }
          return {
            // TODO: update jsverify after the fix is merged
            // @ts-ignore
            pass: r === true,
            message: message(r),
          };
        },
      };
    },
  });
});