## Initial state
![collection](assets/images/example/0.gif)

States of admin, read/writers, and readers are A0, W0, R0.

## Charlie emits a request adding Erin to admin
![collection](assets/images/example/1.gif)

## State of "admin" is incremented to A1
![collection](assets/images/example/2.gif)

Because Charlie is in the server's admin collection at state A0, the request is valid. State A0 is mutated to A1.

## Server emits proof + state change, floods to all
![collection](assets/images/example/3.gif)

Proof: Charlie is in A0, thus you are requested to: mutate A0, adding Erin.

## Verifiers accept the proof and mutation
![collection](assets/images/example/4.gif)

QUESTION: What are they checking?

## New consistent view of state A1
![collection](assets/images/example/5.gif)

## Erin emits a request adding Dan to RW
![collection](assets/images/example/6.gif)

## State of "read/writers" is incremented to W1
![collection](assets/images/example/7.gif)

Because Erin is in the admin collection at state A1, the request is valid. State W0 is mutated to W1.

## Server emits proof + state change, floods to all
![collection](assets/images/example/8.gif)

Proof: Erin is in A1, thus you are requested to: mutate W0, adding Dan.

## Verifiers accept the proof and mutation
![collection](assets/images/example/9.gif)

## Consistent view of read/writers state == W1
![collection](assets/images/example/10.gif)

## Erin emits request to add Alice to RW
![collection](assets/images/example/11.gif)

## State of "Read/writers" is incremented to W2
![collection](assets/images/example/12.gif)

Because Erin is in A1, the request is valid. State W1 is mutated to state W2.

## Proof and mutation W1->W2 flooded
![collection](assets/images/example/13.gif)

Proof: Erin is in A1, thus you are requested to: mutate W1 adding Alice.

## Verifiers accept the proof and mutation
![collection](assets/images/example/14.gif)

## Consistent view of read/writers state == W2
![collection](assets/images/example/15.gif)

## Charlie emits a request "moving" Dan to read
![collection](assets/images/example/16.gif)

QUESTION: What is this new "move" verb? It seems like it is an Update which atomically combines a delete and an add?

## State of readers is incremented to R1, read/writers to W3
![collection](assets/images/example/17.gif)

Because Charlie is in A1, the request is valid. Decompose it into two proofs, W2-Dan = W3, and R0+Dan=R1.

## Server emits proof + state change, floods
![collection](assets/images/example/18.gif)

## Verifiers accept the proofs and apply the mutations
![collection](assets/images/example/19.gif)

## Consistent view
![collection](assets/images/example/20.gif)

## Server emits false proof and fraudulent mutation to all
![collection](assets/images/example/21.gif)

Proof: Mallory is in A1 (false). Mutation: add "NotAHacker" to read/writers.

## Verifiers do not find Mallory in A1
![collection](assets/images/example/22.gif)

Thus the fraudulent mutation is refused.

## State remains consistent and unmodified
![collection](assets/images/example/23.gif)

