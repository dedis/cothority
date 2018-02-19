KyberJS
=======

Javascript implementation of [Kyber interfaces](https://github.com/dedis/kyber/blob/master/group.go)

1. **This is developmental, and not ready for protecting production data.**
2. **This is not a constant time implementation, and likely has timing side channels that can be attacked.**

Usage
-----

In the browser:

```html
<html>
  <head>
    <meta charset="UTF-8">
    <script src="dist/bundle.min.js" type="text/javascript"></script>
    <script type="text/javascript">
      var nist = kyber.curve.nist;
      var p256 = new nist.Curve(nist.Params.p256);
      var randomPoint = p256.point().pick();
      var randomScalar = p256.scalar().pick();
      var product = p256.point().mul(randomScalar, randomPoint);
      console.log(product.string(), randomPoint.string(), randomScalar.string());
    </script>
  </head>
  <body>
  </body>
</html>
``` 

In node_js:
```js
const kyber = require("@dedis/kyber-js");
```

Dev Setup
---------

```
git clone https://github.com/dedis/cothority
cd cothority/external/js/kyber
npm install
```

Browser Build
-------------

`npm run build` will output `dist/bundle.min.js` and `dist/bundle.node.min.js`

Running Tests
-------------

Execute `npm test` to run the unit tests.

Generate Documentation
----------------------

Execute `npm run doc` to generate JSDoc output in markdown format in
`doc/doc.md`
