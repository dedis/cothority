#!/bin/bash

npm run build

cp README.md dist/.
cp package.json dist/.
cp index.html dist/.

# remove the private field of the package json
sed -i '/"private": true,/d' dist/package.json
# fix the bundle path
sed -i 's/src="dist\//src="/' dist/index.html

if [ "$1" = "--link" ] || [ "$1" = "-l" ]; then
    # linking allow to use the package locally
    cd dist/
    npm link
else
    # don't need the bundle when linking the package, neither the doc
    npm run bundle
    npm run doc
    cp -r doc dist/doc

    npm publish dist/ --dry-run
fi