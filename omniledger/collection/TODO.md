- make golint and go vet happy:
  - change all methods to follow golint's suggestions
  - add comments to all Public methods

- sha256.go should go away. If we ever want to implement this in another
language, then we need to have basic, step-by-step hashes, that can easily
be copied to other languages.

- remove all `csha256`, as go implementations are the basic implementations,
if we do something with the same name (sha256.go), then we need to change _our_
name, not golang's name.
