KyberJS
=======

Javascript implementation of [Kyber interfaces](https://github.com/dedis/kyber/blob/master/group.go)

Usage
-----

```html
<html>
  <head>
    <script src="dist/bundle.min.js" type="text/javascript"></script>
    <script type="text/javascript">
	  var nist = kyber.group.nist;
      var p256 = new nist.Curve(nist.Params.p256);
      console.log(p256.string()); // P256
    </script>
  </head>
  <body>
  </body>
</html>
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

`npm run-script build` will output a browserified `bundle.min.js` in the project
root

Running Tests
-------------

Execute `npm test` to run the unit tests.

Generate Documentation
----------------------

Execute `npm run-script doc` to generate JSDoc output in markdown format in
`doc/doc.md`
