const protobuf = require('protobufjs');
const fs = require('fs');

const root = new protobuf.Root();
root.define('cothority');

const regex = /^.*\.proto$/;

fs.readdir('src/models', (err, items) => {
  items.forEach(file => {
    if (regex.test(file)) {
      root.loadSync('src/models/' + file);
    }
  });

  fs.writeFileSync('src/models/skeleton.js', `export default '${JSON.stringify(root.toJSON())}';`)
});

