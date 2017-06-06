# CothorityProtoBuf
Implementation of messages of the Cothority for protobuf protocl

# How to build #

ES6 compilation
```
npm i
npm i -g rollup
node build_proto.js # transpile the *.proto files
rollup src/index.js --output dist/cothority-messages.js
```

IIFE compilation
````
npm i
node build_proto.js
node build.js
````


# How to use #

use the file `dist/cothority-messages.js`. It is an ES6 module so you need to use Babel or an other transpiler. Then
you can simply use
```
import CothorityMessages from './dist/cothority-messages'

CothorityMessages.createSignatureRequest(...);
```

In the case of the IIFE compilation you can import the scripts in <script></script> tag using both `bundle.js` and `protobuf.js`
