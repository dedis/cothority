const misc = require("../misc");

/**
 * Proof represents a key/value entry in the collection and the path to the
 * root node.
 */
class Proof {
  /**
   * Creates a new proof from the protobuf representation
   * @param proof
   */
  constructor(proof) {
    this._proof = proof;
    let steps = proof.inclusionproof.steps;
    let left = steps[steps.length - 1].left;
    let right = steps[steps.length - 1].right;
    if (misc.uint8ArrayCompare(left.key, this.key, false)) {
      this._leaf = left;
    } else if (misc.uint8ArrayCompare(right.key, this.key, false)) {
      this._leaf = right;
    }
  }

  /**
   * @return {boolean} matches - true if the proof has the key/value pair
   * stored, false if it is a proof of absence.
   */
  matches() {
    return this._leaf !== undefined;
  }

  /**
   * @return {Uint8Array} key - the key of the leaf node
   */
  get key() {
    return this._proof.inclusionproof.key.slice(0);
  }

  /**
   * @return {Uint8Array[]} values - the list of values in the leaf node
   */
  get values() {
    let ret = [];
    this._leaf.values.forEach(v => {
      ret.push(v.slice(0));
    });

    return ret;
  }
}

module.exports = Proof;
