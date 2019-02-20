KyberJS
=======

Javascript implementation of [Kyber interfaces](https://github.com/dedis/kyber/blob/master/group.go)

1. **This is developmental, and not ready for protecting production data.**
2. **This is not a constant time implementation, and likely has timing side channels that can be attacked.**

Usage
-----

In the browser:

The bundle is compiled using the command:

```
npm run bundle
```

Check index.html for a browser-based usage

In NodeJS:

```js
import kyber from "@dedis/kyber";
import { newCurve } from "@dedis/kyber/curve";
...
```

Dev Setup
---------

```
git clone https://github.com/dedis/cothority
cd cothority/external/js/kyber
npm run link

cd $WORK_DIR
npm link @dedis/kyber
```

Browser Build
-------------

`npm run build` will transpile the typescript files of the _src_ folder into _dist_ and
`npm run bundle` will pack everything inside a minimalistic bundle again in _dist_

Running Tests
-------------

Execute `npm test` to run the unit tests and get the coverage

Generate Documentation
----------------------

Execute `npm run doc` to generate the documentation and browse doc/index.html

Publishing
----------

You must use the given script instead of `npm publish` because we need to publish the _dist_ folder instead. If you try to use the official command, you will get an error on purpose.
