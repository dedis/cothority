#!/bin/bash

# Stop on error
set -e

npm run build

cp README.md dist/.
cp package.json dist/.
cp package-lock.json dist/.
cp index.html dist/.

# remove the private field of the package json
sed -i -e '/"private": true,/d' dist/package.json
# fix the bundle path
sed -i -e 's/src="dist\//src="/' dist/index.html

if [ "$1" = "--link" ] || [ "$1" = "-l" ]; then
    # linking allow to use the package locally
    cd dist/
    npm link
else
    # don't need the bundle when linking the package, neither the doc
    npm run bundle
    rm -rf doc dist/doc
    npm run doc
    cp -r doc dist/doc

    npm publish dist --access public $*
fi
