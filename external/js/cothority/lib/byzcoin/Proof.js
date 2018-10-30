const StateChangeBody = require("./StateChangeBody");

/**
 * Proof represents a key/value entry in the collection and the path to the
 * root node.
 */
class Proof {
  /**
   * Creates a new proof from the protobuf representation
   * @param proto
   */
  constructor(proto) {
    this._proof = proto.inclusionproof;
    this._latest = proto.latest;
    this._links = proto.links;

    if (this._proof.leaf.key.length !== 0) {
      this._stateChangeBody = StateChangeBody.fromByteBuffer(this._proof.leaf.value)
    }
  }

  /**
   * @return {boolean} matches - true if the proof has the key/value pair
   * stored, false if it is a proof of absence.
   */
  matches() {
    // TODO write a proper way to do match
    return this._stateChangeBody !== undefined;
  }

  /**
   * @return {Uint8Array} key - the key of the leaf node
   */
  get key() {
    return this._proof.leaf.key
  }

  /**
   * @return {StateChangeBody} stateChangeBody - the content in the leaf node
   */
  get stateChangeBody() {
    return this._stateChangeBody
  }
}

module.exports = Proof;
